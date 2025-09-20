// Package multilistener provides a TCP net.Listener that allows listening on multiple addresses.
package multilistener

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync/atomic"
	"syscall"
)

var _ net.Listener = (*Listener)(nil)

// Listener is a [net.Listener] for TCP networks that allows listening on multiple addresses.
type Listener struct {
	listeners []net.Listener
	conns     chan connErrPair
	closeCh   chan struct{}
	closed    atomic.Bool
}

type connErrPair struct {
	conn net.Conn
	err  error
}

// Listen returns a [Listener] to listen on provided addresses.
func Listen(ctx context.Context, addrs []string) (*Listener, error) {
	if len(addrs) == 0 {
		return nil, errors.New("no addresses to listen on")
	}

	lc := &net.ListenConfig{
		Control: func(_, _ string, conn syscall.RawConn) error {
			return control(conn)
		},
	}
	mln := &Listener{
		listeners: make([]net.Listener, 0, len(addrs)),
		conns:     make(chan connErrPair),
		closeCh:   make(chan struct{}),
	}

	for _, addr := range addrs {
		ln, lerr := lc.Listen(ctx, "tcp", addr)
		if lerr != nil {
			// Close all the listeners.
			cerr := mln.Close()
			return nil, errors.Join(lerr, cerr)
		}
		mln.listeners = append(mln.listeners, ln)
	}
	mln.listeners = slices.Clip(mln.listeners)

	mln.acceptLoop()
	return mln, nil
}

func (l *Listener) acceptLoop() {
	for _, ln := range l.listeners {
		go func() {
			for {
				conn, err := ln.Accept()
				select {
				case l.conns <- connErrPair{conn: conn, err: err}:
				case <-l.closeCh:
					if conn != nil {
						_ = conn.Close()
					}
					return
				}
				if err != nil {
					// Don't loop on Accept() returning an error.
					return
				}
			}
		}()
	}
}

// Accept implements [net.Listener.Accept].
// It waits for and returns a connection from any of the sub-listeners.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c.conn, c.err
	case <-l.closeCh:
		return nil, net.ErrClosed
	}
}

// Close implements [net.Listener.Close]. It closes all sub-listeners.
func (l *Listener) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return net.ErrClosed
	}

	close(l.closeCh)
	var err error
	for _, ln := range l.listeners {
		if cerr := ln.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

// Addr implements [net.Listener.Addr].
// It returns the address of the first sub-listener.
func (l *Listener) Addr() net.Addr {
	return l.listeners[0].Addr()
}

// Addrs returns the addresses of all sub-listeners.
func (l *Listener) Addrs() []net.Addr {
	addrs := make([]net.Addr, len(l.listeners))
	for i, ln := range l.listeners {
		addrs[i] = ln.Addr()
	}
	return addrs
}
