package iotest

import "net"

type DetachedConn struct {
	Conn    net.Conn // original Conn
	InBytes []byte   // extra input data
}

func (conn *DetachedConn) Read(p []byte) (n int, err error) {
	if len(conn.InBytes) > 0 {
		if len(conn.InBytes) <= len(p) {
			copy(p, conn.InBytes)
			n = len(conn.InBytes)
			conn.InBytes = nil
			return
		}
		copy(p, conn.InBytes[:len(p)])
		n = len(p)
		conn.InBytes = conn.InBytes[n:]
		return
	}
	return conn.Conn.Read(p)
}

func (conn *DetachedConn) Write(p []byte) (n int, err error) {
	return conn.Conn.Write(p)
}

func (conn *DetachedConn) Close() error {
	return conn.Conn.Close()
}