package hub

import (
	"testing"
	"time"
)

func TestHubRegisterAndGetClient(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := &Client{
		Hub:      h,
		DeviceID: "dev-1",
		Send:     make(chan []byte, 1),
	}
	h.Register(c)

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		got, ok := h.GetClient("dev-1")
		if ok {
			if got != c {
				t.Fatalf("expected same client pointer")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("client was not registered in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHubUnregisterRemovesClientAndClosesChannel(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := &Client{
		Hub:      h,
		DeviceID: "dev-2",
		Send:     make(chan []byte, 1),
	}
	h.Register(c)

	// Wait until client is visible.
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if _, ok := h.GetClient("dev-2"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("client was not registered in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	h.Unregister(c)

	// Wait until client is removed.
	deadline = time.Now().Add(500 * time.Millisecond)
	for {
		if _, ok := h.GetClient("dev-2"); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("client was not unregistered in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Send channel should be closed by hub.
	_, ok := <-c.Send
	if ok {
		t.Fatalf("expected send channel to be closed")
	}
}

func TestHubUnregisterDifferentPointerDoesNotDeleteCurrentClient(t *testing.T) {
	h := NewHub()
	go h.Run()

	current := &Client{
		Hub:      h,
		DeviceID: "dev-3",
		Send:     make(chan []byte, 1),
	}
	stale := &Client{
		Hub:      h,
		DeviceID: "dev-3",
		Send:     make(chan []byte, 1),
	}

	h.Register(current)
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if _, ok := h.GetClient("dev-3"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("client was not registered in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	h.Unregister(stale)
	time.Sleep(50 * time.Millisecond)

	got, ok := h.GetClient("dev-3")
	if !ok || got != current {
		t.Fatalf("expected current client to remain registered")
	}
}

