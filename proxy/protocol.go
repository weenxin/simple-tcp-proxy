package proxy

import (
	"bytes"
)

const (
	ProtocolStartChar = 'D'
	ResponseEndChar   = 'Z'
	RequestStartChar  = 'Q'
)

func IsGoodRequest(data []byte) bool {
	return data[0] == byte(RequestStartChar)
}

//FormatProtocol 封装一个数据包，返回数据包在 data 数据中的片段，不会新建和copy
func FormatProtocol(data []byte) ([]byte, error) {
	//结束 data 只有一个字节
	if data[0] == byte(ResponseEndChar) {
		return data[0:1], nil
	}
	if data[0] != byte(ProtocolStartChar) {
		return nil, ErrResponseProtocolFormat
	}
	if index := bytes.IndexByte(data[1:], byte(ProtocolStartChar)); index >= 0 {
		return data[:index+1], nil
	}

	if index := bytes.IndexByte(data[1:], byte(ResponseEndChar)); index >= 0 {
		return data[:index+1], nil
	}

	if len(data) == MaxProtocolLength {
		return nil, ErrMaxResponseProtocolSizeExceeded
	}

	//其实这里还是可以改进的，如果server端一定是一个完整的protocol发送，那么即使没有找到Z或者D，说明刚好收到一包数据。
	return nil, nil
}

//IsEndResponse 是否最后一帧，代表protocol结束
func IsEndResponse(data []byte) bool {
	return len(data) == 1 && data[0] == byte(ResponseEndChar)
}
