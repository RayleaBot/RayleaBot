package live

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/andybalholm/brotli"
)

const (
	WSHeaderSize  = 16
	WSProtoRaw    = 0
	WSProtoZlib   = 2
	WSProtoBrotli = 3

	WSOpHeartbeat      = 2
	WSOpHeartbeatReply = 3
	WSOpNotice         = 5
	WSOpVerify         = 7
	WSOpVerifyReply    = 8
)

func Pack(body []byte, protocolVersion int, operation int) []byte {
	packetLength := WSHeaderSize + len(body)
	buffer := bytes.NewBuffer(make([]byte, 0, packetLength))
	_ = binary.Write(buffer, binary.BigEndian, uint32(packetLength))
	_ = binary.Write(buffer, binary.BigEndian, uint16(WSHeaderSize))
	_ = binary.Write(buffer, binary.BigEndian, uint16(protocolVersion))
	_ = binary.Write(buffer, binary.BigEndian, uint32(operation))
	_ = binary.Write(buffer, binary.BigEndian, uint32(1))
	buffer.Write(body)
	return buffer.Bytes()
}

func Unpack(data []byte) ([]map[string]any, error) {
	if len(data) < WSHeaderSize {
		return nil, nil
	}
	protocol := int(binary.BigEndian.Uint16(data[6:8]))
	operation := int(binary.BigEndian.Uint32(data[8:12]))
	if operation == WSOpHeartbeatReply || operation == WSOpVerifyReply {
		return nil, nil
	}
	if protocol == WSProtoZlib {
		reader, err := zlib.NewReader(bytes.NewReader(data[WSHeaderSize:]))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		inflated, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		return Unpack(inflated)
	}
	if protocol == WSProtoBrotli {
		inflated, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data[WSHeaderSize:])))
		if err != nil {
			return nil, err
		}
		return Unpack(inflated)
	}
	result := []map[string]any{}
	offset := 0
	for offset+WSHeaderSize <= len(data) {
		packetLength := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if packetLength <= WSHeaderSize || offset+packetLength > len(data) {
			break
		}
		op := int(binary.BigEndian.Uint32(data[offset+8 : offset+12]))
		if op == WSOpNotice {
			body := data[offset+WSHeaderSize : offset+packetLength]
			var item map[string]any
			if err := json.Unmarshal(body, &item); err == nil {
				result = append(result, item)
			}
		}
		offset += packetLength
	}
	if len(result) == 0 && protocol == WSProtoRaw && operation == WSOpNotice {
		body := data[WSHeaderSize:]
		var item map[string]any
		if err := json.Unmarshal(body, &item); err == nil {
			result = append(result, item)
		}
	}
	return result, nil
}
