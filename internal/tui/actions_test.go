package tui

import (
	"testing"

	"github.com/Pantani/gorchestrator/internal/doctor"
	"github.com/Pantani/gorchestrator/internal/domain"
)

func TestRunConfigValidateFor(t *testing.T) {
	base := runConfig{SpecPath: "spec.yaml", OutputDir: ".bgorch/render"}

	t.Run("missing spec", func(t *testing.T) {
		cfg := base
		cfg.SpecPath = ""
		if err := cfg.validateFor(actionValidate); err == nil {
			t.Fatalf("expected missing spec validation error")
		}
	})

	t.Run("missing output dir for render", func(t *testing.T) {
		cfg := base
		cfg.OutputDir = ""
		if err := cfg.validateFor(actionRender); err == nil {
			t.Fatalf("expected missing output dir validation error")
		}
	})

	t.Run("invalid apply flags", func(t *testing.T) {
		cfg := base
		cfg.DryRun = true
		cfg.RuntimeExec = true
		if err := cfg.validateFor(actionApply); err == nil {
			t.Fatalf("expected dry-run/runtime-exec validation error")
		}
	})

	t.Run("valid apply config", func(t *testing.T) {
		cfg := base
		cfg.DryRun = true
		if err := cfg.validateFor(actionApply); err != nil {
			t.Fatalf("expected valid config, got err: %v", err)
		}
	})
}

func TestNonNoopPlanChanges(t *testing.T) {
	plan := domain.Plan{Changes: []domain.PlanChange{
		{Type: domain.ChangeNoop, ResourceType: "service", Name: "noop"},
		{Type: domain.ChangeCreate, ResourceType: "service", Name: "create"},
		{Type: domain.ChangeUpdate, ResourceType: "service", Name: "update"},
	}}

	changes := nonNoopPlanChanges(plan)
	if len(changes) != 2 {
		t.Fatalf("expected 2 non-noop changes, got %d", len(changes))
	}
	if changes[0].Name != "create" || changes[1].Name != "update" {
		t.Fatalf("unexpected changes order/content: %#v", changes)
	}
}

func TestDoctorCounts(t *testing.T) {
	passes, warns, fails := doctorCounts([]doctor.Check{
		{Status: doctor.StatusPass},
		{Status: doctor.StatusWarn},
		{Status: doctor.StatusWarn},
		{Status: doctor.StatusFail},
	})

	if passes != 1 || warns != 2 || fails != 1 {
		t.Fatalf("unexpected counts: pass=%d warn=%d fail=%d", passes, warns, fails)
	}
}
