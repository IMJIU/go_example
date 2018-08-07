package iotest

import "net"

type OioConn struct {
	addrIndex   int
	localAddr   net.Addr
	remoteAddr  net.Addr
	Conn        net.Conn    // original connection
	ctx         interface{} // user-defined context
	Loop        *Loop       // owner Loop
	listenerIdx int         // index of Listener
	extraData   []byte      // extra data for done connection
	done        int32       // 0: attached, 1: closed, 2: detached
}

func (conn *OioConn) Context() interface{}       { return conn.ctx }
func (conn *OioConn) SetContext(ctx interface{}) { conn.ctx = ctx }
func (conn *OioConn) AddrIndex() int             { return conn.addrIndex }
func (conn *OioConn) LocalAddr() net.Addr        { return conn.localAddr }
func (conn *OioConn) RemoteAddr() net.Addr       { return conn.remoteAddr }

