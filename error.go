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

func (h *handshakeError) Error() string {
	return fmt.Sprintf("error: %v, value: %s", h.err, h.val)
}

func SetLogger() {
	f, err := os.OpenFile("wslogdmp.txt", os.O_RDWR|os.O_APPEND, 0666)
	logger = &log.Logger{}
	if err != nil {
		logger.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	logger.SetFlags(log.LstdFlags | log.Lshortfile)
	logger.SetOutput(f)
}