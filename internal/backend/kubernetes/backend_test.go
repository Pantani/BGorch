package kubernetes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
