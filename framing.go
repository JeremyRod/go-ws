package websocket

type WsFrame struct {
	msgType     uint8 // this includes fin, rsv1-3 and the opcodes
	payloadMask uint8 // lowest bit is mask flag, other 7 are payload len
	// extPayload  uint64 // Extended payload. if payload == 127
	// maskingKey  [4]byte
	// above commented out since they may not be there. is dynamic.
	data []byte
}

type OPCODE uint8 // its actually only 4 bits

const (
	Cont OPCODE = iota
	Text
	Bin
	ConClose = 0x8
	Ping     = 0x9
	Pong     = 0xA
)
