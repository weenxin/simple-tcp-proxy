package proxy

import (
	"bytes"
)

func IsGoodRequest(protocol []byte) bool {
	return protocol[0] == byte('Q')
}

func FormatRequest(protocol []byte) ([]byte, error) {
	//结束 protocol 只有一个字节
	if protocol[0] == byte('Z') {
		return protocol[0:1], nil
	}
	if protocol[0] != byte('D') {
		return nil, ErrResponseProtocolFormat
	}
	if index := bytes.IndexByte(protocol[1:], byte('D')); index >= 0 {
		return protocol[:index+1], nil
	}

	if index := bytes.IndexByte(protocol[1:], byte('Z')); index >= 0 {
		return protocol[:index+1], nil
	}

	if len(protocol) == MaxProtocolLength {
		return nil, ErrMaxResponseProtocolSizeExceeded
	}

	return nil, nil
}

func IsEndResponse(protocol []byte) bool {
	return len(protocol) == 1 && protocol[0] == byte('Z')
}
