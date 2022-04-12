package proxy

import (
	"bytes"
)

const (
	ProtocolStartChar = 'D'
	ResponseEndChar   = 'Z'
	RequestStartChar  = 'Q'
)

func IsGoodRequest(protocol []byte) bool {
	return protocol[0] == byte(RequestStartChar)
}

func FormatRequest(protocol []byte) ([]byte, error) {
	//结束 protocol 只有一个字节
	if protocol[0] == byte(ResponseEndChar) {
		return protocol[0:1], nil
	}
	if protocol[0] != byte(ProtocolStartChar) {
		return nil, ErrResponseProtocolFormat
	}
	if index := bytes.IndexByte(protocol[1:], byte(ProtocolStartChar)); index >= 0 {
		return protocol[:index+1], nil
	}

	if index := bytes.IndexByte(protocol[1:], byte(ResponseEndChar)); index >= 0 {
		return protocol[:index+1], nil
	}

	if len(protocol) == MaxProtocolLength {
		return nil, ErrMaxResponseProtocolSizeExceeded
	}

	return nil, nil
}

func IsEndResponse(protocol []byte) bool {
	return len(protocol) == 1 && protocol[0] == byte(ResponseEndChar)
}
