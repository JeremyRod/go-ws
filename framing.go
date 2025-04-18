package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

type OPCODE uint8 // its actually only 4 bits

const (
	Cont OPCODE = iota
	Text
	Bin
	ConClose = 0x8
	Ping     = 0x9
	Pong     = 0xA
)

const ( //payload indexes
	Type uint8 = iota
	Mask
	ExtPayload
	NoMaskKeyNoExt = 2
	NoMaskKey16Ext = 4
	NoMaskKey64Ext = 10
	MaskKeyNoExt   = 6
	MaskKey16Ext   = 8
	MaskKey64Ext   = 14
)

type WsInstance struct {
	conn        net.Conn
	readBuffer  []byte
	writeBuffer []byte
	server      bool
	sendCh      chan []byte
}

type WsHeader struct {
	msgType     OPCODE // this includes fin, rsv1-3 and the opcodes
	payloadMask uint8  // lowest bit is mask flag, other 7 are payload len
	extPayload  uint64 // Extended payload. if payload == 127
	maskingKey  []byte
}

type WsFrame struct {
	header WsHeader
	data   []byte
}

// make WsConn be a conn interfacee
type WsConn interface {
	// Read reads data from the connection.
	// Read can be made to time out and return an error after a fixed
	// time limit; see SetDeadline and SetReadDeadline.
	Read(b []byte) (n int, err error)

	// Write writes data to the connection.
	// Write can be made to time out and return an error after a fixed
	// time limit; see SetDeadline and SetWriteDeadline.
	Write(b []byte) (n int, err error)
}

// make ws an interface that can ideally act as a conn

// read and write message will be the higher level functions
// that read frames and reconstruct messages
func (c *WsInstance) ReadMessage() error {
	frame, err := c.readFrame()
	if err != nil {
		return err
	}
	logger.Printf("Rec frame MsgType: %v\r\n", frame.header.msgType&0x7F)

	// read the data depending on what the opcode is.
	switch frame.header.msgType & 0x7F {
	case Text:

	case Bin:

	case Cont:

	case ConClose:
		// ack and send the close
		c.WriteMessage([]byte(frame.data), ConClose)
		logger.Println("closing connection")
		c.conn.Close()
		return errors.New("connection closed")

	case Ping:
		// send pong
	case Pong:
		// ?
	}
	return nil
}

func (c *WsInstance) WriteMessage(msg []byte, opcode OPCODE) error {
	// leave the rest of the frame to be written by the writeFrame function
	// TODO: if message is too large, we should split it into multiple frames.
	frame := WsFrame{
		data: msg,
		header: WsHeader{
			msgType: opcode + 128,
		},
	}
	err := c.writeFrame(frame)
	if err != nil {
		return err
	}
	return nil
}

func (c *WsInstance) SendMessage(msg []byte) error {
	c.sendCh <- msg
	return nil
}

// read frame and write frame will read each frame one by one
func (c *WsInstance) writeFrame(frame WsFrame) error {
	// write frame should probably take in a frame not just the data.
	c.conn.SetWriteDeadline(time.Now().Add(time.Second))
	// format the frame, determine if the client is writing the frame
	payLen := len(c.writeBuffer)
	extPayload := 0
	if payLen > 125 {
		extPayload = payLen
		payLen = 127
		if len(c.writeBuffer) == 126 {
			payLen = 126
		}
	}
	if !c.server {
		payLen += 128
	}
	frame.header.payloadMask = uint8(payLen) // no mask for the moment
	frame.header.extPayload = uint64(extPayload)
	frame.header.maskingKey = []byte{0, 0, 0, 0}

	// format header slice for prepending to write buffer
	writeHeader := []byte{}

	writeHeader = append(writeHeader, byte(frame.header.msgType), frame.header.payloadMask)
	if frame.header.extPayload > 0 {
		writeHeader = append(writeHeader, byte(frame.header.extPayload))
	}
	if frame.header.payloadMask>>7 == 1 {
		// masked key added
		binary.BigEndian.PutUint32(frame.header.maskingKey, rand.Uint32())
		writeHeader = append(writeHeader, frame.header.maskingKey[0:4]...)
	}

	//mask data if from the client
	if !c.server {
		ApplyMask(&frame.data, frame.header.maskingKey)
	}

	//write header to data
	frame.data = append(writeHeader, frame.data...)
	_, err := c.conn.Write(frame.data)
	if err != nil {
		// this means we cant write to the client?
		// they have aborted the con.
		//log.Println(err.Error()) // all this means is there was no data left.
		return err
	}
	return nil
}

func (c *WsInstance) readFrame() (WsFrame, error) {
	//fmt.Printf("set deadline: %v\r\n", time.Now())
	//c.conn.SetReadDeadline(time.Now().Add(time.Second))
	c.readBuffer = make([]byte, 1024)
	n, err := c.conn.Read(c.readBuffer)
	if err != nil {
		logger.Printf("Connection closed / read error: %v\r\n", err)
		return WsFrame{}, err
	}
	frame := WsFrame{
		header: WsHeader{
			msgType:     OPCODE(c.readBuffer[0]),
			payloadMask: c.readBuffer[1],
			maskingKey:  []byte{},
		},
		data: []byte{},
	}
	payloadSize := frame.header.payloadMask << 1
	if payloadSize > 125 {
		// we have an extended payload, determine which one.
		if payloadSize == 126 {
			// next 2 bytes are the extended size.
			frame.header.extPayload = uint64(binary.LittleEndian.Uint16(c.readBuffer[2:4]))
		} else {
			// next 8 bytes are extended size.
			frame.header.extPayload = uint64(binary.LittleEndian.Uint64(frame.data[2:10]))
		}
	}
	if frame.header.payloadMask>>7 == 1 {
		//frame is masked from client to server
		if payloadSize < 126 {
			frame.header.maskingKey = []byte(c.readBuffer[2:6])
		} else if payloadSize == 126 {
			frame.header.maskingKey = []byte(c.readBuffer[4:6])
		} else { // must equal 127
			frame.header.maskingKey = []byte(c.readBuffer[10:14])
		}
	}
	index := 0
	switch {
	case len(frame.header.maskingKey) == 4:
		switch {
		case payloadSize == 126:
			index = MaskKey16Ext
		case payloadSize == 127:
			index = MaskKey64Ext
		case payloadSize < 125:
			index = MaskKeyNoExt
		default:
			//len is wrong
			index = 0
		}
	case len(frame.header.maskingKey) == 0:
		switch {
		case payloadSize == 126:
			index = NoMaskKey16Ext
		case payloadSize == 127:
			index = NoMaskKey64Ext
		case payloadSize < 125:
			index = NoMaskKeyNoExt
		default:
			//len is wrong
			index = 0
		}
	default:
		//len is wrong
		index = 0
	}
	frame.data = c.readBuffer[index:n]
	if frame.header.payloadMask>>7 == 1 && c.server {
		DecodeMask(&frame.data, frame.header.maskingKey)
	}
	if frame.header.payloadMask>>7 == 1 && !c.server {
		logger.Println("err: client received masked frame.")
	}
	fmt.Printf("Rec frame: %s\r\n", string(frame.data))
	return frame, nil

}
