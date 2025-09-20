package multilistener

import (
	"errors"
	"net"
	"slices"
	"testing"
	"time"
)

func TestListen(t *testing.T) {
	t.Parallel()

	t.Run("without addresses", func(t *testing.T) {
		t.Parallel()
		if _, err := Listen(t.Context(), nil); err == nil {
			t.Fatal("listen() didn't fail")
		}
	})
	t.Run("invalid address", func(t *testing.T) {
		t.Parallel()
		addrs := []string{"127.0.0.1:0", "127.0.0.1:0", "invalid address"}
		if _, err := Listen(t.Context(), addrs); err == nil {
			t.Errorf("listen() didn't fail")
		}
	})
	t.Run("with addresses", func(t *testing.T) {
		t.Parallel()
		for n := 1; n <= 5; n++ {
			ln, err := Listen(t.Context(), freeAddrs(t, n))
			if err != nil {
				t.Fatalf("listen() failed on %d addresses: %v", n, err)
			}
			if err := ln.Close(); err != nil {
				t.Errorf("listener.Close() failed: %v", err)
			}
		}
	})
	t.Run("same address-port", func(t *testing.T) {
		t.Parallel()
		addr := freeAddrs(t, 1)[0]
		ln, err := Listen(t.Context(), []string{addr, addr, addr})
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}
		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
	})
}

func TestListener_Addr(t *testing.T) {
	t.Parallel()

	addrs := freeAddrs(t, 3)
	ln, err := Listen(t.Context(), addrs)
	if err != nil {
		t.Fatalf("listen() failed: %v", err)
	}
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
	})

	if ln.Addr().String() != addrs[0] {
		t.Errorf("Addr() = %q, want %q", ln.Addr().String(), addrs[0])
	}
}

func TestListen_Addrs(t *testing.T) {
	t.Parallel()

	addrs := freeAddrs(t, 3)
	ln, err := Listen(t.Context(), addrs)
	if err != nil {
		t.Fatalf("listen() failed: %v", err)
	}
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
	})

	var gotAddrs []string //nolint:prealloc
	for _, addr := range ln.Addrs() {
		gotAddrs = append(gotAddrs, addr.String())
	}
	if !slices.Equal(gotAddrs, addrs) {
		t.Errorf("Addr() = %q, want %q", gotAddrs, addrs)
	}
}

func TestListener_Close(t *testing.T) {
	t.Parallel()

	t.Run("close on close on close", func(t *testing.T) {
		t.Parallel()

		ln, err := Listen(t.Context(), freeAddrs(t, 3))
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}

		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
		if err := ln.Close(); !errors.Is(err, net.ErrClosed) {
			t.Errorf("already closed: listener.Close() = %v, want %v", err, net.ErrClosed)
		}
		if err := ln.Close(); !errors.Is(err, net.ErrClosed) {
			t.Errorf("already closed: listener.Close() = %v, want %v", err, net.ErrClosed)
		}
	})
	t.Run("without accepted connections", func(t *testing.T) {
		t.Parallel()

		ln, err := Listen(t.Context(), freeAddrs(t, 3))
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}

		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
		for _, ln := range ln.listeners {
			if err := ln.Close(); !errors.Is(err, net.ErrClosed) {
				t.Errorf("sub-listener not closed: net.Listener.Close() = %v, want %v", err, net.ErrClosed)
			}
		}
	})
	t.Run("with pending connections", func(t *testing.T) {
		t.Parallel()

		addrs := freeAddrs(t, 3)
		ln, err := Listen(t.Context(), addrs)
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}

		for _, addr := range addrs {
			if _, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr); err != nil {
				t.Fatalf("net.Dial(%q) failed: %v", addr, err)
			}
		}

		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
		for _, ln := range ln.listeners {
			if err := ln.Close(); !errors.Is(err, net.ErrClosed) {
				t.Errorf("sub-listener not closed: net.Listener.Close() = %v, want %v", err, net.ErrClosed)
			}
		}
	})
	t.Run("without pending connections", func(t *testing.T) {
		t.Parallel()

		addrs := freeAddrs(t, 3)
		ln, err := Listen(t.Context(), addrs)
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}

		for _, addr := range addrs {
			if _, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr); err != nil {
				t.Fatalf("net.Dial(%q) failed: %v", addr, err)
			}
			if _, err := ln.Accept(); err != nil {
				t.Errorf("listener.Accept() failed: %v", err)
			}
		}

		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}
		for _, ln := range ln.listeners {
			if err := ln.Close(); !errors.Is(err, net.ErrClosed) {
				t.Errorf("sub-listener not closed: net.Listener.Close() = %v, want %v", err, net.ErrClosed)
			}
		}
	})
}

