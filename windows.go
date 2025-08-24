//go:build windows
// +build windows

package waitpid

import (
	"errors"
	"fmt"
	"math/rand"

	"golang.org/x/sys/windows"
)

const PROCESS_SYNCHRONIZE = 0x00100000

type windowsWaitHandle struct {
	fakeFdToHandle map[int32]windows.Handle
}

func (wh *windowsWaitHandle) Open(pid int) (int32, error) {
	handle, err := windows.OpenProcess(PROCESS_SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return -2, err
	}
	if handle == 0 {
		return -2, errors.New("failed to open process handle")
	}
	fakeFd := rand.Int31()
	wh.fakeFdToHandle[fakeFd] = handle
	return fakeFd, nil
}

func (wh *windowsWaitHandle) Wait(fd int32) error {
	// FIXME: May be close handle?
	defer func() {
		delete(wh.fakeFdToHandle, fd)
	}()
	handle, ok := wh.fakeFdToHandle[fd]
	if !ok {
		return errors.New("wait handle for given fd is not found. you must call Open() first")
	}

	ret, err := windows.WaitForSingleObject(handle, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("failed to wait pid: %e", err)
	}

	if ret == windows.WAIT_OBJECT_0 {
		return nil
	}
	return fmt.Errorf("failed to wait pid. return code: %d", ret)
}

func getWaitHandle() WaitHandle {
	wh := windowsWaitHandle{
		fakeFdToHandle: make(map[int32]windows.Handle),
	}
	return &wh
}
