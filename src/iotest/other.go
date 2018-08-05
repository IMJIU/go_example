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

func (lisener *Listener) close() {
	if lisener.listener != nil {
		lisener.listener.Close()
	}
	if lisener.packetConn != nil {
		lisener.packetConn.Close()
	}
	if lisener.network == "unix" {
		os.RemoveAll(lisener.addr)
	}
}

func (ln *Listener) system() error {
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
