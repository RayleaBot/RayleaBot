package bilibili

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/andybalholm/brotli"
)

const (
	liveWSHeaderSize  = 16
	liveWSProtoRaw    = 0
	liveWSProtoZlib   = 2
	liveWSProtoBrotli = 3

	liveWSOpHeartbeat      = 2
	liveWSOpHeartbeatReply = 3
	liveWSOpNotice         = 5
	liveWSOpVerify         = 7
	liveWSOpVerifyReply    = 8
)

func liveWSPack(body []byte, protocolVersion int, operation int) []byte {
	packetLength := liveWSHeaderSize + len(body)
	buffer := bytes.NewBuffer(make([]byte, 0, packetLength))
	_ = binary.Write(buffer, binary.BigEndian, uint32(packetLength))
	_ = binary.Write(buffer, binary.BigEndian, uint16(liveWSHeaderSize))
	_ = binary.Write(buffer, binary.BigEndian, uint16(protocolVersion))
	_ = binary.Write(buffer, binary.BigEndian, uint32(operation))
	_ = binary.Write(buffer, binary.BigEndian, uint32(1))
	buffer.Write(body)
	return buffer.Bytes()
}

func liveWSUnpack(data []byte) ([]map[string]any, error) {
	if len(data) < liveWSHeaderSize {
		return nil, nil
	}
	protocol := int(binary.BigEndian.Uint16(data[6:8]))
	operation := int(binary.BigEndian.Uint32(data[8:12]))
	if operation == liveWSOpHeartbeatReply || operation == liveWSOpVerifyReply {
		return nil, nil
	}
	if protocol == liveWSProtoZlib {
		reader, err := zlib.NewReader(bytes.NewReader(data[liveWSHeaderSize:]))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		inflated, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		return liveWSUnpack(inflated)
	}
	if protocol == liveWSProtoBrotli {
		inflated, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data[liveWSHeaderSize:])))
		if err != nil {
			return nil, err
		}
		return liveWSUnpack(inflated)
	}
	result := []map[string]any{}
	offset := 0
	for offset+liveWSHeaderSize <= len(data) {
		packetLength := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if packetLength <= liveWSHeaderSize || offset+packetLength > len(data) {
			break
		}
		op := int(binary.BigEndian.Uint32(data[offset+8 : offset+12]))
		if op == liveWSOpNotice {
			body := data[offset+liveWSHeaderSize : offset+packetLength]
			var item map[string]any
			if err := json.Unmarshal(body, &item); err == nil {
				result = append(result, item)
			}
		}
		offset += packetLength
	}
	if len(result) == 0 && protocol == liveWSProtoRaw && operation == liveWSOpNotice {
		body := data[liveWSHeaderSize:]
		var item map[string]any
		if err := json.Unmarshal(body, &item); err == nil {
			result = append(result, item)
		}
	}
	return result, nil
}
