package waitpid

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

func init() {
	// Disable Ryuk (reaper) to avoid relying on the Docker "bridge" network
	// which is absent in rootless Podman (default network is "podman").
	// Testcontainers will still remove containers via Terminate().
	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
}

func ExecGetLog(ctx context.Context, t *testing.T, container testcontainers.Container, cmd []string, options ...tcexec.ProcessOption) (string, error) {
	t.Logf("will run: %s", cmd)
	// processOption := tcexec.WithEnv([]string{"LS_COLORS=\"\""})
	// options = append(options, processOption)
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
	_, err := ExecGetLog(ctx, t, c, []string{"wget", "--output-file=/dev/null", "--output-document=/tmp/go.tar.gz", golangUrl})
	if err != nil {
		return fmt.Errorf("failed to get golang: %v", err)
	}
	log, err := ExecGetLog(ctx, t, c, []string{"tar", "-C", "/usr/local", "-xzf", "/tmp/go.tar.gz"})
	if err != nil {
		return fmt.Errorf("failed to extract golang: %v", err)
	}
	t.Logf("go archive extraction log: %s", log)
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
	t.Logf("successfully installed golang into container")
	return nil
}

func CompileTestBinary(ctx context.Context, t *testing.T, c testcontainers.Container, os string) error {
	// FIXME: Copy before building and make mount RO
	optionCwd := tcexec.WithWorkingDir("/app/example")
	optionOS := tcexec.WithEnv([]string{fmt.Sprintf("GOOS=%s", os)})

	log, err := ExecGetLog(ctx, t, c, []string{"go", "build", "example.go"}, optionCwd, optionOS)
	if err != nil {
		return fmt.Errorf("failed to comtile go test binary: %v", err)
	}
	t.Logf("test app build log: %s", log)
	return nil
}
