package main

import (
	"fmt"

	"github.com/yma-het/waitpid-go"
)

func main() {
	w := waitpid.GetWaitHandle()
	// windows
	//fd, err := w.Open(0x114)
	fd, err := w.Open(48)
	if err != nil {
		fmt.Println("w.Open: %v", err)
		return
	}
	fmt.Println("fd: %d", fd)
	err = w.Wait(fd)
	if err != nil {
		fmt.Println("w.Wait: %v", err)
		return
	}

}
