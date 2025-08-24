//go:build darwin || freebsd
// +build darwin freebsd

package waitpid

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

type bsdWaitHandle struct{}

func (bwh bsdWaitHandle) Open(pid int) (int32, error) {
	if pid <= 0 {
		return -2, fmt.Errorf("invalid PID %d", pid)
	}

	kq, err := unix.Kqueue()
	if err != nil {
		return -2, fmt.Errorf("failed to create kqueue: %w", err)
	}

	var changes [1]unix.Kevent_t
	changes[0] = unix.Kevent_t{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD,
		Fflags: unix.NOTE_EXIT,
		Data:   0,
		Udata:  nil,
	}

	_, err = unix.Kevent(kq, changes[:], nil, nil)
	if err != nil {
		unix.Close(kq)
		return -2, fmt.Errorf("failed to register kevent: %w", err)
	}

	return int32(kq), nil
}

func (bwh bsdWaitHandle) Wait(fd int32) error {

	var events [1]unix.Kevent_t

	_, err := unix.Kevent(int(fd), nil, events[:], nil)
	if err != nil {
		return fmt.Errorf("kevent failed: %w", err)
	}

	if events[0].Filter == unix.EVFILT_PROC && (events[0].Fflags&unix.NOTE_EXIT) != 0 {
		return nil
	}

	return errors.New("unexpected kevent result")
}

func getWaitHandle() WaitHandle {
	return bsdWaitHandle{}
}
