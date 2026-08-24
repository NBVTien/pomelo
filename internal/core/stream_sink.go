package core

import (
	"net"

	"github.com/pomelohq/pomelo/internal/stream"
)

func pumpPtyOutput(sock net.Conn, sink stream.Sink) {
	buf := make([]byte, 32*1024)
	for {
		n, err := sock.Read(buf)
		if n > 0 {
			if sink.SendBinary(buf[:n]) != nil {
				return
			}
		}
		if err != nil {
			_ = sink.SendText([]byte(`{"type":"exit"}`))
			_ = sink.Close()
			return
		}
	}
}

func forwardBytes(sink stream.Sink, ch <-chan []byte, done <-chan struct{}) {
	for {
		select {
		case b := <-ch:
			if sink.SendJSONBytes(b) != nil {
				return
			}
		case <-done:
			return
		}
	}
}