func TestListener_Accept(t *testing.T) {
	t.Parallel()

	t.Run("all connections", func(t *testing.T) {
		t.Parallel()

		addrs := freeAddrs(t, 10)
		ln, err := Listen(t.Context(), addrs)
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}
		t.Cleanup(func() {
			if err := ln.Close(); err != nil {
				t.Errorf("listener.Close() failed: %v", err)
			}
		})

		var connAddrs []string
		for _, addr := range addrs {
			conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
			if err != nil {
				t.Fatalf("net.Dial(%q) failed: %v", addr, err)
			}
			connAddrs = append(connAddrs, conn.LocalAddr().String())
		}

		for i := range addrs {
			conn, err := ln.Accept()
			if err != nil {
				t.Errorf("listener.Accept() failed: %v", err)
			}
			if caddr := conn.LocalAddr().String(); caddr != addrs[i] {
				t.Errorf("net.Conn.LocalAddr() %q, want %q", caddr, addrs[i])
			}
			if caddr := conn.RemoteAddr().String(); caddr != connAddrs[i] {
				t.Errorf("net.Conn.RemoteAddr() %q, want %q", caddr, connAddrs[i])
			}
		}
	})
	t.Run("subset of connections", func(t *testing.T) {
		t.Parallel()

		addrs := freeAddrs(t, 10)
		ln, err := Listen(t.Context(), addrs)
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}
		t.Cleanup(func() {
			if err := ln.Close(); err != nil {
				t.Errorf("listener.Close() failed: %v", err)
			}
		})

		var connAddrs []string
		for _, addr := range addrs {
			conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
			if err != nil {
				t.Fatalf("net.Dial(%q) failed: %v", addr, err)
			}
			connAddrs = append(connAddrs, conn.LocalAddr().String())
		}

		for i := range 5 {
			conn, err := ln.Accept()
			if err != nil {
				t.Errorf("listener.Accept() failed: %v", err)
			}
			if caddr := conn.LocalAddr().String(); caddr != addrs[i] {
				t.Errorf("net.Conn.LocalAddr() %q, want %q", caddr, addrs[i])
			}
			if caddr := conn.RemoteAddr().String(); caddr != connAddrs[i] {
				t.Errorf("net.Conn.RemoteAddr() %q, want %q", caddr, connAddrs[i])
			}
		}
	})
	t.Run("after close", func(t *testing.T) {
		t.Parallel()

		addrs := freeAddrs(t, 10)
		ln, err := Listen(t.Context(), addrs)
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}

		for _, addr := range addrs {
			if _, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr); err != nil {
				t.Fatalf("net.Dial(%q) failed: %v", addr, err)
			}
		}
		if err := ln.Close(); err != nil {
			t.Errorf("listener.Close() failed: %v", err)
		}

		for range addrs {
			if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
				t.Errorf("listener.Accept() %v, want %v", err, net.ErrClosed)
			}
		}
	})
	t.Run("remove sub-listener on accept error", func(t *testing.T) {
		t.Parallel()

		addr := freeAddrs(t, 1)
		ln, err := Listen(t.Context(), addr)
		if err != nil {
			t.Fatalf("listen() failed: %v", err)
		}

		if _, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr[0]); err != nil {
			t.Fatalf("net.Dial(%q) failed: %v", addr, err)
		}

		if _, err := ln.Accept(); err != nil {
			t.Errorf("listener.Accept() failed: %v", err)
		}

		// Make the sub-listener return an error on Accept().
		if err := ln.listeners[0].Close(); err != nil {
			t.Fatalf("net.Listener.Close() failed: %v", err)
		}
		if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
			t.Errorf("listener.Accept() %v, want %v", err, net.ErrClosed)
		}

		// Subsequent calls to Accept() should be blocked.
		acc := make(chan struct{})
		go func() {
			_, _ = ln.Accept()
			close(acc)
		}()
		select {
		case <-acc:
			t.Error("listener.Accept() unexpectedly returned")
		case <-time.After(50 * time.Millisecond):
		}
	})
}

func freeAddrs(t *testing.T, count int) []string {
	t.Helper()

	var addrs []string //nolint:prealloc
	for range count {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("ResolveTCPAddr() failed: %v", err)
		}

		ln, err := net.ListenTCP("tcp", addr)
		if err != nil {
			t.Fatalf("ListenTCP() failed: %v", err)
		}
		_ = ln.Close()
		addrs = append(addrs, ln.Addr().String())
	}
	return addrs
}
