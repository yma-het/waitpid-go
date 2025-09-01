package waitpid

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

func ExecGetLog(ctx context.Context, t *testing.T, container testcontainers.Container, cmd []string, options ...tcexec.ProcessOption) (string, error) {
	t.Logf("will run: %s", cmd)
	processOption := tcexec.WithEnv([]string{"LS_COLORS=\"\""})
	options = append(options, processOption)
	exitCode, r, err := container.Exec(ctx, cmd, options...)
	if err != nil {
		return "", fmt.Errorf("failed to exec: %w", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, r)
	if err != nil {
		t.Fatalf("failed to demultiplex output: %v", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("got non-zero exit code %d\nstdout: %s\nstderr: %s", exitCode, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}

func InstallGolang(ctx context.Context, t *testing.T, c testcontainers.Container) error {
	golangUrl := fmt.Sprintf("https://go.dev/dl/%s.linux-%s.tar.gz", runtime.Version(), runtime.GOARCH)
	//_, _, err := container.Exec(ctx, []string{"wget", "-O", "/tmp/go.tar.gz", golangUrl})
	_, err := ExecGetLog(ctx, t, c, []string{"curl", "-s", "-L", "-o", "/tmp/go.tar.gz", golangUrl})
	//log, err := ExecGetLog(ctx, c, []string{"echo", golangUrl})
	if err != nil {
		return fmt.Errorf("failed to get golang: %v", err)
	}
	//t.Logf("go fetch log: %s", log)
	//_, _, err = c.Exec(ctx, []string{"tar", "-C", "/usr/local", "-xzf", "/tmp/go.tar.gz"})
	log, err := ExecGetLog(ctx, t, c, []string{"tar", "-C", "/usr/local", "-xzf", "/tmp/go.tar.gz"})
	if err != nil {
		return fmt.Errorf("failed to extract golang: %v", err)
	}
	t.Logf("go archive extraction log: %s", log)
	//_, _, err = c.Exec(ctx, []string{"rm", "/tmp/go.tar.gz"})
	_, err = ExecGetLog(ctx, t, c, []string{"rm", "/tmp/go.tar.gz"})
	if err != nil {
		return fmt.Errorf("failed to cleanup golang binary: %v", err)
	}
	_, err = ExecGetLog(ctx, t, c, []string{"ln", "-sf", "/usr/local/go/bin/go", "/usr/bin/go"})
	if err != nil {
		return fmt.Errorf("failed to create symlink to golang binary: %v", err)
	}
	_, err = ExecGetLog(ctx, t, c, []string{"ln", "-sf", "/usr/local/go/bin/gofmt", "/usr/bin/gofmt"})
	if err != nil {
		return fmt.Errorf("failed to create symlink to gofmt binary: %v", err)
	}
	t.Logf("cuccessfully installed golang into container")
	return nil
}

func TestHelloEmpty(t *testing.T) {
	ctx := context.Background()

	// Запускаем контейнер с Wine
	req := testcontainers.ContainerRequest{
		Image:        "cbea9b841d8", // FIXME: replace with propper external image
		ExposedPorts: []string{},
		WaitingFor:   wait.ForLog(""),
		Cmd:          []string{"tail", "-f", "/dev/null"}, // Держим контейнер живым
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(".", ""), "/app"),
		),
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

	// this will spawn sleep process and print it's pid to stdout
	log, err := ExecGetLog(ctx, t, container, []string{"/bin/bash", "-c", "sleep 10000 & echo $!"})
	if err != nil {
		t.Fatalf("failed to spawn sleep process: %v", err)
	}

	_, err = strconv.Atoi(log[:len(log)-1])
	if err != nil {
		t.Fatalf("failed to convert container output to pid from (%s): %v", log, err)
	}
}
