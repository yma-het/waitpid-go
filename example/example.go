package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/yma-het/waitpid-go"
)

func main() {
	pidStr := os.Args[1]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Println("strconv.Atoi: %v", err)
		return
	}
	w := waitpid.GetWaitHandle()
	// windows
	//fd, err := w.Open(0x114)
	fd, err := w.Open(pid)
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
