package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func startClient(msg string) {
	secWebSocketKey := "x3JJHMbDL1EzLkh9GBhXDw=="

	// Similar to the websocket server implementation,
	// we will hijack if the response is correct and keep it open.

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()
	url, err := url.Parse("http://localhost:8080/ws")
	if err != nil {
		log.Fatalf("url parse error: %v", err)
	}
	req := &http.Request{
		Method:     http.MethodGet,
		URL:        url,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       "localhost",
	}
	req.Header.Set("Host", "localhost:8080")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", secWebSocketKey)
	req.Header.Set("Sec-WebSocket-Version", "13")

	err = req.Write(conn)
	if err != nil {
		log.Fatalf("failed to write to server: %v", err)
	}

	respReader := bufio.NewReader(conn)
	statusLine, err := respReader.ReadString('\n')
	if err != nil {
		log.Fatalf("failed to read status line: %v", err)
	}
	if !strings.Contains(statusLine, "101 Switching Protocols") {
		log.Fatalf("expected 101 Switching Protocols, got: %s", statusLine)
	}
	headers := make(map[string]string)
	for {
		line, err := respReader.ReadString('\n')
		if err != nil {
			log.Fatalf("readline failed: %v", err)
		}
		fmt.Print(string(line))
		line = strings.TrimSpace(line)
		if line == "" {
			break // no more headers
		}
		parts := strings.SplitN(line, ": ", 2) // only look for first key.
		headers[parts[0]] = parts[1]
	}
	expectedAccept := genHandshakeResp(secWebSocketKey)
	if headers["Sec-Websocket-Accept"] != expectedAccept {
		log.Fatalf("invalid Sec-WebSocket-Accept key: expected %s, got %s", expectedAccept, headers["Sec-Websocket-Accept"])
	}
	log.Println("WebSocket handshake successful! Connection upgraded.")
	inst := WsInstance{
		conn:   conn,
		server: false,
		sendCh: make(chan []byte, 5),
	}

	// Create a done channel to signal when we should exit
	done := make(chan bool)

	go func() {
		for {
			err := inst.ReadMessage()
			if err != nil {
				logger.Println("Error reading message:", err)
				done <- true
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case msg, ok := <-inst.sendCh:
				if !ok {
					return
				}
				err := inst.WriteMessage(msg)
				if err != nil {
					logger.Println("Error writing message:", err)
					done <- true
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Start reading from stdin and sending messages
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Enter messages to send (press Ctrl+C to exit):")
		for scanner.Scan() {
			text := scanner.Text()
			if text == "exit" {
				done <- true
				return
			}
			inst.sendCh <- []byte(text)
		}
		if err := scanner.Err(); err != nil {
			logger.Println("Error reading from stdin:", err)
			done <- true
		}
	}()

	// Send the message
	inst.SendMessage([]byte(msg))

	// Wait for connection to close
	<-done
	conn.Close()
}
