package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
const websocketPort = "9002"

type UpgradeConn struct {
}

func genHandshakeResp(wsKey string) string {
	hasher := sha1.New()
	wsKey = strings.Trim(wsKey, " ")
	wsKey = wsKey + websocketGUID
	hasher.Write([]byte(wsKey))
	sEnc := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	return sEnc
}

// The client init handshake is a http req to the server side.
func ClientHandshake(key string, conn net.Conn) error {
	encKey := base64.StdEncoding.EncodeToString([]byte(key))

	reqHeaders := "GET /ws HTTP/1.1\r\n" +
		"Host: localhost:" + websocketPort + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + encKey +
		"Sec-WebSocket-Version: 13\r\n\r\n"

	_, err := conn.Write([]byte(reqHeaders))
	if err != nil {
		return &handshakeError{err: err.Error(), val: "write failed"}
	}
	response, err := bufio.NewReaderSize(conn, 1024).ReadString('\n')
	if err != nil {
		return &handshakeError{err: err.Error(), val: "read failed"}
	}
	// Check for HTTP 101 Switching Protocols response
	if !strings.Contains(response, "101 Switching Protocols") {
		return &handshakeError{err: "Wrong protocol", val: response}
	}

	for {
		line, err := bufio.NewReaderSize(conn, 1024).ReadString('\n')
		if err != nil {
			return &handshakeError{err: err.Error(), val: line}
		}
		if line == "\r\n" {
			break // End of headers
		}
		if strings.HasPrefix(line, "Sec-WebSocket-Accept:") {
			expwskey := genHandshakeResp(encKey)
			recwskey := strings.TrimSpace(strings.Split(line, ":")[1])
			if expwskey != recwskey {
				return &handshakeError{err: "invalid handshake", val: fmt.Sprintf("got: %s expected: %s", recwskey, expwskey)}
			}
		}
	}

	fmt.Println("WebSocket handshake successful!")
	return nil
}
