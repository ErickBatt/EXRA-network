package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
)

var (
	// bufferPool reduces GC pressure by reusing byte slices for IO.
	bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 32768) // 32KB buffers
		},
	}

	// readTimeout bounds how long a single relay iteration will wait for the
	// next WebSocket frame before tearing the session down. Without this a
	// silent peer holds two goroutines and two pooled buffers indefinitely —
	// AUDIT_MARKETPLACE_v2.4.1 §1 A3 (slowloris / half-open-TCP leak).
	//
	// 90 s chosen to comfortably outlast typical NAT/keepalive intervals
	// while still bounding leak in the event of a silent peer.
	readTimeout = 90 * time.Second
)

// Bridge connects two network connections and relays WebSocket frames between
// them. It enforces per-read deadlines and guarantees that when either side
// goes down BOTH underlying connections are closed, so the partner relay
// goroutine is unblocked and released (AUDIT §1 A3).
//
// The returned byte counts are the payload bytes copied in each direction
// (1→2 and 2→1). Callers that hold billing context (sessionID, price) use
// these totals to settle Redis credits on close (AUDIT §1 G3).
func Bridge(conn1, conn2 net.Conn) (int64, int64) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			cancel()
			_ = conn1.Close()
			_ = conn2.Close()
		})
	}

	var bytes1to2, bytes2to1 atomic.Int64

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer closeBoth()
		relay(ctx, conn1, conn2, &bytes2to1)
	}()
	go func() {
		defer wg.Done()
		defer closeBoth()
		relay(ctx, conn2, conn1, &bytes1to2)
	}()

	wg.Wait()

	b1, b2 := bytes1to2.Load(), bytes2to1.Load()
	if b1+b2 > 0 {
		log.Printf("[Gateway] Bridge closed: bytes_total=%d (%d/%d)", b1+b2, b1, b2)
	}
	return b1, b2
}

// relay shovels WebSocket frames from src to dst. It sets a read deadline on
// every iteration; on deadline expiry or any unexpected error it tears down
// the connection pair via the caller's closeBoth.
//
// dst is the destination; src is where we read the next ws frame from. The
// counter param accumulates bytes read from src (== bytes written to dst),
// which the caller uses to drive billing on settlement.
func relay(ctx context.Context, dst, src net.Conn, counter *atomic.Int64) {
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)

	for {
		if ctx.Err() != nil {
			return
		}
		// Bound the wait for the next WebSocket header. Some net.Conn
		// implementations (e.g. net.Pipe in tests) don't honour deadlines —
		// in that case this is a no-op, but real *net.TCPConn and
		// gobwas/ws-wrapped sockets do.
		_ = src.SetReadDeadline(time.Now().Add(readTimeout))

		header, err := ws.ReadHeader(src)
		if err != nil {
			if isNormalClose(err) {
				return
			}
			// Deadline exceeded is the slowloris / half-open signal. Just
			// bail — the deferred closeBoth will tear down both sides and
			// unblock our partner relay. Writing a Close frame here is
			// tempting but on a silent peer the Write would block too.
			if errors.Is(err, os.ErrDeadlineExceeded) || isTimeout(err) {
				return
			}
			log.Printf("[Gateway] Read error: %v", err)
			return
		}

		_ = dst.SetWriteDeadline(time.Now().Add(readTimeout))
		if err := ws.WriteHeader(dst, header); err != nil {
			log.Printf("[Gateway] Write header error: %v", err)
			return
		}

		if header.Length > 0 {
			n, err := io.CopyBuffer(dst, io.LimitReader(src, header.Length), buf)
			if n > 0 {
				counter.Add(n)
			}
			if err != nil {
				log.Printf("[Gateway] Copy payload error: %v", err)
				return
			}
		}

		if header.OpCode == ws.OpClose {
			return
		}
	}
}

func isNormalClose(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return false
}

func isTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}

