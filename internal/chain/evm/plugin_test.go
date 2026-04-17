package evm

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Pantani/gorchestrator/internal/domain"
	"github.com/Pantani/gorchestrator/internal/spec"
)

func TestBuildFromExample(t *testing.T) {
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "evm-geth-rpc.yaml"))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	plugin := New()
	diags := plugin.Validate(cluster)
	if hasError(diags) {
		t.Fatalf("unexpected validation errors: %#v", diags)
	}
	if err := plugin.Normalize(cluster); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	outFirst, err := plugin.Build(context.Background(), cluster)
	if err != nil {
		t.Fatalf("build first pass: %v", err)
	}
	outSecond, err := plugin.Build(context.Background(), cluster)
	if err != nil {
		t.Fatalf("build second pass: %v", err)
	}
	if !reflect.DeepEqual(outFirst.Artifacts, outSecond.Artifacts) {
		t.Fatalf("build output is not deterministic")
	}

	env, ok := findArtifact(outFirst.Artifacts, "nodes/rpc/config/evm.env")
	if !ok {
		t.Fatalf("evm env artifact not found")
	}
	if !strings.Contains(env, "EVM_CLIENT=geth") {
		t.Fatalf("expected geth client in env:\n%s", env)
	}
	if !strings.Contains(env, "EVM_NETWORK=sepolia") {
		t.Fatalf("expected sepolia network in env:\n%s", env)
	}
}

func findArtifact(artifacts []domain.Artifact, path string) (string, bool) {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return artifact.Content, true
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
