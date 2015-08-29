package debug

import (
	"io"
	"log"
	"net"
	"time"
)

type Conn struct {
	io.Reader
	io.Writer
	conn net.Conn
}

func WrapConn(c net.Conn) *Conn {
	rl := NewReadLogger("<", c)
	wl := NewWriteLogger(">", c)

	return &Conn{
		Reader: rl,
		Writer: wl,
		conn:   c,
	}
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) SetDeadline(t time.Time) error {
	log.Println("WARNING: SetDeadline() not sure this works")
	return c.conn.SetDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	log.Println("WARNING: SetReadDeadline() not sure this works")
	return c.conn.SetReadDeadline(t)
}
func (c *Conn) SetWriteDeadline(t time.Time) error {
	log.Println("WARNING: SetWriteDeadline() not sure this works")
	return c.conn.SetWriteDeadline(t)
}
