package utils

import (
	"fmt"
	"net"

	"github.com/lucas-clemente/quic-go"
)

type QuicStreamConnWrapper struct {
	quic.Stream
}

func (s *QuicStreamConnWrapper) Network() string {
	return "quic"
}

func (s *QuicStreamConnWrapper) String() string {
	return fmt.Sprintf("%s", s.StreamID())
}

func (s *QuicStreamConnWrapper) LocalAddr() net.Addr {
	return s
}

func (s *QuicStreamConnWrapper) RemoteAddr() net.Addr {
	return s
}
