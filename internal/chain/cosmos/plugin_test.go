package cosmos

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
	cluster, err := spec.LoadFromFile(filepath.Join("..", "..", "..", "examples", "cosmos-sdk-validator.yaml"))
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

	env, ok := findArtifact(outFirst.Artifacts, "nodes/validator/config/cosmos.env")
	if !ok {
		t.Fatalf("cosmos env artifact not found")
	}
	if !strings.Contains(env, "COSMOS_CHAIN_ID=cosmos-localnet") {
		t.Fatalf("expected chain id in env:\n%s", env)
	}

	appToml, ok := findArtifact(outFirst.Artifacts, "nodes/validator/config/app.toml")
	if !ok {
		t.Fatalf("app.toml artifact not found")
	}
	if !strings.Contains(appToml, "minimum-gas-prices = \"0stake\"") {
		t.Fatalf("expected minimum gas prices in app.toml:\n%s", appToml)
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
