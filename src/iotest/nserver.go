package iotest

import (
	"time"
	"net"
	"io"
	"os"
	"strings"
)

// Action is an action that occurs after the completion of an event.
type Action int

const (
	// None indicates that no action should occur following an event.
	None Action = iota
	// Detach detaches a connection. Not available for UDP connections.
	Detach
	// Close closes the connection.
	Close
	// Shutdown shutdowns the server.
	Shutdown
)

// Options are set when the client opens.
type Options struct {
	// TCPKeepAlive (SO_KEEPALIVE) socket option.
	TCPKeepAlive time.Duration
	// ReuseInputBuffer will forces the connection to share and reuse the
	// same input packet buffer with all other connections that also use
	// this option.
	// Default value is false, which means that all input data which is
	// passed to the Data event will be a uniquely copied []byte slice.
	ReuseInputBuffer bool
}

type Server struct {
	Addrs    []net.Addr
	NumLoops int
}

// Conn is an evio connection.
type Conn interface {
	// Context returns a user-defined context.
	Context() interface{}
	// SetContext sets a user-defined context.
	SetContext(interface{})
	// AddrIndex is the index of server address that was passed to the Serve call.
	AddrIndex() int
	// LocalAddr is the connection's local socket address.
	LocalAddr() net.Addr
	// RemoteAddr is the connection's remote peer address.
	RemoteAddr() net.Addr
}

// LoadBalance sets the load balancing method.
type LoadBalance int

const (
	Random           LoadBalance = iota
	RoundRobin
	LeastConnections
)

type Events struct {
	NumLoops    int
	LoadBalance LoadBalance
	Serving     func(server Server) (action Action)
	Opened      func(conn Conn) (outBytes []byte, opts Options, action Action)
	Closed      func(conn Conn, err error) (action Action)
	Detached    func(conn Conn, rwc io.ReadWriteCloser) (action Action)
	PreWrite    func()
	Data        func(conn Conn, inBytes []byte) (outBytes []byte, action Action)
	Tick        func() (delay time.Duration, action Action)
}

type AddrOpts struct {
	reusePort bool
}

type Listener struct {
	listener     net.Listener
	listenerAddr net.Addr
	packetConn   net.PacketConn
	opts         AddrOpts
	file         *os.File
	fd           int
	network      string
	addr         string
}

func Serve(events Events, addr ...string) error {
	var listeners []*Listener
	defer func() {
		for _, listener := range listeners {
			listener.close()
		}
	}()
	var stdlib bool
	for _, addr := range addr {
		var listener Listener
		var stdlibt bool
		listener.network, listener.addr, listener.opts, stdlibt = parseAddr(addr)
		if stdlibt {
			stdlib = true
		}
		if listener.network == "unix" {
			os.RemoveAll(listener.addr)
		}
		var err error
		if listener.network == "udp" {
			if listener.opts.reusePort {
				listener.packetConn, err = reuseportListenPacket(listener.network, listener.addr)
			} else {
				listener.packetConn, err = net.ListenPacket(listener.network, listener.addr)
			}
		} else {
			if listener.opts.reusePort {
				listener.listener, err = reuseportListen(listener.network, listener.addr)
			} else {
				listener.listener, err = net.Listen(listener.network, listener.addr)
			}
		}
		if err != nil {
			return err
		}
		if listener.packetConn != nil {
			listener.listenerAddr = listener.packetConn.LocalAddr()
		} else {
			listener.listenerAddr = listener.listener.Addr()
		}
		if !stdlib {
			if err := listener.system(); err != nil {
				return err
			}
		}
		listeners = append(listeners, &listener)
	}
	if stdlib {
		return stdserve(events, listeners)
	}
	return serve(events, listeners)
}

func parseAddr(addr string) (proxool, address string, opts AddrOpts, stdlib bool) {
	proxool = "tcp"
	address = addr
	opts.reusePort = false
	if strings.Contains(address, "://") {
		proxool = strings.Split(address, "://")[0]
		address = strings.Split(address, "://")[1]
	}
	if strings.HasSuffix(proxool, "-net") {
		stdlib = true
		proxool = proxool[:len(proxool)-4]
	}
	index := strings.Index(address, "?")
	if index != -1 {
		for _, part := range strings.Split(address[index+1:], "&") {
			kv := strings.Split(part, "=")
			if len(kv) == 2 {
				switch kv[0] {
				case "reuseport":
					if len(kv[1]) != 0 {
						switch kv[1][0] {
						default:
							opts.reusePort = kv[1][0] >= '1' && kv[1][0] <= '9'
						case 'T', 't', 'Y', 'y':
							opts.reusePort = true
						}
					}
				}
			}
		}
		address = address[:index]
	}
	return
}
