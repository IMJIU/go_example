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

type OioInByte struct {
	c  *OioConn
	in []byte
}
type StdErr struct {
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

	server := &OioServer{}
	server.events = events
	server.listeners = listeners
	server.cond = sync.NewCond(&sync.Mutex{})

	//println("-- server starting")
	if events.OnStart != nil {
		var svr Server
		svr.LoopCnt = numLoops
		svr.Addrs = make([]net.Addr, len(listeners))
		for i, ln := range listeners {
			svr.Addrs[i] = ln.listenerAddr
		}
		action := events.OnStart(svr)
		switch action {
		case Shutdown:
			return nil
		}
	}
	for i := 0; i < numLoops; i++ {
		server.loops = append(server.loops, &Loop{
			idx:   i,
			ch:    make(chan interface{}),
			conns: make(map[*OioConn]bool),
		})
	}
	var ferr error
	defer func() {
		// wait on a signal for shutdown
		ferr = server.waitForShutdown()

		// notify all loops to close by closing all listeners
		for _, loop := range server.loops {
			loop.ch <- errClosing
		}

		// wait on all loops to main loop channel events
		server.loopWg.Wait()

		// shutdown all listeners
		for i := 0; i < len(server.listeners); i++ {
			server.listeners[i].close()
		}

		// wait on all listeners to complete
		server.listenerWg.Wait()

		// close all connections
		server.loopWg.Add(len(server.loops))
		for _, l := range server.loops {
			l.ch <- errCloseConns
		}
		server.loopWg.Wait()

	}()
	server.loopWg.Add(numLoops)
	for i := 0; i < numLoops; i++ {
		go loopRun(server, server.loops[i])
	}
	server.listenerWg.Add(len(listeners))
	for i := 0; i < len(listeners); i++ {
		go listenerRun(server, listeners[i], i)
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
						loop.ch <- &StdErr{c, err}
						return
					}
					loop.ch <- &OioInByte{c, append([]byte{}, packet[:n]...)}
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
			case *OioInByte:
				err = loopRead(server, loop, v.c, v.in)
			case *UdpConn:
				err = loopReadUDP(server, loop, v)
			case *StdErr:
				err = loopError(server, loop, v.c, v.err)
			}
		}
		if err != nil {
			return
		}
	}
}

func loopEgress(server *OioServer, loop *Loop) {
	var closed bool
loop:
	for v := range loop.ch {
		switch v := v.(type) {
		case error:
			if v == errCloseConns {
				closed = true
				for c := range loop.conns {
					loopClose(server, loop, c)
				}
			}
		case *StdErr:
			loopError(server, loop, v.c, v.err)
		}
		if len(loop.conns) == 0 && closed {
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
func loopError(server *OioServer, loop *Loop, conn *OioConn, err error) error {
	delete(loop.conns, conn)
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
		if server.events.Detached == nil {
			conn.conn.Close()
		} else {
			closeEvent = false
			switch server.events.Detached(conn, &DetachedConn{conn.conn, conn.extraData}) {
			case Shutdown:
				return errClosing
			}
		}
	}
	if closeEvent {
		if server.events.Closed != nil {
			switch server.events.Closed(conn, err) {
			case Shutdown:
				return errClosing
			}
		}
	}
	return nil
}

func loopRead(server *OioServer, loop *Loop, oioConn *OioConn, inBytes []byte) error {
	if atomic.LoadInt32(&oioConn.done) == 2 {
		// should not ignore reads for detached connections
		oioConn.extraData = append(oioConn.extraData, inBytes...)
		return nil
	}
	if server.events.Data != nil {
		out, action := server.events.Data(oioConn, inBytes)
		if len(out) > 0 {
			if server.events.PreWrite != nil {
				server.events.PreWrite()
			}
			oioConn.conn.Write(out)
		}
		switch action {
		case Shutdown:
			return errClosing
		case Detach:
			return loopDetach(server, loop, oioConn)
		case Close:
			return loopClose(server, loop, oioConn)
		}
	}
	return nil
}

func loopDetach(s *OioServer, loop *Loop, c *OioConn) error {
	atomic.StoreInt32(&c.done, 2)
	c.conn.SetReadDeadline(time.Now())
	return nil
}

func loopClose(s *OioServer, l *Loop, c *OioConn) error {
	atomic.StoreInt32(&c.done, 1)
	c.conn.SetReadDeadline(time.Now())
	return nil
}

func loopAccept(server *OioServer, loop *Loop, oioConn *OioConn) error {
	loop.conns[oioConn] = true
	oioConn.addrIndex = oioConn.listenerIdx
	oioConn.localAddr = server.listeners[oioConn.listenerIdx].listenerAddr
	oioConn.remoteAddr = oioConn.conn.RemoteAddr()

	if server.events.Opened != nil {
		out, opts, action := server.events.Opened(oioConn)
		if len(out) > 0 {
			if server.events.PreWrite != nil {
				server.events.PreWrite()
			}
			oioConn.conn.Write(out)
		}
		if opts.TCPKeepAlive > 0 {
			if c, ok := oioConn.conn.(*net.TCPConn); ok {
				c.SetKeepAlive(true)
				c.SetKeepAlivePeriod(opts.TCPKeepAlive)
			}
		}
		switch action {
		case Shutdown:
			return errClosing
		case Detach:
			return loopDetach(server, loop, oioConn)
		case Close:
			return loopClose(server, loop, oioConn)
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