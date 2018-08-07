package iotest

import "net"

type UdpConn struct {
	addrIndex  int
	localAddr  net.Addr
	remoteAddr net.Addr
	inBytes    []byte
}

func (c *UdpConn) Context() interface{}       { return nil }
func (c *UdpConn) SetContext(ctx interface{}) {}
func (c *UdpConn) AddrIndex() int             { return c.addrIndex }
func (c *UdpConn) LocalAddr() net.Addr        { return c.localAddr }
func (c *UdpConn) RemoteAddr() net.Addr       { return c.remoteAddr }