package iotest

import (
	"errors"
	"sync"
	"net"
	"runtime"
	"time"
	"sync/atomic"
	"io"
)

var errClosing = errors.New("closing")
var errCloseConns = errors.New("close conns")

type Loop struct {
	idx   int               // loop index
	ch    chan interface{}  // command channel
	conns map[*OioConn]bool // track all the conns bound to this loop
}

type OioConn struct {
	addrIndex   int
	localAddr   net.Addr
	remoteAddr  net.Addr
	conn        net.Conn    // original connection
	ctx         interface{} // user-defined context
	loop        *Loop       // owner loop
	listenerIdx int         // index of Listener
	extraData   []byte      // extra data for done connection
	done        int32       // 0: attached, 1: closed, 2: detached
}

func (conn *OioConn) Context() interface{}       { return conn.ctx }
func (conn *OioConn) SetContext(ctx interface{}) { conn.ctx = ctx }
func (conn *OioConn) AddrIndex() int             { return conn.addrIndex }
func (conn *OioConn) LocalAddr() net.Addr        { return conn.localAddr }
func (conn *OioConn) RemoteAddr() net.Addr       { return conn.remoteAddr }

type OioServer struct {
	events      Events         // user events
	loops       []*Loop        // all the loops
	listeners   []*Listener    // all the listeners
	loopWg      sync.WaitGroup // loop close waitgroup
	listenerWg  sync.WaitGroup // Listener close waitgroup
	cond        *sync.Cond     // shutdown signaler
	error       error          // signal error
	acceptCount uintptr        // accept counter
}
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

type stdin struct {
	c  *OioConn
	in []byte
}
type stderr struct {
	c   *OioConn
	err error
}

// waitForShutdown waits for a signal to shutdown
func (server *OioServer) waitForShutdown() error {
	server.cond.L.Lock()
	server.cond.Wait()
	err := server.error
	server.cond.L.Unlock()
	return err
}

// signalShutdown signals a shutdown an begins server closing
func (server *OioServer) signalShutdown(err error) {
	server.cond.L.Lock()
	server.error = err
	server.cond.Signal()
	server.cond.L.Unlock()
}

func stdserve(events Events, listeners []*Listener) error {
	numLoops := events.NumLoops
	if numLoops <= 0 {
		if numLoops == 0 {
			numLoops = 1
		} else {
			numLoops = runtime.NumCPU()
		}
	}

	s := &OioServer{}
	s.events = events
	s.listeners = listeners
	s.cond = sync.NewCond(&sync.Mutex{})

	//println("-- server starting")
	if events.Serving != nil {
		var svr Server
		svr.NumLoops = numLoops
		svr.Addrs = make([]net.Addr, len(listeners))
		for i, ln := range listeners {
			svr.Addrs[i] = ln.listenerAddr
		}
		action := events.Serving(svr)
		switch action {
		case Shutdown:
			return nil
		}
	}
	for i := 0; i < numLoops; i++ {
		s.loops = append(s.loops, &Loop{
			idx:   i,
			ch:    make(chan interface{}),
			conns: make(map[*OioConn]bool),
		})
	}
	var ferr error
	defer func() {
		// wait on a signal for shutdown
		ferr = s.waitForShutdown()

		// notify all loops to close by closing all listeners
		for _, l := range s.loops {
			l.ch <- errClosing
		}

		// wait on all loops to main loop channel events
		s.loopWg.Wait()

		// shutdown all listeners
		for i := 0; i < len(s.listeners); i++ {
			s.listeners[i].close()
		}

		// wait on all listeners to complete
		s.listenerWg.Wait()

		// close all connections
		s.loopWg.Add(len(s.loops))
		for _, l := range s.loops {
			l.ch <- errCloseConns
		}
		s.loopWg.Wait()

	}()
	s.loopWg.Add(numLoops)
	for i := 0; i < numLoops; i++ {
		go loopRun(s, s.loops[i])
	}
	s.listenerWg.Add(len(listeners))
	for i := 0; i < len(listeners); i++ {
		go listenerRun(s, listeners[i], i)
	}
	return ferr
}

func listenerRun(server *OioServer, listener *Listener, listenerIndex int) {
	var ferr error
	defer func() {
		server.signalShutdown(ferr)
		server.listenerWg.Done()
	}()
	var packet [0xFFFF]byte
	for {
		if listener.packetConn != nil {
			// udp
			n, addr, err := listener.packetConn.ReadFrom(packet[:])
			if err != nil {
				ferr = err
				return
			}
			loop := server.loops[int(atomic.AddUintptr(&server.acceptCount, 1))%len(server.loops)]
			loop.ch <- &UdpConn{
				addrIndex:  listenerIndex,
				localAddr:  listener.listenerAddr,
				remoteAddr: addr,
				inBytes:    append([]byte{}, packet[:n]...),
			}
		} else {
			// tcp
			conn, err := listener.listener.Accept()
			if err != nil {
				ferr = err
				return
			}
			loop := server.loops[int(atomic.AddUintptr(&server.acceptCount, 1))%len(server.loops)]
			oioConn := &OioConn{conn: conn, loop: loop, listenerIdx: listenerIndex}
			loop.ch <- oioConn
			go func(c *OioConn) {
				var packet [0xFFFF]byte
				for {
					n, err := c.conn.Read(packet[:])
					if err != nil {
						c.conn.SetReadDeadline(time.Time{})
						loop.ch <- &stderr{c, err}
						return
					}
					loop.ch <- &stdin{c, append([]byte{}, packet[:n]...)}
				}
			}(oioConn)
		}
	}
}
func loopRun(server *OioServer, loop *Loop) {
	var err error
	tick := make(chan bool)
	tock := make(chan time.Duration)
	defer func() {
		//fmt.Println("-- loop stopped --", loop.idx)
		if loop.idx == 0 && server.events.Tick != nil {
			close(tock)
			go func() {
				for range tick {
				}
			}()
		}
		server.signalShutdown(err)
		server.loopWg.Done()
		loopEgress(server, loop)
		server.loopWg.Done()
	}()
	if loop.idx == 0 && server.events.Tick != nil {
		go func() {
			for {
				tick <- true
				delay, ok := <-tock
				if !ok {
					break
				}
				time.Sleep(delay)
			}
		}()
	}
	//fmt.Println("-- loop started --", loop.idx)
	for {
		select {
		case <-tick:
			delay, action := server.events.Tick()
			switch action {
			case Shutdown:
				err = errClosing
			}
			tock <- delay
		case v := <-loop.ch:
			switch v := v.(type) {
			case error:
				err = v
			case *OioConn:
				err = loopAccept(server, loop, v)
			case *stdin:
				err = loopRead(server, loop, v.c, v.in)
			case *UdpConn:
				err = loopReadUDP(server, loop, v)
			case *stderr:
				err = loopError(server, loop, v.c, v.err)
			}
		}
		if err != nil {
			return
		}
	}
}

