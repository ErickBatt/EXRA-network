package main

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/gobwas/ws"
)

var (
	// bufferPool reduces GC pressure by reusing byte slices for IO.
	bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 32768) // 32KB buffers
		},
	}
)

// Bridge connects two network connections and relays WebSocket frames between them.
// It uses a zero-allocation approach where possible.
func Bridge(conn1, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		relay(conn1, conn2)
		conn2.Close() // Ensure counterpart closes when one side fails
	}()

	go func() {
		defer wg.Done()
		relay(conn2, conn1)
		conn1.Close()
	}()

	wg.Wait()
}

func relay(dst, src net.Conn) {
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)

	for {
		// Read next frame from source
		header, err := ws.ReadHeader(src)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Gateway] Read error: %v", err)
			}
			return
		}

		// Prepare to write same header to destination
		if err := ws.WriteHeader(dst, header); err != nil {
			log.Printf("[Gateway] Write header error: %v", err)
			return
		}

		// Copy payload using our pooled buffer
		if _, err := io.CopyBuffer(dst, io.LimitReader(src, header.Length), buf); err != nil {
			log.Printf("[Gateway] Copy payload error: %v", err)
			return
		}

		// Handle control frames (Ping/Pong/Close)
		if header.OpCode == ws.OpClose {
			return
		}
	}
}

