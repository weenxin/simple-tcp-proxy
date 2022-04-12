package server

type Server interface {
	Connect() (Client, error)
}

type Client interface {
	Read(data []byte) (int, error)
	//Request 可以返一个io.Reader，简单起见，直接分两步吧
	Request([]byte) error
}
