package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/yma-het/waitpid-go"
)

func main() {
	// disable printing time
	log.SetFlags(0)
	stdoutLogger := log.New(os.Stdout, "", 0)
	if len(os.Args) != 2 {
		log.Printf("waitpid demo arguments: PID")
		return
	}
	pidStr := os.Args[1]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Fatalf("strconv.Atoi: %v", err)
	}
	w := waitpid.GetWaitHandle()
	fd, err := w.Open(pid)
	if err != nil {
		log.Fatalf("w.Open: %v", err)
	}
	stdoutLogger.Printf("started waiting: %d\n", time.Now().UnixNano())
	err = w.Wait(fd)
	if err != nil {
		log.Fatalf("w.Wait: %v", err)
	}
	stdoutLogger.Printf("waited: %d\n", time.Now().UnixNano())
}
