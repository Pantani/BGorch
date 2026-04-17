package bitcoin

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
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "bitcoin-core-node.yaml"))
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

	conf, ok := findArtifact(outFirst.Artifacts, "nodes/fullnode/config/bitcoin.conf")
	if !ok {
		t.Fatalf("bitcoin.conf artifact not found")
	}
	if !strings.Contains(conf, "signet=1") {
		t.Fatalf("expected signet network in config:\n%s", conf)
	}
	if !strings.Contains(conf, "rpcport=38332") {
		t.Fatalf("expected rpc port in config:\n%s", conf)
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
