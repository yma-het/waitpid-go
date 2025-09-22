package waitpid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

const PODMAN_ENGINE_NAME = "Podman Engine"

func GoBinaryPath() (string, error) {
	if goroot := runtime.GOROOT(); goroot != "" {
		ext := ""
		if runtime.GOOS == "windows" {
			ext = ".exe"
		}
		p := filepath.Join(goroot, "bin", "go"+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("go"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("cannot find go: no %q/bin/go, no in PATH", runtime.GOROOT())
}

func CompileFor(targetOS string) (string, error) {
	tempFile, err := os.CreateTemp("", "gobinary")
	if err != nil {
		return "", err
	}

	goPath, err := GoBinaryPath()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(goPath, "build", "-o", tempFile.Name(), "example.go")
	cmd.Dir = "example"
	cmd.Env = append(os.Environ(), "GOOS="+targetOS)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func GetProvider() (testcontainers.ContainerProvider, error) {

	// there is a bug in testcontainers-go
	// it will run container with default network name "bridge" even if the engine is Podman
	// so we need to detect the engine and set the default network name accordingly

	defaultNetworkNmae := testcontainers.Bridge
	cli, err := testcontainers.NewDockerClientWithOpts(context.Background())
	if err != nil {
		return nil, err
	}
	serverVersion, err := cli.ServerVersion(context.Background())
	if err != nil {
		return nil, err
	}
	for _, versionComponent := range serverVersion.Components {
		if versionComponent.Name == PODMAN_ENGINE_NAME {
			defaultNetworkNmae = testcontainers.Podman
		}
	}
	return testcontainers.NewDockerProvider(
		testcontainers.WithDefaultBridgeNetwork(defaultNetworkNmae),
	)
}

func ExecGetLog(ctx context.Context, t *testing.T, container testcontainers.Container, cmd []string, options ...tcexec.ProcessOption) (string, int, error) {
	t.Logf("will run: %s", cmd)
	// processOption := tcexec.WithEnv([]string{"LS_COLORS=\"\""})
	// options = append(options, processOption)
	exitCode, r, err := container.Exec(ctx, cmd, options...)
	if err != nil {
		return "", -1, fmt.Errorf("failed to exec: %w", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, r)
	if err != nil {
		t.Fatalf("failed to demultiplex output: %v", err)
	}
	if exitCode != 0 {
		return "", exitCode, fmt.Errorf("got non-zero exit code %d\nstdout: %s\nstderr: %s", exitCode, stdout.String(), stderr.String())
	}
	return stdout.String(), exitCode, nil
}

func InstallGolang(ctx context.Context, t *testing.T, c testcontainers.Container) error {
	checkWgetPresent, exitCode, err := ExecGetLog(ctx, t, c, []string{"which", "wget"})
	if err != nil && exitCode == 1 && checkWgetPresent == "" {
		_, _, err = ExecGetLog(ctx, t, c, []string{"apt-get", "update"})
		if err != nil {
			return fmt.Errorf("failed to update apt: %v", err)
		}
		_, _, err = ExecGetLog(ctx, t, c, []string{"apt-get", "install", "-y", "wget"})
		if err != nil {
			return fmt.Errorf("failed to install wget: %v", err)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to check if wget is present: %v", err)
	}
	golangUrl := fmt.Sprintf("https://go.dev/dl/%s.linux-%s.tar.gz", runtime.Version(), runtime.GOARCH)
	_, _, err = ExecGetLog(ctx, t, c, []string{"wget", "--output-file=/dev/null", "--output-document=/tmp/go.tar.gz", golangUrl})
	if err != nil {
		return fmt.Errorf("failed to get golang: %v", err)
	}
	log, _, err := ExecGetLog(ctx, t, c, []string{"tar", "-C", "/usr/local", "-xzf", "/tmp/go.tar.gz"})
	if err != nil {
		return fmt.Errorf("failed to extract golang: %v", err)
	}
	t.Logf("go archive extraction log: %s", log)
	_, _, err = ExecGetLog(ctx, t, c, []string{"rm", "/tmp/go.tar.gz"})
	if err != nil {
		return fmt.Errorf("failed to cleanup golang binary: %v", err)
	}
	_, _, err = ExecGetLog(ctx, t, c, []string{"ln", "-sf", "/usr/local/go/bin/go", "/usr/bin/go"})
	if err != nil {
		return fmt.Errorf("failed to create symlink to golang binary: %v", err)
	}
	_, _, err = ExecGetLog(ctx, t, c, []string{"ln", "-sf", "/usr/local/go/bin/gofmt", "/usr/bin/gofmt"})
	if err != nil {
		return fmt.Errorf("failed to create symlink to gofmt binary: %v", err)
	}
	t.Logf("successfully installed golang into container")
	return nil
}

func CompileTestBinary(ctx context.Context, t *testing.T, c testcontainers.Container, os string) error {
	optionCwd := tcexec.WithWorkingDir("/app/example")
	optionOS := tcexec.WithEnv([]string{fmt.Sprintf("GOOS=%s", os)})

	log, _, err := ExecGetLog(ctx, t, c, []string{"go", "build", "example.go"}, optionCwd, optionOS)
	if err != nil {
		return fmt.Errorf("failed to comtile go test binary: %v", err)
	}
	t.Logf("test app build log: %s", log)
	return nil
}

func ParseTestBinaryOutput(output string) (int64, int64, error) {
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
