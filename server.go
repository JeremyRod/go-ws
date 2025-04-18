package main

import (
	"log"
	"net/http"
	"os"
	"strings"
)

var activeConnections = make(map[string]*WsInstance)

// This is the handler for the route to get for upgrades
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	// The request should contain the handshake for upgrading to websocket.
	if r.Method != http.MethodGet {
		http.Error(w, "405 Method Not Allowed (wrong method)", http.StatusMethodNotAllowed)
		return //&handshakeError{err: "Not Get method"}
	}
	if r.Header.Get("Upgrade") != "websocket" || r.Header.Get("Connection") != "Upgrade" ||
		r.Header.Get("Sec-WebSocket-Version") != "13" {
		http.Error(w, "400 Bad Request (invalid params)", http.StatusBadRequest)
		return //&handshakeError{err: "Invalid params", val: "-1"}
	}

	wsKey := r.Header.Get("Sec-WebSocket-Key")
	if wsKey == "" {
		http.Error(w, "400 Bad Request (invalid key)", http.StatusBadRequest)
		return //&handshakeError{err: "WebSocket-Sec-Key header missing", val: wsKey}
	}
	genKey := genHandshakeResp(wsKey)
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Sec-WebSocket-Accept", genKey)
	w.WriteHeader(http.StatusSwitchingProtocols)
	//fmt.Println("responding")

	// Hijack the connection to gain access to the raw TCP connection
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("Failed to hijack connection: %v", err)
		return
	}

	// Create done channel for cleanup
	done := make(chan bool)

	inst := WsInstance{
		conn:   conn,
		server: true,
		sendCh: make(chan []byte, 5),
	}

	// Start reading messages in a goroutine
	go func() {
		for {
			err := inst.ReadMessage()
			if err != nil {
				logger.Println("Error reading message:", err)
				close(done)
				return
			}
		}
	}()

	// Start writing messages in a goroutine
	go func() {
		for {
			select {
			case msg, ok := <-inst.sendCh:
				if !ok {
					return
				}
				err := inst.WriteMessage(msg, Text)
				if err != nil {
					logger.Println("Error writing message:", err)
					close(done)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Wait for connection to close
	<-done
}

func main() {
	// Should the demo run in client or server mode.
	// Use cmd line args

	// 1st arg either client or server
	// 2nd arg only checked for client, data to send
	SetLogger()

	cmdArgs := os.Args[1:]

	if cmdArgs[0] == "server" {

		// Register an HTTP handler for WebSocket requests
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
				handleWebsocket(w, r)
			} else {
				http.Error(w, "Not a WebSocket request", http.StatusBadRequest)
			}
		})
		// Register a regular HTTP handler for other requests
		// http.HandleFunc("/", handleHome)

		logger.Println("Starting server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Fatalf("Server failed: %v", err)
		}
	} else {
		startClient(cmdArgs[1])
	}
	CloseLogger()
}
