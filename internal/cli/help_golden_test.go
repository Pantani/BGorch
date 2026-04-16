package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelpGolden(t *testing.T) {
	t.Setenv("CHAINOPS_NON_INTERACTIVE", "true")

	cmd := NewRootCommand("chainops")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}

	got := normalizeHelp(out.String())
	repoRoot := findRepoRoot(t)
	goldenPath := filepath.Join(repoRoot, "test", "golden", "chainops-help.golden.txt")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}

	wantRaw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := normalizeHelp(string(wantRaw))
	if got != want {
		t.Fatalf("help output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func normalizeHelp(in string) string {
	in = strings.ReplaceAll(in, "\r\n", "\n")
	return strings.TrimSpace(in) + "\n"
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root from %q", cwd)
		}
		dir = parent
	}
}
