//go:build linux
// +build linux

package waitpid

import (
	"golang.org/x/sys/unix"
)

type linuxWaitHandle struct{}

func (l linuxWaitHandle) Open(pid int) (int32, error) {
	// 0 stands for blocking mode
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return -2, err
	}
	return int32(fd), nil
}

func (l linuxWaitHandle) Wait(fd int32) error {
	pollFd := unix.PollFd{
		Fd:     fd,
		Events: unix.POLLIN | unix.POLLHUP | unix.POLLERR,
	}
	pollFds := []unix.PollFd{pollFd}

	// -1 stands for infinite await
	_, err := unix.Poll(pollFds, -1)
	if err != nil {
		return err
	}
	return nil
}

func getWaitHandle() WaitHandle {
	return linuxWaitHandle{}
}
