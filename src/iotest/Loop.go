package iotest


type Loop struct {
	Idx   int                      // Loop index
	Ch    chan interface{}         // command channel
	Conns map[*OioConn]bool // track all the conns bound to this Loop
}
