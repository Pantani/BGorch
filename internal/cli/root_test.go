package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunApplyStatusDoctor(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	specPath := writeSpecFile(t, workdir, "cli-cluster")

	if code := Run([]string{"apply", "-f", specPath, "--dry-run"}); code != 0 {
		t.Fatalf("apply --dry-run returned code %d", code)
	}
	if _, err := os.Stat(filepath.Join(workdir, ".bgorch", "render", "compose.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write compose artifact, stat err: %v", err)
	}

	if code := Run([]string{"status", "-f", specPath}); code != 0 {
		t.Fatalf("status returned code %d", code)
	}

	if code := Run([]string{"doctor", "-f", specPath}); code != 0 {
		t.Fatalf("doctor returned code %d", code)
	}
}

func TestRunApplyRejectsDryRunWithRuntimeExec(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	specPath := writeSpecFile(t, workdir, "cli-runtime-conflict")
	if code := Run([]string{"apply", "-f", specPath, "--dry-run", "--runtime-exec"}); code != 2 {
		t.Fatalf("expected argument error code 2, got %d", code)
	}
}

func TestRunApplyRejectsDryRunWithRequireRuntime(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	specPath := writeSpecFile(t, workdir, "cli-require-runtime-conflict")
	if code := Run([]string{"apply", "-f", specPath, "--dry-run", "--require-runtime"}); code != 2 {
		t.Fatalf("expected argument error code 2, got %d", code)
	}
}

func TestRunApplyRequireRuntimeUnsupportedBackendReturnsActionableError(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	specPath := writeSpecFileWithBackend(t, workdir, "cli-require-runtime-apply", "terraform")
	code, stderr := runWithCapturedStderr(t, []string{"apply", "-f", specPath, "--yes", "--require-runtime"})
	if code != 1 {
		t.Fatalf("expected runtime error code 1, got %d", code)
	}
	if !strings.Contains(stderr, "Runtime execution is unavailable for this backend.") {
		t.Fatalf("expected actionable runtime execution title, got: %s", stderr)
	}
	expectedNext := "Next: chainops apply -f " + specPath + " --yes"
	if !strings.Contains(stderr, expectedNext) {
		t.Fatalf("expected suggested next command %q, got: %s", expectedNext, stderr)
	}
}

func TestRunStatusRequireRuntimeUnsupportedBackendReturnsActionableError(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	specPath := writeSpecFileWithBackend(t, workdir, "cli-require-runtime-status", "terraform")
	code, stderr := runWithCapturedStderr(t, []string{"status", "-f", specPath, "--require-runtime"})
	if code != 1 {
		t.Fatalf("expected runtime error code 1, got %d", code)
	}
	if !strings.Contains(stderr, "Runtime observation is unavailable for this backend.") {
		t.Fatalf("expected actionable runtime observation title, got: %s", stderr)
	}
	expectedNext := "Next: chainops status -f " + specPath
	if !strings.Contains(stderr, expectedNext) {
		t.Fatalf("expected suggested next command %q, got: %s", expectedNext, stderr)
	}
}

func TestRunDoctorRequireRuntimeUnsupportedBackendReturnsActionableError(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	specPath := writeSpecFileWithBackend(t, workdir, "cli-require-runtime-doctor", "terraform")
	code, stderr := runWithCapturedStderr(t, []string{"doctor", "-f", specPath, "--require-runtime"})
	if code != 1 {
		t.Fatalf("expected runtime error code 1, got %d", code)
	}
	if !strings.Contains(stderr, "Runtime observation is unavailable for this backend.") {
		t.Fatalf("expected actionable runtime observation title, got: %s", stderr)
	}
	expectedNext := "Next: chainops doctor -f " + specPath
	if !strings.Contains(stderr, expectedNext) {
		t.Fatalf("expected suggested next command %q, got: %s", expectedNext, stderr)
	}
}

func writeSpecFile(t *testing.T, dir, clusterName string) string {
	return writeSpecFileWithBackend(t, dir, clusterName, "docker-compose")
}

func writeSpecFileWithBackend(t *testing.T, dir, clusterName, backend string) string {
	t.Helper()
	path := filepath.Join(dir, "cluster.yaml")
	raw := "apiVersion: bgorch.io/v1alpha1\n" +
		"kind: ChainCluster\n" +
		"metadata:\n" +
		"  name: " + clusterName + "\n" +
		"spec:\n" +
		"  family: generic\n" +
		"  plugin: generic-process\n" +
		"  runtime:\n" +
		"    backend: " + backend + "\n" +
		"  nodePools:\n" +
		"    - name: nodes\n" +
		"      replicas: 1\n" +
		"      template:\n" +
		"        workloads:\n" +
		"          - name: daemon\n" +
		"            mode: container\n" +
		"            image: alpine:3.20\n" +
		"            command: [\"sh\", \"-c\"]\n" +
		"            args: [\"sleep 3600\"]\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}
	return path
}

func runWithCapturedStderr(t *testing.T, args []string) (code int, stderr string) {
	t.Helper()

	origStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	defer func() {
		os.Stderr = origStderr
		_ = reader.Close()
	}()
	os.Stderr = writer

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		done <- buf.String()
	}()

	code = Run(args)
	_ = writer.Close()
	stderr = <-done
	return code, stderr
}
