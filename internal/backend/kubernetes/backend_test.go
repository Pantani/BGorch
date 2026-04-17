package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pantani/gorchestrator/internal/backend"
	"github.com/Pantani/gorchestrator/internal/chain/genericprocess"
	"github.com/Pantani/gorchestrator/internal/domain"
	"github.com/Pantani/gorchestrator/internal/spec"
)

func TestValidateTargetRejectsHostMode(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-single-ssh-systemd.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	cluster.Spec.Runtime.Backend = BackendName

	backend := New()
	diags := backend.ValidateTarget(cluster)
	if !hasError(diags) {
		t.Fatalf("expected validation errors for host workload")
	}
	if !hasDiagnosticPath(diags, "spec.nodePools[0].template.workloads[0].mode") {
		t.Fatalf("expected mode diagnostic, got: %#v", diags)
	}
}

func TestValidateTargetRejectsMissingImage(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-single-kubernetes.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	cluster.Spec.NodePools[0].Template.Workloads[0].Image = ""

	backend := New()
	diags := backend.ValidateTarget(cluster)
	if !hasError(diags) {
		t.Fatalf("expected validation error for missing image")
	}
	if !hasDiagnosticPath(diags, "spec.nodePools[0].template.workloads[0].image") {
		t.Fatalf("expected image diagnostic, got: %#v", diags)
	}
}

