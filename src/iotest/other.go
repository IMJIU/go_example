// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build !darwin,!netbsd,!freebsd,!openbsd,!dragonfly,!linux

package iotest

import (
	"errors"
	"net"
	"os"
)

func (listener *Listener) close() {
	if listener.listener != nil {
		listener.listener.Close()
	}
	if listener.packetConn != nil {
		listener.packetConn.Close()
	}
	if listener.network == "unix" {
		os.RemoveAll(listener.addr)
	}
}

func (listener *Listener) system() error {
	return nil
}

func serve(events Events, listeners []*Listener) error {
	return stdserve(events, listeners)
}

func reuseportListenPacket(proto, addr string) (l net.PacketConn, err error) {
	return nil, errors.New("reuseport is not available")
}

func reuseportListen(proto, addr string) (l net.Listener, err error) {
	return nil, errors.New("reuseport is not available")
}
