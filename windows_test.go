package waitpid

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

func getPingPid(output string) (int, error) {
	lines := strings.Split(output, "\n")
	// skip headers and delimiters:
	//Image Name                     PID Session Name        Session#    Mem Usage
	//========================= ======== ================ =========== ============
	//start.exe                       32 Console                    1     16,328 K

	lines = lines[2:]
	for _, line := range lines {
		if strings.Contains(line, "ping.exe") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, errors.New("invalid line format")
			}
			return strconv.Atoi(fields[1])
		}
	}
	return 0, errors.New("ping.exe not found")
}
func TestWindows(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider, err := GetProvider()
	if err != nil {
		t.Fatalf("failed to create docker provider: %v", err)
	}
	defer provider.Close()

	container, err := provider.RunContainer(ctx, testcontainers.ContainerRequest{
		Image: "scottyhardy/docker-wine",
		Cmd:   []string{"tail", "-f", "/dev/null"},
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
	defer container.Terminate(ctx)

	compiledBinaryPath, err := CompileFor("windows")
	if err != nil {
		t.Fatalf("failed to compile test binary: %v", err)
	}
	defer os.Remove(compiledBinaryPath)

	err = container.CopyFileToContainer(ctx, compiledBinaryPath, "/app.exe", 0755)
	if err != nil {
		t.Fatalf("failed to copy test binary to container: %v", err)
	}

	// this will spawn sleep process and print it's pid to stdout
	// 1000 is greater than 600, default go test timeout
	log, _, err := ExecGetLog(ctx, t, container, []string{"wine", "cmd.exe", "/c", "START /B ping -n 1000 127.0.0.1  && tasklist"})
	if err != nil {
		t.Fatalf("failed to spawn sleep process: %v", err)
	}

	pid, err := getPingPid(log)
	if err != nil {
		t.Fatalf("failed to get ping pid from (%s): %v", log, err)
	}

	c := make(chan int64)
	e := make(chan error)

	t.Logf("will wait for pid: %d", pid)
	go func() {
		log, _, err := ExecGetLog(ctx, t, container, []string{"wine", "/app.exe", strconv.Itoa(pid)})
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
	_, _, err = ExecGetLog(ctx, t, container, []string{"wine", "cmd.exe", "/c", "taskkill /F /IM ping.exe"})
	if err != nil {
		t.Fatalf("failed to kill ping process: %v", err)
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
