package websocket

// masking only used by the client when sending data to server.
import (
	"math/rand/v2"
)

func getMask() []byte {
	maskingKey := rand.Uint32()
	keyArr := make([]byte, 4)
	for i := range keyArr {
		keyArr[i] = byte(maskingKey >> (i * 8))
	}
	return keyArr
}

// mask should be a random int32 xored to the data
// TODO: check if frame is also masked.
func ApplyMask(data *[]byte) {
	maskingKey := getMask()
	pos := 0
	for i := 0; i < len(*data); i++ {
		pos = i & 3
		(*data)[i] = (*data)[i] ^ maskingKey[pos]
	}
}
