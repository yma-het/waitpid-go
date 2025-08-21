package waitpid

type WaitHandle interface {
	Open(pid int) (int32, error)
	Wait(fd int32) error
}

func GetWaitHandle() WaitHandle {
	return getWaitHandle()
}
