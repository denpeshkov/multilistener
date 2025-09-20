package multilistener

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

func control(c syscall.RawConn) error {
	var sockErr error
	err := c.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if sockErr != nil {
			return
		}
		sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if sockErr != nil {
			return
		}
	})
	return errors.Join(err, sockErr)
}
