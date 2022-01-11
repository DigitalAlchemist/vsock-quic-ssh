package utils

import (
	"net"
)

type VSockConn struct {
	net.Conn
}

func (c *VSockConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Read(p)
	addr = c.RemoteAddr()
	return
}

func (c *VSockConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	n, err = c.Write(p)
	return
}
