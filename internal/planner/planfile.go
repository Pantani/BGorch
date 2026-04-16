package planner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pantani/gorchestrator/internal/domain"
	"gopkg.in/yaml.v3"
)

const (
	// PlanFileAPIVersion identifies the plan serialization format.
	PlanFileAPIVersion = "chainops.io/v1alpha1"
	// PlanFileKind identifies plan documents.
	PlanFileKind = "Plan"
)

// File is the persisted plan envelope consumed by `chainops apply <plan-file>`.
type File struct {
	APIVersion string      `json:"apiVersion" yaml:"apiVersion"`
	Kind       string      `json:"kind" yaml:"kind"`
	SourceSpec string      `json:"sourceSpec" yaml:"sourceSpec"`
	Cluster    string      `json:"cluster" yaml:"cluster"`
	Backend    string      `json:"backend" yaml:"backend"`
	Generated  time.Time   `json:"generated" yaml:"generated"`
	Plan       domain.Plan `json:"plan" yaml:"plan"`
}

// NewFile builds a plan envelope with an absolute source spec path.
func NewFile(sourceSpec, cluster, backend string, plan domain.Plan) (File, error) {
	abs, err := filepath.Abs(sourceSpec)
	if err != nil {
		return File{}, fmt.Errorf("resolve source spec path: %w", err)
	}
	return File{
		APIVersion: PlanFileAPIVersion,
		Kind:       PlanFileKind,
		SourceSpec: abs,
		Cluster:    cluster,
		Backend:    backend,
		Generated:  time.Now().UTC(),
		Plan:       plan,
	}, nil
}

// WriteFile persists the plan document in JSON/YAML depending on extension.
func WriteFile(path string, planFile File) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("plan output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create plan output directory: %w", err)
	}

	encoded, err := marshalByExtension(path, planFile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write plan file %q: %w", path, err)
	}
	return nil
}

// ReadFile loads a persisted plan document.
func ReadFile(path string) (File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read plan file %q: %w", path, err)
	}

	var out File
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(raw, &out); err != nil {
			return File{}, fmt.Errorf("decode yaml plan file: %w", err)
		}
	default:
		if err := json.Unmarshal(raw, &out); err != nil {
			return File{}, fmt.Errorf("decode json plan file: %w", err)
		}
	}

	if strings.TrimSpace(out.APIVersion) == "" {
		out.APIVersion = PlanFileAPIVersion
	}
	if strings.TrimSpace(out.Kind) == "" {
		out.Kind = PlanFileKind
	}

	if out.Kind != PlanFileKind {
		return File{}, fmt.Errorf("invalid plan kind %q (expected %q)", out.Kind, PlanFileKind)
	}
	if out.APIVersion != PlanFileAPIVersion {
		return File{}, fmt.Errorf("unsupported plan apiVersion %q (expected %q)", out.APIVersion, PlanFileAPIVersion)
	}
	if strings.TrimSpace(out.SourceSpec) == "" {
		return File{}, fmt.Errorf("invalid plan file: sourceSpec is required")
	}

	return out, nil
}

func marshalByExtension(path string, planFile File) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		encoded, err := yaml.Marshal(planFile)
		if err != nil {
			return nil, fmt.Errorf("encode yaml plan file: %w", err)
		}
		return encoded, nil
	default:
		encoded, err := json.MarshalIndent(planFile, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode json plan file: %w", err)
		}
		return encoded, nil
	}
}
