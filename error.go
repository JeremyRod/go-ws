package main

import (
	"fmt"
	"log"
	"os"
)

type handshakeError struct {
	val string
	err string
}

var logger *log.Logger
var f *os.File

func (h *handshakeError) Error() string {
	return fmt.Sprintf("error: %v, value: %s", h.err, h.val)
}

func SetLogger() {
	f, err := os.OpenFile("wslogdmp.txt", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	logger = &log.Logger{}
	if err != nil {
		logger.Fatalf("error opening file: %v", err)
	}
	logger.SetFlags(log.LstdFlags | log.Lshortfile)
	logger.SetOutput(f)
}

func CloseLogger() {
	f.Close()
}
