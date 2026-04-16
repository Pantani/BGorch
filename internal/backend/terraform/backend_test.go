package terraform

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Pantani/gorchestrator/internal/api/v1alpha1"
	"github.com/Pantani/gorchestrator/internal/chain/genericprocess"
	"github.com/Pantani/gorchestrator/internal/domain"
	"github.com/Pantani/gorchestrator/internal/spec"
)

func TestName(t *testing.T) {
	t.Parallel()
	if got := New().Name(); got != BackendName {
		t.Fatalf("unexpected backend name: %q", got)
	}
}

func TestValidateTargetRejectsBackendMismatch(t *testing.T) {
	t.Parallel()

	cluster := &v1alpha1.ChainCluster{}
	cluster.Spec.Runtime.Backend = "docker-compose"

	diags := New().ValidateTarget(cluster)
	if !hasError(diags, "spec.runtime.backend") {
		t.Fatalf("expected backend mismatch error, got: %#v", diags)
	}
}

func TestValidateTargetWarnsIgnoredConfig(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-infra-terraform.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	cluster.Spec.Runtime.BackendConfig.Compose = &v1alpha1.ComposeConfig{ProjectName: "ignored"}
	cluster.Spec.Runtime.BackendConfig.SSHSystemd = &v1alpha1.SSHSystemdConfig{User: "ubuntu", Port: 22}

	diags := New().ValidateTarget(cluster)
	if !hasWarning(diags, "spec.runtime.backendConfig.compose") {
		t.Fatalf("expected compose warning, got: %#v", diags)
	}
	if !hasWarning(diags, "spec.runtime.backendConfig.sshSystemd") {
		t.Fatalf("expected ssh warning, got: %#v", diags)
	}
}

func TestBuildDesiredDeterministic(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-infra-terraform.yaml"))
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

	backendImpl := New()
	first, err := backendImpl.BuildDesired(context.Background(), cluster, pluginOut)
	if err != nil {
		t.Fatalf("build desired first: %v", err)
	}
	second, err := backendImpl.BuildDesired(context.Background(), cluster, pluginOut)
	if err != nil {
		t.Fatalf("build desired second: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("terraform desired state is not deterministic")
	}
	if first.Backend != BackendName {
		t.Fatalf("unexpected backend in desired state: %q", first.Backend)
	}
	if len(first.Services) != 0 {
		t.Fatalf("terraform backend should not emit services, got %d", len(first.Services))
	}

	assertArtifactsSorted(t, first.Artifacts)

	mustHaveArtifact(t, first.Artifacts, "terraform/main.tf")
	mustHaveArtifact(t, first.Artifacts, "terraform/variables.tf")
	mustHaveArtifact(t, first.Artifacts, "terraform/outputs.tf")
	tfvars := mustHaveArtifact(t, first.Artifacts, "terraform/terraform.tfvars.json")
	if !strings.Contains(tfvars, `"runtime_target": "env/dev"`) {
		t.Fatalf("expected runtime_target in tfvars, got:\n%s", tfvars)
	}
	if !strings.Contains(tfvars, `"node_pools"`) {
		t.Fatalf("expected node_pools in tfvars, got:\n%s", tfvars)
	}
}

func assertArtifactsSorted(t *testing.T, artifacts []domain.Artifact) {
	t.Helper()
	if !sort.SliceIsSorted(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path }) {
		t.Fatalf("artifacts are not sorted by path")
	}
}

func mustHaveArtifact(t *testing.T, artifacts []domain.Artifact, path string) string {
	t.Helper()
	for _, a := range artifacts {
		if a.Path == path {
			return a.Content
		}
	}
	t.Fatalf("artifact %q not found", path)
	return ""
}

func hasError(diags []domain.Diagnostic, path string) bool {
	for _, d := range diags {
		if d.Severity == domain.SeverityError && d.Path == path {
			return true
		}
	}
	return false
}

func hasWarning(diags []domain.Diagnostic, path string) bool {
	for _, d := range diags {
		if d.Severity == domain.SeverityWarning && d.Path == path {
			return true
		}
	}
	return false
}
