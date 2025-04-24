package main

type State int

const (
	CONNECTING State = iota
	OPEN
	CLOSING
	CLOSED
)