func TestBuildDesiredRenderGolden(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-single-kubernetes.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	plugin := genericprocess.New()
	if err := plugin.Normalize(cluster); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	pluginOut, err := plugin.Build(context.Background(), cluster)
	if err != nil {
		t.Fatalf("plugin build: %v", err)
	}

	backend := New()
	desired, err := backend.BuildDesired(context.Background(), cluster, pluginOut)
	if err != nil {
		t.Fatalf("build desired: %v", err)
	}

	assertArtifactsSorted(t, desired)
	assertServicesSorted(t, desired)

	manifest, ok := findArtifact(desired, "kubernetes/manifests.yaml")
	if !ok {
		t.Fatalf("manifest artifact not found")
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "manifest-single.golden.yaml"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if manifest != string(expected) {
		t.Fatalf("manifest mismatch\n--- got ---\n%s\n--- expected ---\n%s", manifest, string(expected))
	}

	if !strings.Contains(manifest, "chainops.io/source-volume-type: bind") {
		t.Fatalf("expected bind fallback annotation in manifest")
	}
}

func TestObserveRuntimeSuccessWithFakeRunner(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, defaultManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("apiVersion: v1\nkind: List\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	runner := &fakeRuntimeRunner{
		handlers: []runtimeRunHandler{
			{
				match: func(_ string, name string, args []string) bool {
					return name == "kubectl" && hasArg(args, "pods")
				},
				output: "validator-0 Running true 0\n",
			},
			{
				match: func(_ string, name string, args []string) bool {
					return name == "kubectl" && hasArg(args, "services")
				},
				output: "validator ClusterIP 10.96.0.10 26656\n",
			},
		},
	}

	backendImpl := NewWithRunner(runner)
	result, err := backendImpl.ObserveRuntime(context.Background(), backend.RuntimeObserveRequest{
		ClusterName: "testnet",
		OutputDir:   outputDir,
		Desired: domain.DesiredState{
			ClusterName: "testnet",
			Metadata: map[string]string{
				"kubernetes.namespace": "chainops",
				"kubernetes.file":      defaultManifestFile,
			},
		},
	})
	if err != nil {
		t.Fatalf("observe runtime: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 kubectl calls, got %d", len(runner.calls))
	}
	podsCall := runner.calls[0]
	if podsCall.dir != outputDir {
		t.Fatalf("expected command dir %s, got %s", outputDir, podsCall.dir)
	}
	if podsCall.name != "kubectl" {
		t.Fatalf("expected kubectl command, got %s", podsCall.name)
	}
	podsArgs := strings.Join(podsCall.args, " ")
	if !strings.Contains(podsArgs, "get pods") {
		t.Fatalf("expected pods query, got args: %s", podsArgs)
	}
	if !strings.Contains(podsArgs, "-n chainops") {
		t.Fatalf("expected namespace chainops, got args: %s", podsArgs)
	}
	if !strings.Contains(podsArgs, "-l chainops.io/cluster=testnet") {
		t.Fatalf("expected cluster label selector, got args: %s", podsArgs)
	}

	if !strings.Contains(result.Summary, `cluster "testnet"`) {
		t.Fatalf("expected summary to include cluster, got: %s", result.Summary)
	}
	if !strings.Contains(result.Summary, "pods=1") || !strings.Contains(result.Summary, "services=1") {
		t.Fatalf("expected pod/service counters in summary, got: %s", result.Summary)
	}
	expectedDetails := []string{
		"namespace: chainops",
		"selector: chainops.io/cluster=testnet",
		"pod: validator-0 Running true 0",
		"service: validator ClusterIP 10.96.0.10 26656",
	}
	for _, expected := range expectedDetails {
		if !containsLine(result.Details, expected) {
			t.Fatalf("expected detail line %q, got %#v", expected, result.Details)
		}
	}
}

func TestObserveRuntimeManifestMissing(t *testing.T) {
	t.Parallel()

	backendImpl := NewWithRunner(&fakeRuntimeRunner{})
	_, err := backendImpl.ObserveRuntime(context.Background(), backend.RuntimeObserveRequest{
		ClusterName: "testnet",
		OutputDir:   t.TempDir(),
		Desired: domain.DesiredState{
			ClusterName: "testnet",
		},
	})
	if err == nil {
		t.Fatalf("expected manifest missing error")
	}
	if !strings.Contains(err.Error(), "kubernetes manifest not found") {
		t.Fatalf("expected actionable missing-manifest error, got: %v", err)
	}
}

func TestObserveRuntimeKubectlCommandError(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, defaultManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("apiVersion: v1\nkind: List\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	runner := &fakeRuntimeRunner{
		handlers: []runtimeRunHandler{
			{
				match: func(_ string, name string, args []string) bool {
					return name == "kubectl" && hasArg(args, "pods")
				},
				output: "Error from server (Forbidden): pods is forbidden\n",
				err:    errors.New("exit status 1"),
			},
		},
	}

	backendImpl := NewWithRunner(runner)
	_, err := backendImpl.ObserveRuntime(context.Background(), backend.RuntimeObserveRequest{
		ClusterName: "testnet",
		OutputDir:   outputDir,
		Desired: domain.DesiredState{
			ClusterName: "testnet",
			Metadata: map[string]string{
				"kubernetes.file": defaultManifestFile,
			},
		},
	})
	if err == nil {
		t.Fatalf("expected kubectl command error")
	}
	if !strings.Contains(err.Error(), "runtime observe pods failed") {
		t.Fatalf("expected runtime observe pods error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "kubectl get pods") {
		t.Fatalf("expected failed command metadata in error, got: %v", err)
	}
}

func findArtifact(desired domain.DesiredState, path string) (string, bool) {
	for _, a := range desired.Artifacts {
		if a.Path == path {
			return a.Content, true
		}
	}
	return "", false
}

func hasError(diags []domain.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == domain.SeverityError {
			return true
		}
	}
	return false
}

func hasDiagnosticPath(diags []domain.Diagnostic, path string) bool {
	for _, d := range diags {
		if d.Path == path {
			return true
		}
	}
	return false
}

func assertArtifactsSorted(t *testing.T, desired domain.DesiredState) {
	t.Helper()
	for i := 1; i < len(desired.Artifacts); i++ {
		if desired.Artifacts[i-1].Path > desired.Artifacts[i].Path {
			t.Fatalf("artifacts are not sorted: %s before %s", desired.Artifacts[i-1].Path, desired.Artifacts[i].Path)
		}
	}
}

func assertServicesSorted(t *testing.T, desired domain.DesiredState) {
	t.Helper()
	for i := 1; i < len(desired.Services); i++ {
		if desired.Services[i-1].Name > desired.Services[i].Name {
			t.Fatalf("services are not sorted: %s before %s", desired.Services[i-1].Name, desired.Services[i].Name)
		}
	}
}

type runtimeRunHandler struct {
	match  func(dir string, name string, args []string) bool
	output string
	err    error
}

type runtimeRunCall struct {
	dir  string
	name string
	args []string
}

type fakeRuntimeRunner struct {
	handlers []runtimeRunHandler
	calls    []runtimeRunCall
}

func (r *fakeRuntimeRunner) Run(_ context.Context, dir, name string, args ...string) (string, error) {
	cp := append([]string{}, args...)
	r.calls = append(r.calls, runtimeRunCall{dir: dir, name: name, args: cp})
	for _, h := range r.handlers {
		if h.match != nil && !h.match(dir, name, args) {
			continue
		}
		return h.output, h.err
	}
	return "", fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
}

func hasArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func containsLine(lines []string, target string) bool {
	for _, line := range lines {
		if line == target {
			return true
		}
	}
	return false
}
