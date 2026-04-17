package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func RunTunnel(tunnelAddr string, targetHost string, targetPort int) {
	log.Printf("[Tunnel] Opening tunnel to %s -> %s:%d", tunnelAddr, targetHost, targetPort)

	// 1. Connect to the Hub's tunnel endpoint
	conn, err := net.DialTimeout("tcp", tunnelAddr, 10*time.Second)
	if err != nil {
		log.Printf("[Tunnel] Dial failed: %v", err)
		return
	}
	defer conn.Close()

	// 2. Connect to the target host (the website the buyer wants to visit)
	target, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetHost, targetPort), 10*time.Second)
	if err != nil {
		log.Printf("[Tunnel] Target dial failed (%s:%d): %v", targetHost, targetPort, err)
		return
	}
	defer target.Close()

	// 3. Bidirectional pipe
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(target, conn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(conn, target)
		done <- struct{}{}
	}()

	<-done
	log.Printf("[Tunnel] Session finished for %s:%d", targetHost, targetPort)
}
