package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func startClient() {
	secWebSocketKey := "x3JJHMbDL1EzLkh9GBhXDw=="

	// Similar to the websocket server implementation,
	// we will hijack if the response is correct and keep it open.

	inst := WsInstance{
		conn:   nil,
		server: false,
		sendCh: make(chan []byte, 5),
		state:  CONNECTING,
	}

	conn, err := net.Dial("tcp", "localhost:9001")
	inst.conn = conn
	if err != nil {
		log.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()
	url, err := url.Parse("http://localhost:9001/getCaseCount")
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
		Host:       "localhost:9001",
	}
	req.Header.Set("Host", "localhost:9001")
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
	if headers["Sec-WebSocket-Accept"] != expectedAccept {
		log.Fatalf("invalid Sec-WebSocket-Accept key: expected %s, got %s", expectedAccept, headers["Sec-WebSocket-Accept"])
	}
	log.Println("WebSocket handshake successful! Connection upgraded.")
	inst.readBuffer = make([]byte, 1024)
	n, err := respReader.Read(inst.readBuffer)
	if err != nil {
		log.Fatalf("failed to read from server: %v", err)
	}
	inst.readBuffer = inst.readBuffer[:n]
	inst.MessageFromStart() // only here to read the payload in the connection frame.
	inst.state = OPEN

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
				err := inst.SendFrame(msg)
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
	// go func() {
	// 	scanner := bufio.NewScanner(os.Stdin)
	// 	fmt.Println("Enter messages to send (press Ctrl+C to exit):")
	// 	for scanner.Scan() {
	// 		text := scanner.Text()
	// 		if text == "exit" {
	// 			inst.WriteMessage([]byte("close reason, bye"), ConClose)
	// 			// done <- true
	// 			// return
	// 		}
	// 		if text == "ping" {
	// 			inst.WriteMessage([]byte("ping"), Ping)
	// 			// done <- true
	// 			// return
	// 		}
	// 		inst.WriteMessage([]byte(text), Text)
	// 	}
	// 	if err := scanner.Err(); err != nil {
	// 		logger.Println("Error reading from stdin:", err)
	// 		done <- true
	// 	}
	// }()

	// Wait for connection to close
	<-done
	logger.Println("Connection closed")
}
