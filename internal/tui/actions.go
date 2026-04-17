package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"

	"github.com/Pantani/gorchestrator/internal/app"
	"github.com/Pantani/gorchestrator/internal/doctor"
	"github.com/Pantani/gorchestrator/internal/domain"
)

type actionID string

const (
	actionValidate actionID = "validate"
	actionRender   actionID = "render"
	actionPlan     actionID = "plan"
	actionApply    actionID = "apply"
	actionStatus   actionID = "status"
	actionDoctor   actionID = "doctor"
)

type actionItem struct {
	id          actionID
	title       string
	description string
}

func (a actionItem) FilterValue() string {
	return strings.Join([]string{a.title, a.description, string(a.id)}, " ")
}

func (a actionItem) Title() string {
	return a.title
}

func (a actionItem) Description() string {
	return a.description
}

func allActions() []list.Item {
	return []list.Item{
		actionItem{id: actionValidate, title: "Validate", description: "Lint and semantic checks for cluster spec"},
		actionItem{id: actionRender, title: "Render", description: "Generate desired artifacts to output directory"},
		actionItem{id: actionPlan, title: "Plan", description: "Diff desired state vs local snapshot"},
		actionItem{id: actionApply, title: "Apply", description: "Render + optional runtime execution + snapshot update"},
		actionItem{id: actionStatus, title: "Status", description: "Show desired vs snapshot and optional runtime state"},
		actionItem{id: actionDoctor, title: "Doctor", description: "Operational checks with pass/warn/fail report"},
	}
}

func actionUsesOutputDir(action actionID) bool {
	switch action {
	case actionRender, actionApply, actionStatus, actionDoctor:
		return true
	default:
		return false
	}
}

type optionField int

const (
	optionWriteState optionField = iota
	optionDryRun
	optionRuntimeExec
	optionObserveRuntime
)

type optionDescriptor struct {
	field optionField
	label string
	hint  string
}

func actionOptions(action actionID) []optionDescriptor {
	switch action {
	case actionRender:
		return []optionDescriptor{
			{field: optionWriteState, label: "Write state snapshot", hint: "Persist desired state after render"},
		}
	case actionApply:
		return []optionDescriptor{
			{field: optionDryRun, label: "Dry run", hint: "Compute plan only, no writes"},
			{field: optionRuntimeExec, label: "Runtime exec", hint: "Execute backend runtime after render"},
		}
	case actionStatus, actionDoctor:
		return []optionDescriptor{
			{field: optionObserveRuntime, label: "Observe runtime", hint: "Fetch runtime observations when backend supports it"},
		}
	default:
		return nil
	}
}

type runConfig struct {
	SpecPath       string
	OutputDir      string
	WriteState     bool
	DryRun         bool
	RuntimeExec    bool
	ObserveRuntime bool
}

func (c runConfig) validateFor(action actionID) error {
	if strings.TrimSpace(c.SpecPath) == "" {
		return fmt.Errorf("spec file is required")
	}
	if actionUsesOutputDir(action) && strings.TrimSpace(c.OutputDir) == "" {
		return fmt.Errorf("output directory is required for %s", action)
	}
	if action == actionApply && c.DryRun && c.RuntimeExec {
		return fmt.Errorf("dry-run cannot be combined with runtime exec")
	}
	return nil
}

type outcomeLevel int

const (
	outcomeSuccess outcomeLevel = iota
	outcomeWarning
	outcomeFailure
)

type resultSection struct {
	Title string
	Lines []string
}

type actionResult struct {
	Action       actionID
	Duration     time.Duration
	Outcome      outcomeLevel
	Summary      string
	Err          error
	Sections     []resultSection
	Diagnostics  []domain.Diagnostic
	PlanChanges  []domain.PlanChange
	DoctorChecks []doctor.Check
}

type runner struct {
	application *app.App
}

func newRunner(application *app.App) runner {
	return runner{application: application}
}

