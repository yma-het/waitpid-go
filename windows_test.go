package waitpid

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

func parseTestBinaryOutput(output string) (int64, int64, error) {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return 0, 0, errors.New("invalid line format")
	}
	if !strings.HasPrefix(lines[0], "started waiting") {
		return 0, 0, errors.New("started waiting not found")
	}
	if !strings.HasPrefix(lines[1], "waited") {
		return 0, 0, errors.New("waited not found")
	}
	startedTimeString := strings.TrimPrefix(lines[0], "started waiting: ")
	waitedTimeString := strings.TrimPrefix(lines[1], "waited: ")
	startedTime, err := strconv.ParseInt(startedTimeString, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	waitedTime, err := strconv.ParseInt(waitedTimeString, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return startedTime, waitedTime, nil
}

func TestWindows(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get current dir: %v", err)
	}

	// Запускаем контейнер с Wine
	req := testcontainers.ContainerRequest{
		Image:        "scottyhardy/docker-wine",
		ExposedPorts: []string{},
		//WaitingFor:   wait.ForLog("waited"),
		Cmd: []string{"tail", "-f", "/dev/null"},
		Mounts: testcontainers.ContainerMounts{
			{
				Source: testcontainers.GenericBindMountSource{
					HostPath: currentDir,
				},
				Target:   "/app",
				ReadOnly: false,
			},
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if runtime.GOOS == "linux" {
			t.Log("this issue may be caused by disabled podman socket or by selinux")
		}
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	if err := InstallGolang(ctx, t, container); err != nil {
		t.Fatalf("failed to install golang: %v", err)
	}

	if err := CompileTestBinary(ctx, t, container, "windows"); err != nil {
		t.Fatalf("failed to build test binary: %v", err)
	}

	// this will spawn sleep process and print it's pid to stdout
	// FIXME: REVERT INTERVAL
	//log, err := ExecGetLog(ctx, t, container, []string{"/bin/bash", "-c", "sleep 10 & echo $!"})
	// 1000 is greater than 600, default go test timeout
	log, err := ExecGetLog(ctx, t, container, []string{"wine", "cmd.exe", "/c", "START /B ping -n 1000 127.0.0.1  && tasklist"})
	if err != nil {
		t.Fatalf("failed to spawn sleep process: %v", err)
	}

	t.Logf("log: %s", log)

	// pid, err := strconv.Atoi(log[:len(log)-1])
	// if err != nil {
	// 	t.Fatalf("failed to convert container output to pid from (%s): %v", log, err)
	// }
	pid, err := getPingPid(log)
	if err != nil {
		t.Fatalf("failed to get ping pid from (%s): %v", log, err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	//var startedTime, waitedTime int64
	c := make(chan int64)

	t.Logf("will wait for pid: %d", pid)
	t.Logf("will wait for pid hex: %s", strconv.FormatInt(int64(pid), 16))
	// FIXME: wait for funcs and call t.Fatal inside main goroutine
	go func() {
		log, err := ExecGetLog(ctx, t, container, []string{"wine", "/app/example/example.exe", strconv.Itoa(pid)}) //strconv.FormatInt(int64(pid), 16)})
		if err != nil {
			t.Fatalf("failed to spawn test binary process or error waiting for it to finish: %v", err)
		}
		startedTime, waitedTime, err := parseTestBinaryOutput(log)
		if err != nil {
			t.Fatalf("failed to parse test binary output: %v", err)
		}
		t.Logf("started time: %d", startedTime)
		t.Logf("waited time: %d", waitedTime)
		c <- waitedTime
	}()

	randomDealy := rand.Intn(int(1 * time.Second))
	time.Sleep(time.Duration(randomDealy))

	killTime := time.Now().UnixNano()
	_, err = ExecGetLog(ctx, t, container, []string{"wine", "cmd.exe", "/c", "taskkill /F /IM ping.exe"})
	if err != nil {
		t.Fatalf("failed to kill ping process: %v", err)
	}

	select {
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

	//time.Sleep(10000)

}