func loopEgress(s *OioServer, l *Loop) {
	var closed bool
loop:
	for v := range l.ch {
		switch v := v.(type) {
		case error:
			if v == errCloseConns {
				closed = true
				for c := range l.conns {
					loopClose(s, l, c)
				}
			}
		case *stderr:
			loopError(s, l, v.c, v.err)
		}
		if len(l.conns) == 0 && closed {
			break loop
		}
	}
}

type DetachedConn struct {
	conn    net.Conn // original conn
	inBytes []byte   // extra input data
}

func (conn *DetachedConn) Read(p []byte) (n int, err error) {
	if len(conn.inBytes) > 0 {
		if len(conn.inBytes) <= len(p) {
			copy(p, conn.inBytes)
			n = len(conn.inBytes)
			conn.inBytes = nil
			return
		}
		copy(p, conn.inBytes[:len(p)])
		n = len(p)
		conn.inBytes = conn.inBytes[n:]
		return
	}
	return conn.conn.Read(p)
}

func (c *DetachedConn) Write(p []byte) (n int, err error) {
	return c.conn.Write(p)
}

func (c *DetachedConn) Close() error {
	return c.conn.Close()
}
func loopError(s *OioServer, l *Loop, conn *OioConn, err error) error {
	delete(l.conns, conn)
	closeEvent := true
	switch atomic.LoadInt32(&conn.done) {
	case 0: // read error
		conn.conn.Close()
		if err == io.EOF {
			err = nil
		}
	case 1: // closed
		conn.conn.Close()
		err = nil
	case 2: // detached
		err = nil
		if s.events.Detached == nil {
			conn.conn.Close()
		} else {
			closeEvent = false
			switch s.events.Detached(conn, &DetachedConn{conn.conn, conn.extraData}) {
			case Shutdown:
				return errClosing
			}
		}
	}
	if closeEvent {
		if s.events.Closed != nil {
			switch s.events.Closed(conn, err) {
			case Shutdown:
				return errClosing
			}
		}
	}
	return nil
}

func loopRead(s *OioServer, l *Loop, c *OioConn, in []byte) error {
	if atomic.LoadInt32(&c.done) == 2 {
		// should not ignore reads for detached connections
		c.extraData = append(c.extraData, in...)
		return nil
	}
	if s.events.Data != nil {
		out, action := s.events.Data(c, in)
		if len(out) > 0 {
			if s.events.PreWrite != nil {
				s.events.PreWrite()
			}
			c.conn.Write(out)
		}
		switch action {
		case Shutdown:
			return errClosing
		case Detach:
			return loopDetach(s, l, c)
		case Close:
			return loopClose(s, l, c)
		}
	}
	return nil
}

func loopDetach(s *OioServer, l *Loop, c *OioConn) error {
	atomic.StoreInt32(&c.done, 2)
	c.conn.SetReadDeadline(time.Now())
	return nil
}

func loopClose(s *OioServer, l *Loop, c *OioConn) error {
	atomic.StoreInt32(&c.done, 1)
	c.conn.SetReadDeadline(time.Now())
	return nil
}

func loopAccept(s *OioServer, l *Loop, c *OioConn) error {
	l.conns[c] = true
	c.addrIndex = c.listenerIdx
	c.localAddr = s.listeners[c.listenerIdx].listenerAddr
	c.remoteAddr = c.conn.RemoteAddr()

	if s.events.Opened != nil {
		out, opts, action := s.events.Opened(c)
		if len(out) > 0 {
			if s.events.PreWrite != nil {
				s.events.PreWrite()
			}
			c.conn.Write(out)
		}
		if opts.TCPKeepAlive > 0 {
			if c, ok := c.conn.(*net.TCPConn); ok {
				c.SetKeepAlive(true)
				c.SetKeepAlivePeriod(opts.TCPKeepAlive)
			}
		}
		switch action {
		case Shutdown:
			return errClosing
		case Detach:
			return loopDetach(s, l, c)
		case Close:
			return loopClose(s, l, c)
		}
	}
	return nil
}

func loopReadUDP(s *OioServer, l *Loop, c *UdpConn) error {
	if s.events.Data != nil {
		out, action := s.events.Data(c, c.inBytes)
		if len(out) > 0 {
			if s.events.PreWrite != nil {
				s.events.PreWrite()
			}
			s.listeners[c.addrIndex].packetConn.WriteTo(out, c.remoteAddr)
		}
		switch action {
		case Shutdown:
			return errClosing
		}
	}
	return nil
}