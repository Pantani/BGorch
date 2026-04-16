package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChainopsRenderCanonicalSideEffectFree(t *testing.T) {
	repo := findRepoRoot(t)
	bin := buildChainopsCLI(t, repo)
	workDir := t.TempDir()

	specPath := filepath.Join(workDir, "chainops.yaml")
	spec := `metadata:
  name: canonical-demo
spec:
  family: generic
  runtime:
    backend: docker-compose
  nodePools:
    - name: validator
      template:
        workloads:
          - name: node
            mode: container
            image: ghcr.io/example/chaind:v0.1.0
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	res := runCLI(t, bin, workDir, "render", "-f", specPath, "-o", "yaml")
	requireExitCode(t, res, 0)
	requireContains(t, res.Stdout, "apiVersion: bgorch.io/v1alpha1")
	requireContains(t, res.Stdout, "plugin: generic-process")

	if _, err := os.Stat(filepath.Join(workDir, ".chainops", "render", "compose.yaml")); !os.IsNotExist(err) {
		t.Fatalf("canonical render should not write artifacts, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".chainops", "state", "canonical-demo--docker-compose.json")); !os.IsNotExist(err) {
		t.Fatalf("canonical render should not write snapshots, stat err: %v", err)
	}
}

func TestChainopsApplyRequiresYesInNonInteractive(t *testing.T) {
	repo := findRepoRoot(t)
	bin := buildChainopsCLI(t, repo)
	workDir := t.TempDir()
	specPath := filepath.Join(repo, "examples", "generic-single-compose.yaml")

	res := runCLI(t, bin, workDir, "apply", "-f", specPath, "--non-interactive")
	requireExitCode(t, res, 2)
	requireContains(t, res.Stderr, "Confirmation required for non-interactive apply.")

	res = runCLI(t, bin, workDir, "apply", "-f", specPath, "--yes", "--dry-run")
	requireExitCode(t, res, 0)
	requireContains(t, res.Stdout, "dry-run enabled")
}

func TestChainopsPlanOutAndApplyPlanFile(t *testing.T) {
	repo := findRepoRoot(t)
	bin := buildChainopsCLI(t, repo)
	workDir := t.TempDir()
	specPath := filepath.Join(repo, "examples", "generic-single-compose.yaml")
	planPath := filepath.Join(workDir, "plan.json")

	res := runCLI(t, bin, workDir, "plan", "-f", specPath, "--out", planPath)
	requireExitCode(t, res, 0)
	requireContains(t, res.Stdout, "Plan file written")

	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("expected plan file to exist: %v", err)
	}

	res = runCLI(t, bin, workDir, "apply", planPath, "--yes", "--dry-run")
	requireExitCode(t, res, 0)
	requireContains(t, res.Stdout, "dry-run enabled")
	if strings.TrimSpace(res.Stderr) != "" {
		if !strings.Contains(res.Stderr, "[WARN] plan drift detected") {
			t.Fatalf("unexpected stderr for apply plan file:\n%s", res.Stderr)
		}
	}
}

func buildChainopsCLI(t *testing.T, repoRoot string) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "chainops")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/chainops")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build chainops failed: %v\n%s", err, string(out))
	}
	return binPath
}
