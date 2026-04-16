package ansible

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

func TestValidateTargetWarnsContainerWorkload(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-single-compose.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	cluster.Spec.Runtime.Backend = BackendName

	diags := New().ValidateTarget(cluster)
	if !hasWarning(diags, "spec.nodePools[0].template.workloads[0].mode") {
		t.Fatalf("expected container workload warning, got: %#v", diags)
	}
}

func TestBuildDesiredDeterministic(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-bootstrap-ansible.yaml"))
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
		t.Fatalf("ansible desired state is not deterministic")
	}
	if first.Backend != BackendName {
		t.Fatalf("unexpected backend in desired state: %q", first.Backend)
	}
	if len(first.Services) == 0 {
		t.Fatalf("expected ansible backend to emit service intents")
	}
	assertArtifactsSorted(t, first.Artifacts)

	inventory := mustHaveArtifact(t, first.Artifacts, "ansible/inventory.ini")
	if !strings.Contains(inventory, "ansible_user=ubuntu") || !strings.Contains(inventory, "ansible_port=2222") {
		t.Fatalf("expected SSH metadata in inventory, got:\n%s", inventory)
	}
	if !strings.Contains(inventory, "ansible_host=validator-01.local") {
		t.Fatalf("expected runtime target host in inventory, got:\n%s", inventory)
	}

	playbook := mustHaveArtifact(t, first.Artifacts, "ansible/playbook.bootstrap.yml")
	if !strings.Contains(playbook, "Chainops host bootstrap") {
		t.Fatalf("unexpected playbook content:\n%s", playbook)
	}

	groupVars := mustHaveArtifact(t, first.Artifacts, "ansible/group_vars/all.yml")
	if !strings.Contains(groupVars, "chainops_cluster_name: generic-bootstrap") {
		t.Fatalf("expected cluster var in group_vars, got:\n%s", groupVars)
	}
}

func TestBuildDesiredFallsBackToNodeNamesWhenTargetEmpty(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "generic-bootstrap-ansible.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	cluster.Spec.Runtime.Target = ""

	plugin := genericprocess.New()
	if err := plugin.Normalize(cluster); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	pluginOut, err := plugin.Build(context.Background(), cluster)
	if err != nil {
		t.Fatalf("plugin build: %v", err)
	}

	desired, err := New().BuildDesired(context.Background(), cluster, pluginOut)
	if err != nil {
		t.Fatalf("build desired: %v", err)
	}

	inventory := mustHaveArtifact(t, desired.Artifacts, "ansible/inventory.ini")
	if !strings.Contains(inventory, "ansible_host=validators-00") || !strings.Contains(inventory, "ansible_host=validators-01") {
		t.Fatalf("expected fallback hosts derived from node names, got:\n%s", inventory)
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
