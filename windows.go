//go:build windows
// +build windows

package waitpid

import "fmt"

type windowsWaitHandle struct{}

func (l windowsWaitHandle) Open(pid int) (int32, error) {
	fmt.Println("open windows")
	return 0, nil
}

func (l windowsWaitHandle) Wait(fd int32) error {
	fmt.Println("wait windows")
	return nil
}

func getWaitHandle() WaitHandle {
	return windowsWaitHandle{}
}