func (r runner) run(ctx context.Context, action actionID, cfg runConfig) actionResult {
	startedAt := time.Now()
	result := actionResult{Action: action}
	defer func() {
		result.Duration = time.Since(startedAt)
	}()

	if err := cfg.validateFor(action); err != nil {
		result.Outcome = outcomeFailure
		result.Err = err
		result.Summary = "Invalid command input"
		return result
	}

	specPath := strings.TrimSpace(cfg.SpecPath)
	outputDir := strings.TrimSpace(cfg.OutputDir)

	switch action {
	case actionValidate:
		cluster, diags, err := r.application.ValidateSpec(specPath)
		result.Diagnostics = diags
		if err != nil {
			result.Outcome = outcomeFailure
			result.Err = err
			result.Summary = "Validate failed"
			return result
		}

		warnings, errors := diagnosticCounts(diags)
		result.Sections = []resultSection{
			{Title: "Input", Lines: []string{fmt.Sprintf("Spec: %s", specPath)}},
			{Title: "Validation", Lines: []string{
				fmt.Sprintf("Cluster: %s", cluster.Metadata.Name),
				fmt.Sprintf("Warnings: %d", warnings),
				fmt.Sprintf("Errors: %d", errors),
			}},
		}
		if errors > 0 {
			result.Outcome = outcomeFailure
			result.Summary = fmt.Sprintf("Validation failed with %d error(s)", errors)
		} else if warnings > 0 {
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Validation passed with %d warning(s)", warnings)
		} else {
			result.Outcome = outcomeSuccess
			result.Summary = "Validation passed"
		}
		return result

	case actionRender:
		desired, diags, err := r.application.Render(ctx, specPath, outputDir, cfg.WriteState)
		result.Diagnostics = diags
		if err != nil {
			result.Outcome = outcomeFailure
			result.Err = err
			result.Summary = "Render failed"
			return result
		}

		warnings, errors := diagnosticCounts(diags)
		artifactPaths := make([]string, 0, len(desired.Artifacts))
		for _, artifact := range desired.Artifacts {
			artifactPaths = append(artifactPaths, filepath.Join(outputDir, artifact.Path))
		}
		sort.Strings(artifactPaths)
		if len(artifactPaths) == 0 {
			artifactPaths = []string{"No artifacts generated"}
		}

		result.Sections = []resultSection{
			{Title: "Input", Lines: []string{fmt.Sprintf("Spec: %s", specPath)}},
			{Title: "Output", Lines: []string{
				fmt.Sprintf("Directory: %s", outputDir),
				fmt.Sprintf("Artifacts: %d", len(desired.Artifacts)),
				fmt.Sprintf("Write state: %t", cfg.WriteState),
			}},
			{Title: "Artifacts", Lines: artifactPaths},
		}

		switch {
		case errors > 0:
			result.Outcome = outcomeFailure
			result.Summary = fmt.Sprintf("Render blocked by %d validation error(s)", errors)
		case warnings > 0:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Rendered with %d warning(s)", warnings)
		default:
			result.Outcome = outcomeSuccess
			result.Summary = fmt.Sprintf("Rendered %d artifact(s)", len(desired.Artifacts))
		}
		return result

	case actionPlan:
		planResult, diags, err := r.application.Plan(ctx, specPath)
		result.Diagnostics = diags
		if err != nil {
			result.Outcome = outcomeFailure
			result.Err = err
			result.Summary = "Plan failed"
			return result
		}

		warnings, errors := diagnosticCounts(diags)
		result.PlanChanges = nonNoopPlanChanges(planResult)
		if len(result.PlanChanges) == 0 {
			result.Sections = []resultSection{
				{Title: "Plan", Lines: []string{"No changes detected between desired state and snapshot."}},
			}
		} else {
			result.Sections = []resultSection{
				{Title: "Plan", Lines: []string{fmt.Sprintf("Changes: %d", len(result.PlanChanges))}},
			}
		}

		switch {
		case errors > 0:
			result.Outcome = outcomeFailure
			result.Summary = fmt.Sprintf("Plan blocked by %d validation error(s)", errors)
		case warnings > 0:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Plan generated with %d warning(s)", warnings)
		case len(result.PlanChanges) == 0:
			result.Outcome = outcomeSuccess
			result.Summary = "Plan clean: no changes"
		default:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Plan has %d change(s)", len(result.PlanChanges))
		}
		return result

	case actionApply:
		applyResult, diags, err := r.application.Apply(ctx, specPath, app.ApplyOptions{
			OutputDir:      outputDir,
			DryRun:         cfg.DryRun,
			ExecuteRuntime: cfg.RuntimeExec,
		})
		result.Diagnostics = diags
		if err != nil {
			result.Outcome = outcomeFailure
			result.Err = err
			result.Summary = "Apply failed"
			return result
		}

		warnings, errors := diagnosticCounts(diags)
		result.PlanChanges = nonNoopPlanChanges(applyResult.Plan)
		lines := []string{
			fmt.Sprintf("Cluster: %s", applyResult.ClusterName),
			fmt.Sprintf("Backend: %s", applyResult.Backend),
			fmt.Sprintf("Dry run: %t", applyResult.DryRun),
			fmt.Sprintf("Runtime exec: %t", applyResult.RuntimeRequested),
			fmt.Sprintf("Artifacts written: %d", applyResult.ArtifactsWritten),
			fmt.Sprintf("Snapshot updated: %t", applyResult.SnapshotUpdated),
		}
		if applyResult.LockPath != "" {
			lines = append(lines, fmt.Sprintf("Lock: %s", applyResult.LockPath))
		}
		if applyResult.RuntimeResult != nil {
			lines = append(lines, fmt.Sprintf("Runtime command: %s", applyResult.RuntimeResult.Command))
			if strings.TrimSpace(applyResult.RuntimeResult.Output) != "" {
				lines = append(lines, "Runtime output:")
				lines = append(lines, strings.Split(strings.TrimSpace(applyResult.RuntimeResult.Output), "\n")...)
			}
		}
		result.Sections = []resultSection{
			{Title: "Apply", Lines: lines},
			{Title: "Plan", Lines: []string{fmt.Sprintf("Changes: %d", len(result.PlanChanges))}},
		}

		switch {
		case errors > 0:
			result.Outcome = outcomeFailure
			result.Summary = fmt.Sprintf("Apply blocked by %d validation error(s)", errors)
		case warnings > 0:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Apply completed with %d warning(s)", warnings)
		case applyResult.DryRun:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Dry-run complete with %d planned change(s)", len(result.PlanChanges))
		default:
			result.Outcome = outcomeSuccess
			result.Summary = fmt.Sprintf("Apply completed, %d artifact(s) written", applyResult.ArtifactsWritten)
		}
		return result

	case actionStatus:
		statusResult, diags, err := r.application.Status(ctx, specPath, app.StatusOptions{
			OutputDir:      outputDir,
			ObserveRuntime: cfg.ObserveRuntime,
		})
		result.Diagnostics = diags
		if err != nil {
			result.Outcome = outcomeFailure
			result.Err = err
			result.Summary = "Status failed"
			return result
		}

		warnings, errors := diagnosticCounts(diags)
		result.PlanChanges = nonNoopPlanChanges(statusResult.Plan)
		stateSummary := "not found"
		if statusResult.SnapshotExists && statusResult.Snapshot != nil {
			stateSummary = fmt.Sprintf("present (%s)", statusResult.Snapshot.UpdatedAt.Format(time.RFC3339))
		}
		statusLines := []string{
			fmt.Sprintf("Cluster: %s", statusResult.ClusterName),
			fmt.Sprintf("Backend: %s", statusResult.Backend),
			fmt.Sprintf("Snapshot path: %s", statusResult.SnapshotPath),
			fmt.Sprintf("Snapshot: %s", stateSummary),
			fmt.Sprintf("Desired services: %d", statusResult.DesiredServices),
			fmt.Sprintf("Desired artifacts: %d", statusResult.DesiredArtifacts),
			fmt.Sprintf("Plan changes: %d", len(result.PlanChanges)),
		}
		if statusResult.RuntimeObservation != nil {
			statusLines = append(statusLines, fmt.Sprintf("Runtime: %s", statusResult.RuntimeObservation.Summary))
		}
		if strings.TrimSpace(statusResult.RuntimeObservationError) != "" {
			statusLines = append(statusLines, fmt.Sprintf("Runtime observe error: %s", statusResult.RuntimeObservationError))
		}
		if len(statusResult.Observations) > 0 {
			statusLines = append(statusLines, "Observations:")
			for _, observation := range statusResult.Observations {
				statusLines = append(statusLines, "- "+observation)
			}
		}
		result.Sections = []resultSection{{Title: "Status", Lines: statusLines}}

		switch {
		case errors > 0:
			result.Outcome = outcomeFailure
			result.Summary = fmt.Sprintf("Status has %d validation error(s)", errors)
		case strings.TrimSpace(statusResult.RuntimeObservationError) != "":
			result.Outcome = outcomeWarning
			result.Summary = "Status collected with runtime observation warning"
		case warnings > 0:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Status collected with %d warning(s)", warnings)
		default:
			result.Outcome = outcomeSuccess
			result.Summary = "Status collected"
		}
		return result

	case actionDoctor:
		report, err := r.application.Doctor(ctx, specPath, app.DoctorOptions{
			OutputDir:      outputDir,
			ObserveRuntime: cfg.ObserveRuntime,
		})
		if err != nil {
			result.Outcome = outcomeFailure
			result.Err = err
			result.Summary = "Doctor failed"
			return result
		}

		result.DoctorChecks = append(result.DoctorChecks, report.Checks...)
		passes, warns, fails := doctorCounts(report.Checks)
		lines := []string{
			fmt.Sprintf("Cluster: %s", fallback(report.ClusterName, "-")),
			fmt.Sprintf("Backend: %s", fallback(report.Backend, "-")),
			fmt.Sprintf("Pass: %d", passes),
			fmt.Sprintf("Warn: %d", warns),
			fmt.Sprintf("Fail: %d", fails),
		}
		result.Sections = []resultSection{{Title: "Doctor", Lines: lines}}
		switch {
		case fails > 0:
			result.Outcome = outcomeFailure
			result.Summary = fmt.Sprintf("Doctor found %d failing check(s)", fails)
		case warns > 0:
			result.Outcome = outcomeWarning
			result.Summary = fmt.Sprintf("Doctor completed with %d warning(s)", warns)
		default:
			result.Outcome = outcomeSuccess
			result.Summary = "Doctor checks passed"
		}
		return result

	default:
		result.Outcome = outcomeFailure
		result.Err = fmt.Errorf("unsupported action %q", action)
		result.Summary = "Unsupported command"
		return result
	}
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func diagnosticCounts(diags []domain.Diagnostic) (warnings int, errors int) {
	for _, diagnostic := range diags {
		switch diagnostic.Severity {
		case domain.SeverityWarning:
			warnings++
		case domain.SeverityError:
			errors++
		}
	}
	return warnings, errors
}

func nonNoopPlanChanges(plan domain.Plan) []domain.PlanChange {
	changes := make([]domain.PlanChange, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		if change.Type != domain.ChangeNoop {
			changes = append(changes, change)
		}
	}
	return changes
}

func doctorCounts(checks []doctor.Check) (passes int, warns int, fails int) {
	for _, check := range checks {
		switch check.Status {
		case doctor.StatusPass:
			passes++
		case doctor.StatusWarn:
			warns++
		case doctor.StatusFail:
			fails++
		}
	}
	return passes, warns, fails
}
