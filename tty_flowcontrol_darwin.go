//go:build darwin

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func disableTTYFlowControl() {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return
	}
	termios.Iflag &^= unix.IXON | unix.IXOFF | unix.IXANY
	_ = unix.IoctlSetTermios(fd, unix.TIOCSETA, termios)
}
