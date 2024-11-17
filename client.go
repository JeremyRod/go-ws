package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
		log.Fatalf("invalid Sec-WebSocket-Accept key: expected %s, got %s", expectedAccept, headers["Sec-Webocket-Accept"])
	}
	log.Println("WebSocket handshake successful! Connection upgraded.")
	inst := WsInstance{
		conn:   conn,
		server: false,
	}
	for i := 0; i < 5; i++ {
		msg += strconv.Itoa(i)
		inst.writeBuffer = []byte(msg)
		inst.WriteFrame()
		time.Sleep(time.Millisecond * 500)
		inst.ReadFrame()
	}
	time.Sleep(time.Second * 5)
	for i := 0; i < 5; i++ {
		msg += strconv.Itoa(i)
		inst.writeBuffer = []byte(msg)
		inst.WriteFrame()
		time.Sleep(time.Millisecond * 500)
		inst.ReadFrame()
	}
	for {
		inst.ReadFrame()
	}
	// from here we can handle new reads

}
