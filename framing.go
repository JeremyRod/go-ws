package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

type OPCODE uint8 // its actually only 4 bits
type CloseCode uint16

const (
	Cont OPCODE = iota
	Text
	Bin
	ConClose OPCODE = 0x8
	Ping     OPCODE = 0x9
	Pong     OPCODE = 0xA
)

const (
	CloseNormalClosure           CloseCode = 1000
	CloseGoingAway               CloseCode = 1001
	CloseProtocolError           CloseCode = 1002
	CloseUnsupportedData         CloseCode = 1003
	CloseNoStatusReceived        CloseCode = 1005
	CloseInvalidFramePayloadData CloseCode = 1007
	ClosePolicyViolation         CloseCode = 1008
	CloseMessageTooBig           CloseCode = 1009
	CloseMandatoryExtension      CloseCode = 1010
	CloseInternalServerError     CloseCode = 1011
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
	conn       net.Conn
	readBuffer []byte
	//writeBuffer []byte
	server bool
	sendCh chan []byte
	state  State
}

type WsHeader struct {
	msgType     OPCODE // this includes fin, rsv1-3 and the opcodes
	payloadMask uint8  // lowest bit is mask flag, other 7 are payload len
	extPayload  uint64 // Extended payload. if payload == 127 || 126
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

func (c *WsFrame) toBytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	// Write fixed-size fields
	if err := binary.Write(buf, binary.BigEndian, c.header.msgType); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, c.header.payloadMask); err != nil {
		return nil, err
	}
	if c.header.payloadMask == 127 {
		if err := binary.Write(buf, binary.BigEndian, c.header.extPayload); err != nil {
			return nil, err
		}
	} else if c.header.payloadMask == 126 {
		if err := binary.Write(buf, binary.BigEndian, uint16(c.header.extPayload)); err != nil { // 16 bit
			return nil, err
		}
	}

	// Write variable-size fields
	if _, err := buf.Write(c.header.maskingKey); err != nil {
		return nil, err
	}
	if _, err := buf.Write(c.data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// make ws an interface that can ideally act as a conn

// read and write message will be the higher level functions
// that read frames and reconstruct messages
func (c *WsInstance) ReadMessage() error {
	_, err := c.readFrame()
	if err != nil {
		return err
	}
	// logger.Printf("Rec frame MsgType: %v\r\n", frame.header.msgType&0x7F)

	// // read the data depending on what the opcode is.
	// switch frame.header.msgType & 0x7F {
	// case Text:
	// 	c.WriteMessage([]byte(frame.data), Text)
	// case Bin:
	// 	c.WriteMessage([]byte(frame.data), Bin)
	// case Cont:

	// case ConClose:
	// 	// ack and send the close
	// 	if c.state == CLOSING {
	// 		c.conn.Close()
	// 		c.state = CLOSED
	// 		return errors.New("connection closed")
	// 	}
	// 	c.WriteMessage([]byte(frame.data), ConClose)
	// 	return errors.New("connection closing")

	// case Ping:
	// 	c.WriteMessage([]byte(frame.data), Ping)
	// case Pong:
	// 	c.WriteMessage([]byte(frame.data), Pong)
	// }
	return nil
}

func (c *WsInstance) WriteMessage(msg []byte, opcode OPCODE) error {
	// leave the rest of the frame to be written by the writeFrame function
	// TODO: if message is too large, we should split it into multiple frames.
	frame := WsFrame{
		header: WsHeader{
			msgType: opcode + 128,
		},
		data: msg,
	}
	if c.state == CLOSED || c.state == CLOSING {
		return errors.New("connection closing")
	}
	frame, err := c.getFrame(frame)
	if err != nil {
		return err
	}
	frameBytes, err := frame.toBytes()
	if err != nil {
		return err
	}
	//fmt.Printf("Sending frame: %v\r\n", frameBytes)
	c.sendCh <- frameBytes
	return nil
}

func (c *WsInstance) SendMessage(msg []byte) error {
	c.sendCh <- msg
	return nil
}

// read frame and write frame will read each frame one by one
func (c *WsInstance) SendFrame(msg []byte) error {
	_, err := c.conn.Write(msg)
	if err != nil {
		return err
	}
	//logger.Printf("Sent frame: %v\r\n", msg)
	return nil
}

func (c *WsInstance) getFrame(frame WsFrame) (WsFrame, error) {
	// Calculate payload length and determine if we need extended payload
	payLen := len(frame.data)
	if payLen > 125 {
		if payLen <= 65535 {
			frame.header.payloadMask = 126
			frame.header.extPayload = uint64(payLen)
		} else {
			frame.header.payloadMask = 127
			frame.header.extPayload = uint64(payLen)
		}
	} else {
		frame.header.payloadMask = uint8(payLen)
	}

	// Set masking key if client (not server)
	if !c.server {
		frame.header.payloadMask |= 0x80 // Set mask bit
		frame.header.maskingKey = make([]byte, 4)
		binary.BigEndian.PutUint32(frame.header.maskingKey, rand.Uint32())
		ApplyMask(&frame.data, frame.header.maskingKey)
	}

	// //mask data if from the client
	// if !c.server {
	// 	ApplyMask(&frame.data, frame.header.maskingKey)
	// }

	//write header to data
	//frame.data = append(writeHeader, frame.data...)
	return frame, nil
}

func (c *WsInstance) MessageFromStart() ([]byte, error) {
	return c.readFromStart()
}

func (c *WsInstance) readFromStart() ([]byte, error) {
	msgs := []byte{}
	for {
		frame := WsFrame{}
		if len(c.readBuffer) == 0 {
			break
		}
		//fmt.Printf("Buffer: %v\r\n", c.readBuffer)
		frame = WsFrame{
			header: WsHeader{
				msgType:     OPCODE(c.readBuffer[0]),
				payloadMask: c.readBuffer[1],
				maskingKey:  []byte{},
			},
			data: []byte{},
		}
		var payloadSize int = int(frame.header.payloadMask & 0x7F)
		if payloadSize > 125 {
			// we have an extended payload, determine which one.
			if payloadSize == 126 {
				// next 2 bytes are the extended size.
				frame.header.extPayload = uint64(binary.BigEndian.Uint16(c.readBuffer[2:4]))
				payloadSize = int(frame.header.extPayload)
			} else {
				// next 8 bytes are extended size.
				frame.header.extPayload = uint64(binary.BigEndian.Uint64(c.readBuffer[2:10]))
				payloadSize = int(frame.header.extPayload)
			}
		}
		if frame.header.payloadMask>>7 == 1 {
			//frame is masked from client to server
			if payloadSize < 126 {
				frame.header.maskingKey = []byte(c.readBuffer[2:6])
			} else if payloadSize <= 65535 {
				frame.header.maskingKey = []byte(c.readBuffer[4:8])
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
		// what if there is more than one frame in the buffer?
		frame.data = c.readBuffer[index : index+payloadSize]
		if frame.header.payloadMask>>7 == 1 && c.server {
			DecodeMask(&frame.data, frame.header.maskingKey)
		}
		if frame.header.payloadMask>>7 == 1 && !c.server {
			logger.Println("err: client received masked frame.")
		}

		// prep the buffer for the next frame
		//fmt.Printf("%v, %v\r\n", payloadSize, index)
		c.readBuffer = c.readBuffer[index+payloadSize:]
		logger.Printf("Rec frame MsgType: %v\r\n", frame.header.msgType&0x7F)
		// read the data depending on what the opcode is.
		switch frame.header.msgType & 0x7F {
		case Text:
			fmt.Printf("Rec Text frame: %s\r\n", string(frame.data))
			msgs = append(msgs, frame.data...)
		case Bin:
			fmt.Printf("Rec Bin frame: %v\r\n", []byte(frame.data))
			msgs = append(msgs, frame.data...)
		case Cont:
			// this will need to be aware of the previous frame metadata.
			msgs = append(msgs, frame.data...)
		case ConClose:
			// send the close, then close the connection.
			c.WriteMessage([]byte(frame.data), ConClose)
			c.conn.Close()
			c.state = CLOSED
			return msgs, errors.New("connection closed")
		case Ping:
			c.WriteMessage([]byte(frame.data), Ping)
		case Pong:
			c.WriteMessage([]byte(frame.data), Pong)
		}

	}
	return msgs, nil
}

func (c *WsInstance) readFrame() (WsFrame, error) {
	c.readBuffer = make([]byte, 1024)
	frame := WsFrame{}
	if c.state == CLOSED {
		return WsFrame{}, errors.New("connection closed")
	}
	n, err := c.conn.Read(c.readBuffer)
	readBuffer := make([]byte, n)
	copy(readBuffer, c.readBuffer[:n])
	// we need to ensure we have enough data to read the frame, otherwise we need to read more data.
	if err != nil {
		logger.Printf("Connection closed / read error: %v\r\n", err)
		return WsFrame{}, err
	}
	if n < 2 {
		logger.Println("not enough data to read frame", n)
		n, err = c.conn.Read(c.readBuffer)
		if err != nil {
			logger.Printf("Connection closed / read error: %v\r\n", err)
			return WsFrame{}, err
		}
		readBuffer = append(readBuffer, c.readBuffer[:n]...)
	}
	for {
		if len(readBuffer) == 0 {
			break
		}
		frame = WsFrame{
			header: WsHeader{
				msgType:     OPCODE(readBuffer[0]),
				payloadMask: readBuffer[1],
				maskingKey:  []byte{},
			},
			data: []byte{},
		}
		var payloadSize uint64 = uint64(frame.header.payloadMask & 0x7F)
		// continue reading until the buffer contains the payload
		if payloadSize > 125 {
			// we have an extended payload, determine which one.
			if payloadSize == 126 {
				// next 2 bytes are the extended size.
				frame.header.extPayload = uint64(binary.BigEndian.Uint16(readBuffer[2:4]))
				payloadSize = uint64(frame.header.extPayload)
			} else {
				// next 8 bytes are extended size.
				frame.header.extPayload = uint64(binary.BigEndian.Uint64(readBuffer[2:10]))
				payloadSize = uint64(frame.header.extPayload)
			}
		}
		for uint64(len(readBuffer)) < payloadSize {
			n, err := c.conn.Read(c.readBuffer)
			if err != nil {
				return WsFrame{}, err
			}
			readBuffer = append(readBuffer, c.readBuffer[:n]...)
		}
		// Calculate the base index for masking key and payload
		baseIndex := 2 // Start after the first two bytes (opcode and payload mask)
		if payloadSize > 125 {
			if payloadSize <= 65535 {
				baseIndex += 2 // Add 2 bytes for 16-bit length
			} else {
				baseIndex += 8 // Add 8 bytes for 64-bit length
			}
		}

		if frame.header.payloadMask>>7 == 1 {
			//frame is masked from client to server
			frame.header.maskingKey = readBuffer[baseIndex : baseIndex+4]
			baseIndex += 4 // Move past the masking key
		}
		for uint64(len(readBuffer)) < payloadSize+uint64(baseIndex) {
			n, err := c.conn.Read(c.readBuffer)
			if err != nil {
				return WsFrame{}, err
			}
			readBuffer = append(readBuffer, c.readBuffer[:n]...)
		}

		if (frame.header.msgType&0x7f == Ping || frame.header.msgType&0x7f == Pong || frame.header.msgType&0x7f == ConClose) && payloadSize > 125 {
			c.WriteMessage([]byte{0x03, 0xEA}, ConClose)
			logger.Println("connection closing: protocol error")
			return WsFrame{}, errors.New("connection closing: protocol error")
		}

		// Ensure we have enough data in the buffer
		if uint64(len(readBuffer)) < uint64(baseIndex)+payloadSize {
			logger.Println("readbuffer len", len(readBuffer), "baseIndex", baseIndex, "payloadSize", payloadSize)
			return WsFrame{}, errors.New("buffer too small for payload")
		}

		frame.data = readBuffer[baseIndex : uint64(baseIndex)+payloadSize]
		if frame.header.payloadMask>>7 == 1 && c.server {
			DecodeMask(&frame.data, frame.header.maskingKey)
		}
		if frame.header.payloadMask>>7 == 1 && !c.server {
			logger.Println("err: client received masked frame.")
		}
		readBuffer = readBuffer[uint64(baseIndex)+payloadSize:]
		logger.Printf("Rec frame MsgType: %v\r\n", frame.header.msgType&0x7F)
		logger.Printf("Rec frame length: %v\r\n", payloadSize)
		// read the data depending on what the opcode is.
		switch frame.header.msgType & 0x7F {
		case Text:
			c.WriteMessage(frame.data, Text)
			logger.Printf("Rec Text frame: %v\r\n", string(frame.data))
		case Bin:
			c.WriteMessage(frame.data, Bin)
			logger.Printf("Rec Bin frame: %v\r\n", []byte(frame.data))
		case Cont:
			// this will need to be aware of the previous frame metadata.
		case ConClose:
			// send the close, then close the connection.
			c.WriteMessage([]byte(frame.data), ConClose)
			logger.Printf("Rec Close frame: %v\r\n", string(frame.data))
			// if c.state == CLOSING {
			// 	c.conn.Close()
			// 	c.state = CLOSED
			// }
			// c.state = CLOSING
			time.Sleep(5 * time.Millisecond)
			c.state = CLOSED
			c.conn.Close()
		case Ping:
			c.WriteMessage([]byte(frame.data), Pong)
		case Pong:
			//c.WriteMessage([]byte(frame.data), Pong)
		}
	}
	return frame, nil
}
