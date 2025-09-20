package waitpid

import (
	"context"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

func TestLinux(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider, err := GetProvider()
	if err != nil {
		t.Fatalf("failed to create docker provider: %v", err)
	}
	defer provider.Close()

	container, err := provider.RunContainer(ctx, testcontainers.ContainerRequest{
		Image:        "ubuntu:22.04",
		ExposedPorts: []string{},
		Cmd:          []string{"tail", "-f", "/dev/null"},
	})
	if err != nil {
		t.Fatalf("failed to run container: %v", err)
	}
	defer container.Terminate(ctx)

	if err != nil {
		if runtime.GOOS == "linux" {
			t.Log("this issue may be caused by disabled podman socket or by selinux")
		}
		t.Fatalf("failed to start container: %v", err)
	}

	compiledBinaryPath, err := CompileFor("linux")
	if err != nil {
		t.Fatalf("failed to compile test binary: %v", err)
	}
	defer os.Remove(compiledBinaryPath)

	err = container.CopyFileToContainer(ctx, compiledBinaryPath, "/app", 0755)
	if err != nil {
		t.Fatalf("failed to copy test binary to container: %v", err)
	}

	// this will spawn sleep process and print it's pid to stdout
	// 1000 is greater than 600, default go test timeout
	log, _, err := ExecGetLog(ctx, t, container, []string{"/bin/bash", "-c", "sleep 1000 & echo $!"})
	if err != nil {
		t.Fatalf("failed to spawn sleep process: %v", err)
	}

	pid, err := strconv.Atoi(log[:len(log)-1])
	if err != nil {
		t.Fatalf("failed to convert container output to pid from (%s): %v", log, err)
	}

	c := make(chan int64)
	e := make(chan error)

	t.Logf("will wait for pid: %d", pid)
	go func() {
		log, _, err := ExecGetLog(ctx, t, container, []string{"/app", strconv.Itoa(pid)})
		if err != nil {
			e <- err
		}
		startedTime, waitedTime, err := ParseTestBinaryOutput(log)
		if err != nil {
			e <- err
		}
		t.Logf("started time: %d", startedTime)
		t.Logf("waited time: %d", waitedTime)
		c <- waitedTime
	}()

	randomDealy := rand.Intn(int(1 * time.Second))
	time.Sleep(time.Duration(randomDealy))

	killTime := time.Now().UnixNano()
	_, _, err = ExecGetLog(ctx, t, container, []string{"kill", strconv.Itoa(pid)})
	if err != nil {
		t.Fatalf("failed to kill process with pid %d: %v", pid, err)
	}

	select {
	case err := <-e:
		t.Fatalf("failed to wait for test binary to finish: %v", err)
	case waitedTime := <-c:
		t.Logf("test binary finished")
		killTimeDiff := waitedTime - killTime
		t.Logf("kill time diff: %d", killTimeDiff)
		if killTimeDiff > int64(time.Second) {
			t.Fatalf("kill time diff is too big: %d", killTimeDiff)
		}
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatalf("timeout waiting for test binary to finish")
	}

}
