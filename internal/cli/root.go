package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pantani/gorchestrator/internal/app"
	"github.com/Pantani/gorchestrator/internal/config"
	"github.com/Pantani/gorchestrator/internal/domain"
	"github.com/Pantani/gorchestrator/internal/engine"
	"github.com/Pantani/gorchestrator/internal/output"
	"github.com/Pantani/gorchestrator/internal/planner"
	"github.com/Pantani/gorchestrator/internal/schema"
	"github.com/Pantani/gorchestrator/internal/tui"
	"github.com/Pantani/gorchestrator/internal/workspace"
)

var version = "dev"

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func usageError(err error) error {
	return &exitError{code: 2, err: err}
}

func runtimeError(err error) error {
	return &exitError{code: 1, err: err}
}

func wrapRuntimeUnsupportedError(program, command, specPath string, err error) error {
	var unsupportedErr *app.RuntimeUnsupportedError
	if !errors.As(err, &unsupportedErr) {
		return err
	}

	switch unsupportedErr.Capability {
	case app.RuntimeCapabilityExecution:
		return output.ActionableError(
			"Runtime execution is unavailable for this backend.",
			err.Error(),
			"Use a backend with runtime execution support (docker-compose or ssh-systemd), or rerun without --runtime-exec/--require-runtime.",
			fmt.Sprintf("%s apply -f %s --yes", program, specPath),
		)
	case app.RuntimeCapabilityObservation:
		return output.ActionableError(
			"Runtime observation is unavailable for this backend.",
			err.Error(),
			"Use a backend with runtime observation support (docker-compose or ssh-systemd), or rerun without --require-runtime.",
			fmt.Sprintf("%s %s -f %s", program, command, specPath),
		)
	default:
		return err
	}
}

type runtimeContext struct {
	program          string
	legacy           bool
	cfgResolver      *config.Config
	cfg              config.Values
	loadedConfigFile string
	engine           *engine.Service
	out              io.Writer
	err              io.Writer
}

// Run executes the chainops command tree and returns a process exit code.
func Run(args []string) int {
	return RunProgram("chainops", args)
}

// RunProgram executes the CLI for a given binary name (chainops or bgorch).
func RunProgram(program string, args []string) int {
	cmd := NewRootCommand(program)
	cmd.SetArgs(args)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		var e *exitError
		if errors.As(err, &e) {
			if e.err != nil {
				_, _ = fmt.Fprintln(os.Stderr, e.err.Error())
			}
			return e.code
		}
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err.Error())
		}
		if looksLikeUsageError(err) {
			return 2
		}
		return 1
	}
	return 0
}

// NewRootCommand builds the Cobra command tree for chainops.
func NewRootCommand(program string) *cobra.Command {
	legacy := strings.EqualFold(strings.TrimSpace(program), "bgorch")
	ctx := &runtimeContext{
		program:     program,
		legacy:      legacy,
		cfgResolver: config.New(legacy),
		out:         os.Stdout,
		err:         os.Stderr,
	}
	defaults := config.DefaultValues(legacy)

	short := "Declarative multi-blockchain orchestrator"
	if legacy {
		short = "BGorch (legacy alias)"
	}

	root := &cobra.Command{
		Use:   program,
		Short: short,
		Long: `Chainops is a Go-first declarative orchestrator for blockchain operations.

Mental model:
- You define desired state in a ChainCluster spec.
- chainops resolves defaults, validates schema/plugins/backends, and computes a deterministic plan.
- apply reconciles desired state against current snapshot/runtime with explicit safety gates.

One obvious path for new users:
  chainops init
  chainops doctor
  chainops render -f chainops.yaml -o yaml
  chainops plan -f chainops.yaml
  chainops apply -f chainops.yaml --yes`,
		Example: `  # Starter flow
  chainops init --profile local-dev --name demo
  chainops doctor -f chainops.yaml
  chainops render -f chainops.yaml -o yaml
  chainops plan -f chainops.yaml --out plan.json
  chainops apply plan.json --yes

  # Discoverability
  chainops explain ChainCluster.spec.runtime
  chainops explain plugin generic-process
  chainops profile list
  chainops completion zsh`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return ctx.preRun(cmd)
		},
	}

	root.PersistentFlags().String("config", defaults.ConfigFile, "Path to CLI config file.")
	root.PersistentFlags().StringP("file", "f", defaults.SpecFile, "Path to ChainCluster spec file.")
	root.PersistentFlags().String("state-dir", defaults.StateDir, "State directory used for snapshots and locks.")
	root.PersistentFlags().String("artifacts-dir", defaults.ArtifactsDir, "Directory for rendered backend artifacts.")
	root.PersistentFlags().Bool("non-interactive", defaults.NonInteractive, "Disable prompts; require explicit flags for destructive actions.")
	root.PersistentFlags().Bool("yes", defaults.Yes, "Auto-confirm operations that require interactive confirmation.")

	ctx.cfgResolver.BindRootFlags(root.PersistentFlags())

	root.AddCommand(
		newInitCmd(ctx),
		newExplainCmd(ctx),
		newRenderCmd(ctx),
		newPlanCmd(ctx),
		newDiffCmd(ctx),
		newApplyCmd(ctx),
		newStatusCmd(ctx),
		newLogsCmd(ctx),
		newDoctorCmd(ctx),
		newDestroyCmd(ctx),
		newBackupCmd(ctx),
		newRestoreCmd(ctx),
		newUpgradeCmd(ctx),
		newPluginCmd(ctx),
		newProfileCmd(ctx),
		newContextCmd(ctx),
		newValidateCmd(ctx),
		newCompletionCmd(root),
		newVersionCmd(program),
	)

	if legacy {
		root.AddCommand(newTUICmd(ctx))
	}

	return root
}

func (c *runtimeContext) preRun(cmd *cobra.Command) error {
	if cmd.CommandPath() == c.program+" help" {
		return nil
	}
	resolved, configFileUsed, err := c.cfgResolver.Resolve()
	if err != nil {
		return usageError(output.ActionableError(
			"Invalid CLI configuration.",
			err.Error(),
			"Check --config path, environment variables, and flag values.",
			c.program+" context show",
		))
	}
	c.cfg = resolved
	c.loadedConfigFile = configFileUsed
	c.engine = engine.New(engine.Options{StateDir: resolved.StateDir})
	return nil
}

func (c *runtimeContext) resolveSpecPath(cmd *cobra.Command) (string, error) {
	path, err := cmd.Flags().GetString("file")
	if err != nil {
		return "", usageError(err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", usageError(output.ActionableError(
			"Missing required spec path.",
			"No spec file was provided.",
			"Use -f <file> or set CHAINOPS_FILE.",
			c.program+" init",
		))
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", usageError(output.ActionableError(
				"Spec file not found.",
				fmt.Sprintf("%s does not exist.", path),
				"Create a starter spec with `chainops init` or pass the correct -f path.",
				c.program+" init --name demo --profile local-dev",
			))
		}
		return "", runtimeError(fmt.Errorf("check spec path %q: %w", path, err))
	}
	return path, nil
}

func (c *runtimeContext) resolveArtifactsDir(cmd *cobra.Command) (string, error) {
	artifactsDir, err := cmd.Flags().GetString("artifacts-dir")
	if err != nil {
		return "", usageError(err)
	}
	legacyOutputDir, err := cmd.Flags().GetString("output-dir")
	if err == nil && strings.TrimSpace(legacyOutputDir) != "" {
		artifactsDir = legacyOutputDir
	}
	artifactsDir = strings.TrimSpace(artifactsDir)
	if artifactsDir == "" {
		artifactsDir = c.cfg.ArtifactsDir
	}
	return artifactsDir, nil
}

func (c *runtimeContext) resolveOutputFormat(cmd *cobra.Command, fallback string) (string, error) {
	if flag := cmd.Flags().Lookup("output"); flag != nil && flag.Changed {
		return output.NormalizeFormat(flag.Value.String())
	}
	if format := c.cfgResolver.OutputFromConfig(); strings.TrimSpace(format) != "" {
		return output.NormalizeFormat(format)
	}
	if strings.TrimSpace(fallback) == "" {
		fallback = "table"
	}
	return output.NormalizeFormat(fallback)
}

func (c *runtimeContext) resolveOutputWithLegacyPath(cmd *cobra.Command, fallback string) (format string, legacyArtifactsDir string, err error) {
	if flag := cmd.Flags().Lookup("output"); flag != nil && flag.Changed {
		value := strings.TrimSpace(flag.Value.String())
		if normalized, normalizeErr := output.NormalizeFormat(value); normalizeErr == nil {
			return normalized, "", nil
		}
		// Legacy compatibility: -o was output-dir in bgorch for apply/status/doctor/logs.
		if value != "" {
			return output.FormatTable, value, nil
		}
	}

	resolvedFormat, resolveErr := c.resolveOutputFormat(cmd, fallback)
	if resolveErr != nil {
		return "", "", resolveErr
	}
	return resolvedFormat, "", nil
}

func (c *runtimeContext) printDiagnostics(diags []domain.Diagnostic) {
	c.printDiagnosticsTo(c.err, diags)
}

func (c *runtimeContext) printDiagnosticsTo(target io.Writer, diags []domain.Diagnostic) {
	for _, d := range diags {
		line := fmt.Sprintf("[%s] %s", strings.ToUpper(string(d.Severity)), d.Message)
		if d.Path != "" {
			line += " (" + d.Path + ")"
		}
		_, _ = fmt.Fprintln(target, line)
		if strings.TrimSpace(d.Hint) != "" {
			_, _ = fmt.Fprintf(target, "  hint: %s\n", d.Hint)
		}
	}
}

func (c *runtimeContext) requireConfirmation(cmd *cobra.Command, action string) error {
	if c.legacy || c.cfg.Yes {
		return nil
	}
	if c.cfg.NonInteractive || !isInteractiveSession() {
		return usageError(output.ActionableError(
			"Confirmation required for non-interactive apply.",
			"Running without a TTY can apply irreversible changes silently.",
			"Re-run with --yes to explicitly acknowledge the operation.",
			fmt.Sprintf("%s %s --yes", c.program, action),
		))
	}

	_, _ = fmt.Fprintf(c.out, "This will %s. Continue? [y/N]: ", action)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return runtimeError(fmt.Errorf("operation canceled by user"))
	}
	return nil
}

func newInitCmd(c *runtimeContext) *cobra.Command {
	var clusterName string
	var profileName string
	var family string
	var plugin string
	var backend string
	var outputPath string
	var force bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a new Chainops workspace and starter spec.",
		Long: `Creates a starter chainops.yaml with safe defaults.

Modes:
- Interactive: prompts for missing values.
- Non-interactive: fully scriptable with flags/env vars.

Defaults are profile-driven to minimize cognitive load for first success.`,
		Example: `  chainops init
  chainops init --profile local-dev --name demo
  chainops init --profile vm-single --name validator-1 --non-interactive
  chainops init --profile cometbft-local --name comet-local --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			autoInteractive := isInteractiveSession() && !c.cfg.NonInteractive
			if interactive || (autoInteractive && clusterName == "" && profileName == "") {
				clusterName = promptWithDefault("Cluster name", clusterName, "chainops-local")
				profileName = promptWithDefault("Profile", profileName, "local-dev")
				family = promptWithDefault("Family", family, "")
				plugin = promptWithDefault("Plugin", plugin, "")
				backend = promptWithDefault("Backend", backend, "")
			}

			spec, err := workspace.BuildSpec(workspace.InitRequest{
				ClusterName: clusterName,
				Profile:     profileName,
				Family:      family,
				Plugin:      plugin,
				Backend:     backend,
			})
			if err != nil {
				return usageError(output.ActionableError(
					"Invalid init parameters.",
					err.Error(),
					"Use `chainops profile list` to inspect supported profiles.",
					"chainops profile list",
				))
			}

			if strings.TrimSpace(outputPath) == "" {
				outputPath = "chainops.yaml"
			}
			if _, err := os.Stat(outputPath); err == nil && !force {
				return usageError(output.ActionableError(
					"Workspace already initialized.",
					fmt.Sprintf("File %s already exists.", outputPath),
					"Use --force to overwrite or choose another output path.",
					fmt.Sprintf("%s init --force", c.program),
				))
			}

			if err := os.WriteFile(outputPath, []byte(spec), 0o644); err != nil {
				return runtimeError(fmt.Errorf("write starter spec %q: %w", outputPath, err))
			}

			_, _ = fmt.Fprintf(c.out, "Created %s\n", outputPath)
			_, _ = fmt.Fprintf(c.out, "Next:\n")
			_, _ = fmt.Fprintf(c.out, "  %s doctor -f %s\n", c.program, outputPath)
			_, _ = fmt.Fprintf(c.out, "  %s render -f %s -o yaml\n", c.program, outputPath)
			_, _ = fmt.Fprintf(c.out, "  %s plan -f %s\n", c.program, outputPath)
			_, _ = fmt.Fprintf(c.out, "  %s apply -f %s --yes\n", c.program, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&clusterName, "name", "", "Cluster name for metadata.name.")
	cmd.Flags().StringVar(&profileName, "profile", "local-dev", "Starter profile (local-dev, compose-single, vm-single, cometbft-local).")
	cmd.Flags().StringVar(&family, "family", "", "Override profile family.")
	cmd.Flags().StringVar(&plugin, "plugin", "", "Override profile plugin.")
	cmd.Flags().StringVar(&backend, "backend", "", "Override profile backend.")
	cmd.Flags().StringVarP(&outputPath, "output-path", "p", "chainops.yaml", "Output spec path.")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing output-path.")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "Prompt for missing values.")

	return cmd
}

func newExplainCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain <resource-path|plugin <name>|profile <name>>",
		Short: "Explain schema fields, plugins, and profiles.",
		Long:  "Shows structured explain output inspired by kubectl explain.",
		Example: `  chainops explain ChainCluster
  chainops explain ChainCluster.spec.runtime
  chainops explain ChainCluster.spec.storage
  chainops explain plugin generic-process
  chainops explain profile local-dev`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}

			switch strings.ToLower(args[0]) {
			case "plugin":
				if len(args) < 2 {
					return usageError(output.ActionableError(
						"Missing plugin name.",
						"explain plugin requires a plugin identifier.",
						"Run chainops plugin list to discover names.",
						"chainops plugin list",
					))
				}
				return c.explainPlugin(args[1], format)
			case "profile":
				if len(args) < 2 {
					return usageError(output.ActionableError(
						"Missing profile name.",
						"explain profile requires a profile identifier.",
						"Run chainops profile list to discover names.",
						"chainops profile list",
					))
				}
				return c.explainProfile(args[1], format)
			default:
				query := strings.Join(args, " ")
				doc, ok := schema.Lookup(query)
				if !ok {
					return usageError(output.ActionableError(
						"Explain target not found.",
						fmt.Sprintf("No schema docs found for %q.", query),
						"Try ChainCluster, ChainCluster.spec, or ChainCluster.spec.runtime.",
						"chainops explain ChainCluster.spec",
					))
				}
				return renderSchemaDoc(c.out, format, doc)
			}
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	return cmd
}

func (c *runtimeContext) explainPlugin(name, format string) error {
	plugin, ok := c.engine.Registries().Plugins.Get(name)
	if !ok {
		return usageError(output.ActionableError(
			"Plugin not found.",
			fmt.Sprintf("Plugin %q is not registered.", name),
			"Check available plugins.",
			"chainops plugin list",
		))
	}

	payload := map[string]any{
		"name":         plugin.Name(),
		"family":       plugin.Family(),
		"capabilities": plugin.Capabilities(),
	}

	if format == output.FormatTable {
		rows := [][]string{
			{"name", plugin.Name()},
			{"family", plugin.Family()},
			{"supportsMultiProcess", fmt.Sprintf("%t", plugin.Capabilities().SupportsMultiProcess)},
			{"supportsBootstrap", fmt.Sprintf("%t", plugin.Capabilities().SupportsBootstrap)},
			{"supportsBackup", fmt.Sprintf("%t", plugin.Capabilities().SupportsBackup)},
			{"supportsRestore", fmt.Sprintf("%t", plugin.Capabilities().SupportsRestore)},
			{"supportsUpgrade", fmt.Sprintf("%t", plugin.Capabilities().SupportsUpgrade)},
		}
		output.WriteTable(c.out, []string{"Field", "Value"}, rows)
		return nil
	}
	return output.Encode(c.out, format, payload)
}

func (c *runtimeContext) explainProfile(name, format string) error {
	profile, ok := workspace.GetProfile(name)
	if !ok {
		return usageError(output.ActionableError(
			"Profile not found.",
			fmt.Sprintf("Profile %q does not exist.", name),
			"List built-in profiles.",
			"chainops profile list",
		))
	}

	if format == output.FormatTable {
		rows := [][]string{
			{"name", profile.Name},
			{"summary", profile.Summary},
			{"family", profile.Family},
			{"plugin", profile.Plugin},
			{"backend", profile.Backend},
			{"intendedUsers", profile.IntendedUsers},
		}
		output.WriteTable(c.out, []string{"Field", "Value"}, rows)
		return nil
	}
	return output.Encode(c.out, format, profile)
}

func newRenderCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render canonical resolved config or backend artifacts.",
		Long: `Default mode renders canonical resolved spec (defaults + merges + normalization).

Legacy compatibility:
- If -o is not a format, render treats it as an artifacts directory.
- Use --write-artifacts for explicit artifact rendering mode.`,
		Example: `  chainops render -f chainops.yaml -o yaml
  chainops render -f chainops.yaml --format json
  chainops render -f chainops.yaml --write-artifacts --artifacts-dir .chainops/render
  chainops render -f chainops.yaml -o .chainops/render     # legacy artifact mode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}

			mode, format, artifactsDir, writeState, outFile, err := c.resolveRenderMode(cmd)
			if err != nil {
				return err
			}

			switch mode {
			case "artifacts":
				desired, diags, renderErr := c.engine.RenderArtifacts(context.Background(), specPath, artifactsDir, writeState)
				if renderErr != nil {
					return runtimeError(renderErr)
				}
				c.printDiagnostics(diags)
				if hasErrors(diags) {
					return runtimeError(fmt.Errorf("cannot render artifacts because spec validation failed"))
				}

				if format == output.FormatTable {
					_, _ = fmt.Fprintf(c.out, "rendered %d artifact(s) to %s\n", len(desired.Artifacts), artifactsDir)
					rows := [][]string{
						{"cluster", desired.ClusterName},
						{"backend", desired.Backend},
						{"artifacts", fmt.Sprintf("%d", len(desired.Artifacts))},
						{"outputDir", artifactsDir},
					}
					output.WriteTable(c.out, []string{"Field", "Value"}, rows)
					return nil
				}
				return output.Encode(c.out, format, desired)
			default:
				cluster, diags, renderErr := c.engine.Validate(specPath)
				if renderErr != nil {
					return runtimeError(renderErr)
				}
				c.printDiagnostics(diags)
				if hasErrors(diags) {
					return runtimeError(fmt.Errorf("cannot render canonical config due to validation errors"))
				}

				if strings.TrimSpace(outFile) != "" {
					if err := writeEncodedToFile(outFile, format, cluster); err != nil {
						return runtimeError(err)
					}
					_, _ = fmt.Fprintf(c.out, "Canonical config written to %s\n", outFile)
					return nil
				}

				if format == output.FormatTable {
					rows := [][]string{
						{"apiVersion", cluster.APIVersion},
						{"kind", cluster.Kind},
						{"metadata.name", cluster.Metadata.Name},
						{"spec.family", cluster.Spec.Family},
						{"spec.plugin", cluster.Spec.Plugin},
						{"spec.runtime.backend", cluster.Spec.Runtime.Backend},
						{"spec.nodePools", fmt.Sprintf("%d", len(cluster.Spec.NodePools))},
					}
					output.WriteTable(c.out, []string{"Field", "Value"}, rows)
					return nil
				}
				return output.Encode(c.out, format, cluster)
			}
		},
	}

	cmd.Flags().StringP("output", "o", "yaml", "Output format table|json|yaml. Legacy: if not a format, treated as artifacts directory.")
	cmd.Flags().String("format", "", "Canonical output format override (table|json|yaml).")
	cmd.Flags().String("out-file", "", "Write canonical render output to file.")
	cmd.Flags().Bool("write-artifacts", false, "Render backend artifacts to --artifacts-dir instead of canonical spec output.")
	cmd.Flags().String("output-dir", "", "Legacy alias for --artifacts-dir.")
	cmd.Flags().Bool("write-state", false, "Persist desired-state snapshot while rendering artifacts.")

	return cmd
}

func (c *runtimeContext) resolveRenderMode(cmd *cobra.Command) (mode string, format string, artifactsDir string, writeState bool, outFile string, err error) {
	outValue, _ := cmd.Flags().GetString("output")
	formatValue, _ := cmd.Flags().GetString("format")
	writeArtifacts, _ := cmd.Flags().GetBool("write-artifacts")
	writeState, _ = cmd.Flags().GetBool("write-state")
	outFile, _ = cmd.Flags().GetString("out-file")

	artifactsDir, err = c.resolveArtifactsDir(cmd)
	if err != nil {
		return "", "", "", false, "", err
	}

	if strings.TrimSpace(formatValue) != "" {
		format, err = output.NormalizeFormat(formatValue)
		if err != nil {
			return "", "", "", false, "", usageError(err)
		}
		if writeArtifacts {
			return "artifacts", format, artifactsDir, writeState, outFile, nil
		}
		return "canonical", format, artifactsDir, writeState, outFile, nil
	}

	if normalized, normalizeErr := output.NormalizeFormat(outValue); normalizeErr == nil {
		format = normalized
		if writeArtifacts {
			return "artifacts", format, artifactsDir, writeState, outFile, nil
		}
		return "canonical", format, artifactsDir, writeState, outFile, nil
	}

	if strings.TrimSpace(outValue) != "" {
		artifactsDir = outValue
	}
	if strings.TrimSpace(artifactsDir) == "" {
		artifactsDir = c.cfg.ArtifactsDir
	}
	return "artifacts", output.FormatTable, artifactsDir, writeState, outFile, nil
}

func newPlanCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Preview desired-vs-current changes without side effects.",
		Long: `Plan is side-effect free.

It computes desired state and compares it with the local snapshot to show creates/updates/deletes before apply.`,
		Example: `  chainops plan -f chainops.yaml
  chainops plan -f chainops.yaml --output json
  chainops plan -f chainops.yaml --out plan.json
  chainops plan -f chainops.yaml --out plan.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runPlan(cmd, false)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().String("out", "", "Persist plan document to file (json|yaml).")
	cmd.Flags().Bool("show-noop", false, "Include noop entries in table output.")
	return cmd
}

func newDiffCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show non-noop differences between desired state and snapshot.",
		Long:  "diff is a focused plan view. Use plan --show-noop for full parity details.",
		Example: `  chainops diff -f chainops.yaml
  chainops diff -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runPlan(cmd, true)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().String("out", "", "Persist plan document to file (json|yaml).")
	cmd.Flags().Bool("show-noop", false, "Include noop entries in table output.")
	return cmd
}

func (c *runtimeContext) runPlan(cmd *cobra.Command, diffOnly bool) error {
	specPath, err := c.resolveSpecPath(cmd)
	if err != nil {
		return err
	}
	format, err := c.resolveOutputFormat(cmd, "table")
	if err != nil {
		return usageError(err)
	}
	showNoop, _ := cmd.Flags().GetBool("show-noop")

	plan, diags, planErr := c.engine.Plan(context.Background(), specPath)
	if planErr != nil {
		return runtimeError(planErr)
	}
	c.printDiagnostics(diags)
	if hasErrors(diags) {
		return runtimeError(fmt.Errorf("plan failed due to validation errors"))
	}

	if format == output.FormatTable {
		changes := plan.Changes
		if diffOnly || !showNoop {
			changes = nonNoopChanges(plan.Changes)
		}
		if len(changes) == 0 {
			_, _ = fmt.Fprintln(c.out, "plan: no changes")
		} else {
			_, _ = fmt.Fprintf(c.out, "plan: %d change(s)\n", len(changes))
			rows := make([][]string, 0, len(changes))
			for _, change := range changes {
				rows = append(rows, []string{strings.ToUpper(string(change.Type)), change.ResourceType, change.Name, change.Reason})
			}
			output.WriteTable(c.out, []string{"Type", "Resource", "Name", "Reason"}, rows)
		}
		create, update, del, noop := summarizePlan(plan)
		_, _ = fmt.Fprintf(c.out, "Summary: create=%d update=%d delete=%d noop=%d\n", create, update, del, noop)
	} else {
		payload := plan
		if diffOnly {
			payload.Changes = nonNoopChanges(payload.Changes)
		}
		if err := output.Encode(c.out, format, payload); err != nil {
			return runtimeError(err)
		}
	}

	outPath, _ := cmd.Flags().GetString("out")
	outPath = strings.TrimSpace(outPath)
	if outPath != "" {
		cluster, _, validateErr := c.engine.Validate(specPath)
		if validateErr != nil {
			return runtimeError(validateErr)
		}
		planFile, pfErr := planner.NewFile(specPath, cluster.Metadata.Name, cluster.Spec.Runtime.Backend, plan)
		if pfErr != nil {
			return runtimeError(pfErr)
		}
		if writeErr := planner.WriteFile(outPath, planFile); writeErr != nil {
			return runtimeError(writeErr)
		}
		_, _ = fmt.Fprintf(c.out, "Plan file written to %s\n", outPath)
	}
	return nil
}

func newApplyCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [plan-file]",
		Short: "Apply desired state safely using spec or saved plan file.",
		Long: `Apply executes reconciliation with safety defaults.

- Interactive mode prompts for confirmation.
- Non-interactive mode requires --yes.
- apply plan.json resolves sourceSpec from the plan document.`,
		Example: `  chainops apply -f chainops.yaml --yes
  chainops apply -f chainops.yaml --dry-run
  chainops apply -f chainops.yaml --runtime-exec --yes
  chainops apply -f chainops.yaml --require-runtime --yes
  chainops apply plan.json --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			runtimeExec, _ := cmd.Flags().GetBool("runtime-exec")
			requireRuntime, _ := cmd.Flags().GetBool("require-runtime")
			executeRuntime := runtimeExec || requireRuntime
			format, legacyArtifactsDir, err := c.resolveOutputWithLegacyPath(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			if dryRun && executeRuntime {
				return usageError(output.ActionableError(
					"Invalid flag combination.",
					"--dry-run cannot be combined with --runtime-exec or --require-runtime.",
					"Remove one of the flags.",
					fmt.Sprintf("%s apply -f chainops.yaml --dry-run", c.program),
				))
			}

			specPath, planPath, err := c.resolveApplyInput(cmd, args)
			if err != nil {
				return err
			}

			plan, diags, planErr := c.engine.Plan(context.Background(), specPath)
			if planErr != nil {
				return runtimeError(planErr)
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("apply aborted due to validation errors"))
			}

			if planPath != "" {
				filePlan, readErr := planner.ReadFile(planPath)
				if readErr != nil {
					return runtimeError(readErr)
				}
				if !reflect.DeepEqual(filePlan.Plan.Changes, plan.Changes) {
					_, _ = fmt.Fprintln(c.err, "[WARN] plan drift detected: current plan differs from saved plan file.")
				}
			}

			if !dryRun {
				action := "apply desired state"
				if executeRuntime {
					action = "apply desired state and execute runtime actions"
				}
				if err := c.requireConfirmation(cmd, action); err != nil {
					return err
				}
			}

			artifactsDir, err := c.resolveArtifactsDir(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(legacyArtifactsDir) != "" {
				artifactsDir = legacyArtifactsDir
			}

			result, applyDiags, applyErr := c.engine.Apply(context.Background(), specPath, app.ApplyOptions{
				OutputDir:      artifactsDir,
				DryRun:         dryRun,
				ExecuteRuntime: runtimeExec,
				RequireRuntime: requireRuntime,
			})
			if applyErr != nil {
				return runtimeError(wrapRuntimeUnsupportedError(c.program, "apply", specPath, applyErr))
			}
			c.printDiagnostics(applyDiags)
			if hasErrors(applyDiags) {
				return runtimeError(fmt.Errorf("apply failed due to diagnostics"))
			}

			if format == output.FormatTable {
				rows := [][]string{
					{"cluster", result.ClusterName},
					{"backend", result.Backend},
					{"dryRun", fmt.Sprintf("%t", result.DryRun)},
					{"runtimeRequested", fmt.Sprintf("%t", result.RuntimeRequested)},
					{"artifactsWritten", fmt.Sprintf("%d", result.ArtifactsWritten)},
					{"snapshotUpdated", fmt.Sprintf("%t", result.SnapshotUpdated)},
				}
				if result.LockPath != "" {
					rows = append(rows, []string{"lockPath", result.LockPath})
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				if result.DryRun {
					_, _ = fmt.Fprintln(c.out, "dry-run enabled: no artifacts or state were written")
				}
				if result.SnapshotUpdated {
					_, _ = fmt.Fprintln(c.out, "state snapshot updated")
				}

				changes := nonNoopChanges(result.Plan.Changes)
				if len(changes) > 0 {
					planRows := make([][]string, 0, len(changes))
					for _, change := range changes {
						planRows = append(planRows, []string{strings.ToUpper(string(change.Type)), change.ResourceType, change.Name, change.Reason})
					}
					_, _ = fmt.Fprintln(c.out)
					output.WriteTable(c.out, []string{"Type", "Resource", "Name", "Reason"}, planRows)
				}

				if result.RuntimeResult != nil && strings.TrimSpace(result.RuntimeResult.Output) != "" {
					_, _ = fmt.Fprintln(c.out, "Runtime output:")
					_, _ = fmt.Fprintln(c.out, strings.TrimSpace(result.RuntimeResult.Output))
				}
				return nil
			}
			return output.Encode(c.out, format, result)
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().Bool("dry-run", false, "Plan + validate without writing artifacts/snapshot.")
	cmd.Flags().Bool("runtime-exec", false, "Execute backend runtime actions after artifact render.")
	cmd.Flags().Bool("require-runtime", false, "Require runtime execution support and execute runtime actions.")
	cmd.Flags().String("output-dir", "", "Legacy alias for --artifacts-dir.")

	return cmd
}

func (c *runtimeContext) resolveApplyInput(cmd *cobra.Command, args []string) (specPath string, planPath string, err error) {
	if len(args) == 1 {
		candidate := strings.TrimSpace(args[0])
		if candidate != "" {
			planFile, readErr := planner.ReadFile(candidate)
			if readErr == nil {
				return planFile.SourceSpec, candidate, nil
			}
			if strings.HasSuffix(candidate, ".json") || strings.HasSuffix(candidate, ".yaml") || strings.HasSuffix(candidate, ".yml") {
				return "", "", usageError(output.ActionableError(
					"Invalid plan file.",
					readErr.Error(),
					"Generate a new plan with `chainops plan --out`.",
					"chainops plan -f chainops.yaml --out plan.json",
				))
			}
		}
	}
	resolvedSpec, resolveErr := c.resolveSpecPath(cmd)
	if resolveErr != nil {
		return "", "", resolveErr
	}
	return resolvedSpec, "", nil
}

func newStatusCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show desired vs observed state and optional runtime observations.",
		Long:  "Status compares desired state against snapshot and can optionally inspect runtime state.",
		Example: `  chainops status -f chainops.yaml
  chainops status -f chainops.yaml --observe-runtime
  chainops status -f chainops.yaml --require-runtime
  chainops status -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			format, legacyArtifactsDir, err := c.resolveOutputWithLegacyPath(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			observeRuntime, _ := cmd.Flags().GetBool("observe-runtime")
			requireRuntime, _ := cmd.Flags().GetBool("require-runtime")
			artifactsDir, err := c.resolveArtifactsDir(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(legacyArtifactsDir) != "" {
				artifactsDir = legacyArtifactsDir
			}

			result, diags, statusErr := c.engine.Status(context.Background(), specPath, app.StatusOptions{
				OutputDir:      artifactsDir,
				ObserveRuntime: observeRuntime,
				RequireRuntime: requireRuntime,
			})
			if statusErr != nil {
				return runtimeError(wrapRuntimeUnsupportedError(c.program, "status", specPath, statusErr))
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("status failed due to diagnostics"))
			}

			if format == output.FormatTable {
				if c.legacy {
					_, _ = fmt.Fprintf(c.out, "cluster: %s\n", result.ClusterName)
					_, _ = fmt.Fprintf(c.out, "backend: %s\n", result.Backend)
					_, _ = fmt.Fprintf(c.out, "snapshot path: %s\n", result.SnapshotPath)
					if result.SnapshotExists && result.Snapshot != nil {
						_, _ = fmt.Fprintf(c.out, "snapshot: present (%s)\n", result.Snapshot.UpdatedAt.Format(time.RFC3339))
					} else {
						_, _ = fmt.Fprintln(c.out, "snapshot: not found")
					}
					_, _ = fmt.Fprintf(c.out, "desired services: %d\n", result.DesiredServices)
					_, _ = fmt.Fprintf(c.out, "desired artifacts: %d\n", result.DesiredArtifacts)
					changes := nonNoopChanges(result.Plan.Changes)
					if len(changes) == 0 {
						_, _ = fmt.Fprintln(c.out, "plan: no changes")
					} else {
						_, _ = fmt.Fprintf(c.out, "plan: %d change(s)\n", len(changes))
						for _, change := range changes {
							_, _ = fmt.Fprintf(c.out, "- %s %s %s (%s)\n", strings.ToUpper(string(change.Type)), change.ResourceType, change.Name, change.Reason)
						}
					}
					if len(result.Observations) > 0 {
						_, _ = fmt.Fprintln(c.out, "observations:")
						for _, line := range result.Observations {
							_, _ = fmt.Fprintf(c.out, "- %s\n", line)
						}
					}
					if result.RuntimeObservation != nil {
						_, _ = fmt.Fprintf(c.out, "runtime: %s\n", result.RuntimeObservation.Summary)
					}
					if result.RuntimeObservationError != "" {
						_, _ = fmt.Fprintf(c.out, "runtime observe error: %s\n", result.RuntimeObservationError)
					}
				} else {
					rows := [][]string{
						{"cluster", result.ClusterName},
						{"backend", result.Backend},
						{"snapshotPath", result.SnapshotPath},
						{"snapshotExists", fmt.Sprintf("%t", result.SnapshotExists)},
						{"desiredServices", fmt.Sprintf("%d", result.DesiredServices)},
						{"desiredArtifacts", fmt.Sprintf("%d", result.DesiredArtifacts)},
					}
					if result.SnapshotExists && result.Snapshot != nil {
						rows = append(rows, []string{"snapshotUpdatedAt", result.Snapshot.UpdatedAt.Format(time.RFC3339)})
					}
					if result.RuntimeObservationError != "" {
						rows = append(rows, []string{"runtimeObservationError", result.RuntimeObservationError})
					}
					if result.RuntimeObservation != nil {
						rows = append(rows, []string{"runtimeSummary", result.RuntimeObservation.Summary})
					}
					output.WriteTable(c.out, []string{"Field", "Value"}, rows)
					if len(result.Observations) > 0 {
						_, _ = fmt.Fprintln(c.out)
						obsRows := make([][]string, 0, len(result.Observations))
						for _, line := range result.Observations {
							obsRows = append(obsRows, []string{line})
						}
						output.WriteTable(c.out, []string{"Observations"}, obsRows)
					}
					if result.RuntimeObservationError != "" {
						_, _ = fmt.Fprintf(c.out, "runtime observe error: %s\n", result.RuntimeObservationError)
					}
					changes := nonNoopChanges(result.Plan.Changes)
					if len(changes) > 0 {
						_, _ = fmt.Fprintln(c.out)
						planRows := make([][]string, 0, len(changes))
						for _, change := range changes {
							planRows = append(planRows, []string{strings.ToUpper(string(change.Type)), change.ResourceType, change.Name, change.Reason})
						}
						output.WriteTable(c.out, []string{"Type", "Resource", "Name", "Reason"}, planRows)
					}
				}
				return nil
			}
			return output.Encode(c.out, format, result)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().Bool("observe-runtime", false, "Query runtime backend when supported.")
	cmd.Flags().Bool("require-runtime", false, "Require runtime observation support (implies --observe-runtime).")
	cmd.Flags().String("output-dir", "", "Legacy alias for --artifacts-dir.")
	return cmd
}

func newLogsCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Fetch runtime observation details for troubleshooting.",
		Long:  "logs uses runtime observation hooks when available and prints detailed lines.",
		Example: `  chainops logs -f chainops.yaml
  chainops logs -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			format, legacyArtifactsDir, err := c.resolveOutputWithLegacyPath(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			artifactsDir, err := c.resolveArtifactsDir(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(legacyArtifactsDir) != "" {
				artifactsDir = legacyArtifactsDir
			}

			result, diags, statusErr := c.engine.Status(context.Background(), specPath, app.StatusOptions{
				OutputDir:      artifactsDir,
				ObserveRuntime: true,
			})
			if statusErr != nil {
				return runtimeError(statusErr)
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("logs failed due to diagnostics"))
			}

			if result.RuntimeObservation == nil {
				if result.RuntimeObservationError != "" {
					return runtimeError(output.ActionableError(
						"Runtime observation failed.",
						result.RuntimeObservationError,
						"Render artifacts and verify backend runtime prerequisites.",
						fmt.Sprintf("%s doctor -f %s --observe-runtime", c.program, specPath),
					))
				}
				return runtimeError(output.ActionableError(
					"Runtime logs are unavailable.",
					"Current backend does not expose runtime observation details.",
					"Use status/doctor for local snapshot diagnostics.",
					fmt.Sprintf("%s status -f %s", c.program, specPath),
				))
			}

			if format == output.FormatTable {
				rows := [][]string{{"summary", result.RuntimeObservation.Summary}}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				if len(result.RuntimeObservation.Details) > 0 {
					_, _ = fmt.Fprintln(c.out)
					detailRows := make([][]string, 0, len(result.RuntimeObservation.Details))
					for _, line := range result.RuntimeObservation.Details {
						detailRows = append(detailRows, []string{line})
					}
					output.WriteTable(c.out, []string{"Details"}, detailRows)
				}
				return nil
			}
			return output.Encode(c.out, format, result.RuntimeObservation)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().String("output-dir", "", "Legacy alias for --artifacts-dir.")
	return cmd
}

func newDoctorCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run actionable preflight and convergence checks.",
		Long:  "doctor reports pass/warn/fail checks with hints for configuration and runtime readiness.",
		Example: `  chainops doctor -f chainops.yaml
  chainops doctor -f chainops.yaml --observe-runtime
  chainops doctor -f chainops.yaml --require-runtime
  chainops doctor -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			format, legacyArtifactsDir, err := c.resolveOutputWithLegacyPath(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			observeRuntime, _ := cmd.Flags().GetBool("observe-runtime")
			requireRuntime, _ := cmd.Flags().GetBool("require-runtime")
			artifactsDir, err := c.resolveArtifactsDir(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(legacyArtifactsDir) != "" {
				artifactsDir = legacyArtifactsDir
			}

			report, doctorErr := c.engine.Doctor(context.Background(), specPath, app.DoctorOptions{
				OutputDir:      artifactsDir,
				ObserveRuntime: observeRuntime,
				RequireRuntime: requireRuntime,
			})
			if doctorErr != nil {
				return runtimeError(wrapRuntimeUnsupportedError(c.program, "doctor", specPath, doctorErr))
			}

			if format == output.FormatTable {
				if c.legacy {
					if report.ClusterName != "" {
						_, _ = fmt.Fprintf(c.out, "cluster: %s\n", report.ClusterName)
					}
					if report.Backend != "" {
						_, _ = fmt.Fprintf(c.out, "backend: %s\n", report.Backend)
					}
					for _, check := range report.Checks {
						_, _ = fmt.Fprintf(c.out, "[%s] %s: %s\n", strings.ToUpper(string(check.Status)), check.Name, check.Message)
						if check.Hint != "" {
							_, _ = fmt.Fprintf(c.out, "  hint: %s\n", check.Hint)
						}
					}
				} else {
					rows := [][]string{
						{"cluster", report.ClusterName},
						{"backend", report.Backend},
						{"generatedAt", report.GeneratedAt.Format(time.RFC3339)},
					}
					output.WriteTable(c.out, []string{"Field", "Value"}, rows)
					_, _ = fmt.Fprintln(c.out)
					checkRows := make([][]string, 0, len(report.Checks))
					for _, check := range report.Checks {
						checkRows = append(checkRows, []string{strings.ToUpper(string(check.Status)), check.Name, check.Message, check.Hint})
					}
					output.WriteTable(c.out, []string{"Status", "Check", "Message", "Hint"}, checkRows)
				}
			} else {
				if err := output.Encode(c.out, format, report); err != nil {
					return runtimeError(err)
				}
			}
			if report.HasFailures() {
				return runtimeError(fmt.Errorf("doctor found failing checks"))
			}
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().Bool("observe-runtime", false, "Query runtime backend when supported.")
	cmd.Flags().Bool("require-runtime", false, "Require runtime observation support (implies --observe-runtime).")
	cmd.Flags().String("output-dir", "", "Legacy alias for --artifacts-dir.")
	return cmd
}

func newDestroyCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Explicitly tear down local rendered state and snapshots.",
		Long: `destroy is intentionally explicit and safe-by-default.

Current implementation removes local rendered artifacts and local snapshots.
Remote runtime teardown is backend-specific and not yet automated.`,
		Example: `  chainops destroy -f chainops.yaml --yes
  chainops destroy -f chainops.yaml --artifacts-dir .chainops/render --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}

			cluster, diags, validateErr := c.engine.Validate(specPath)
			if validateErr != nil {
				return runtimeError(validateErr)
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("destroy aborted due to validation errors"))
			}

			if err := c.requireConfirmation(cmd, "remove local artifacts and state snapshot"); err != nil {
				return err
			}

			artifactsDir, err := c.resolveArtifactsDir(cmd)
			if err != nil {
				return err
			}
			if err := c.engine.RemoveArtifactsDir(artifactsDir); err != nil {
				return runtimeError(err)
			}

			if err := c.engine.DeleteStateSnapshot(cluster.Metadata.Name, cluster.Spec.Runtime.Backend); err != nil {
				return runtimeError(err)
			}

			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			payload := map[string]any{
				"cluster":      cluster.Metadata.Name,
				"backend":      cluster.Spec.Runtime.Backend,
				"artifactsDir": artifactsDir,
				"snapshotPath": c.engine.ResolveSnapshotPath(cluster.Metadata.Name, cluster.Spec.Runtime.Backend),
				"status":       "destroy completed (local teardown)",
			}
			if format == output.FormatTable {
				rows := [][]string{
					{"cluster", cluster.Metadata.Name},
					{"backend", cluster.Spec.Runtime.Backend},
					{"artifactsDirRemoved", artifactsDir},
					{"snapshotRemoved", c.engine.ResolveSnapshotPath(cluster.Metadata.Name, cluster.Spec.Runtime.Backend)},
					{"status", "destroy completed (local teardown)"},
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				return nil
			}
			return output.Encode(c.out, format, payload)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	cmd.Flags().String("output-dir", "", "Legacy alias for --artifacts-dir.")
	return cmd
}

func newBackupCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Inspect declared backup policy and backend readiness.",
		Long:  "backup currently validates and explains policy intent; runtime backup execution adapters are not implemented yet.",
		Example: `  chainops backup -f chainops.yaml
  chainops backup -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			cluster, diags, validateErr := c.engine.Validate(specPath)
			if validateErr != nil {
				return runtimeError(validateErr)
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("backup policy inspection aborted due to validation errors"))
			}

			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			payload := map[string]any{
				"cluster":        cluster.Metadata.Name,
				"backend":        cluster.Spec.Runtime.Backend,
				"policy":         cluster.Spec.Backup,
				"implementation": "policy-only (runtime adapter pending)",
				"next":           "Use `chainops doctor` + external backup job until adapter lands.",
			}
			if format == output.FormatTable {
				rows := [][]string{
					{"cluster", cluster.Metadata.Name},
					{"backend", cluster.Spec.Runtime.Backend},
					{"enabled", fmt.Sprintf("%t", cluster.Spec.Backup.Enabled)},
					{"schedule", cluster.Spec.Backup.Schedule},
					{"retention", fmt.Sprintf("%d", cluster.Spec.Backup.Retention)},
					{"implementation", "policy-only (runtime adapter pending)"},
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				return nil
			}
			return output.Encode(c.out, format, payload)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	return cmd
}

func newRestoreCmd(c *runtimeContext) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Inspect restore intent and validate prerequisites.",
		Long:  "restore currently validates input + policy intent; runtime restore adapters are not implemented yet.",
		Example: `  chainops restore -f chainops.yaml --from s3://bucket/snapshot.tar
  chainops restore -f chainops.yaml --from ./backup.tar --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(from) == "" {
				return usageError(output.ActionableError(
					"Missing restore source.",
					"--from is required for restore intent.",
					"Provide snapshot URI or local path.",
					fmt.Sprintf("%s restore -f chainops.yaml --from ./backup.tar", c.program),
				))
			}
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			cluster, diags, validateErr := c.engine.Validate(specPath)
			if validateErr != nil {
				return runtimeError(validateErr)
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("restore inspection aborted due to validation errors"))
			}

			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			payload := map[string]any{
				"cluster":        cluster.Metadata.Name,
				"backend":        cluster.Spec.Runtime.Backend,
				"from":           from,
				"implementation": "policy-only (runtime adapter pending)",
			}
			if format == output.FormatTable {
				rows := [][]string{
					{"cluster", cluster.Metadata.Name},
					{"backend", cluster.Spec.Runtime.Backend},
					{"from", from},
					{"implementation", "policy-only (runtime adapter pending)"},
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				return nil
			}
			return output.Encode(c.out, format, payload)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Snapshot source path/URI for restore intent.")
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	return cmd
}

func newUpgradeCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Inspect declared upgrade strategy before rollout.",
		Long:  "upgrade currently validates and explains upgrade policy intent; runtime rollout adapters are not implemented yet.",
		Example: `  chainops upgrade -f chainops.yaml
  chainops upgrade -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			cluster, diags, validateErr := c.engine.Validate(specPath)
			if validateErr != nil {
				return runtimeError(validateErr)
			}
			c.printDiagnostics(diags)
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("upgrade inspection aborted due to validation errors"))
			}

			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			payload := map[string]any{
				"cluster":        cluster.Metadata.Name,
				"backend":        cluster.Spec.Runtime.Backend,
				"policy":         cluster.Spec.Upgrade,
				"implementation": "policy-only (runtime adapter pending)",
			}
			if format == output.FormatTable {
				rows := [][]string{
					{"cluster", cluster.Metadata.Name},
					{"backend", cluster.Spec.Runtime.Backend},
					{"strategy", cluster.Spec.Upgrade.Strategy},
					{"maxUnavailable", fmt.Sprintf("%d", cluster.Spec.Upgrade.MaxUnavailable)},
					{"implementation", "policy-only (runtime adapter pending)"},
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				return nil
			}
			return output.Encode(c.out, format, payload)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	return cmd
}

func newPluginCmd(c *runtimeContext) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered plugins.",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}

			names := c.engine.PluginNames()
			rows := make([][]string, 0, len(names))
			payload := make([]map[string]any, 0, len(names))
			for _, name := range names {
				plugin, _ := c.engine.Registries().Plugins.Get(name)
				caps := plugin.Capabilities()
				rows = append(rows, []string{name, plugin.Family(), fmt.Sprintf("%t", caps.SupportsUpgrade), fmt.Sprintf("%t", caps.SupportsBackup), fmt.Sprintf("%t", caps.SupportsRestore)})
				payload = append(payload, map[string]any{
					"name":   name,
					"family": plugin.Family(),
					"caps":   caps,
				})
			}
			if format == output.FormatTable {
				output.WriteTable(c.out, []string{"Name", "Family", "Upgrade", "Backup", "Restore"}, rows)
				return nil
			}
			return output.Encode(c.out, format, payload)
		},
	}
	listCmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")

	root := &cobra.Command{
		Use:   "plugin",
		Short: "Discover and inspect plugin integrations.",
		Long:  "Plugin commands expose registered family integrations and capabilities.",
		Example: `  chainops plugin list
  chainops explain plugin generic-process`,
		RunE: listCmd.RunE,
	}
	root.Flags().AddFlagSet(listCmd.Flags())
	root.AddCommand(listCmd)
	return root
}

func newProfileCmd(c *runtimeContext) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List built-in onboarding profiles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			profiles := workspace.Profiles()
			if format == output.FormatTable {
				rows := make([][]string, 0, len(profiles))
				for _, profile := range profiles {
					rows = append(rows, []string{profile.Name, profile.Family, profile.Plugin, profile.Backend, profile.Summary})
				}
				output.WriteTable(c.out, []string{"Name", "Family", "Plugin", "Backend", "Summary"}, rows)
				return nil
			}
			return output.Encode(c.out, format, profiles)
		},
	}
	listCmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")

	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show one profile in detail.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			profile, ok := workspace.GetProfile(args[0])
			if !ok {
				return usageError(output.ActionableError(
					"Profile not found.",
					fmt.Sprintf("%q is not a known profile.", args[0]),
					"Run chainops profile list.",
					"chainops profile list",
				))
			}
			if format == output.FormatTable {
				rows := [][]string{
					{"name", profile.Name},
					{"summary", profile.Summary},
					{"family", profile.Family},
					{"plugin", profile.Plugin},
					{"backend", profile.Backend},
					{"intendedUsers", profile.IntendedUsers},
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				return nil
			}
			return output.Encode(c.out, format, profile)
		},
	}
	showCmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")

	root := &cobra.Command{
		Use:     "profile",
		Short:   "Discover and inspect starter profiles.",
		Long:    "Profile commands help onboarding by exposing curated family/plugin/backend presets.",
		Example: "chainops profile list\nchainops profile show local-dev",
		RunE:    listCmd.RunE,
	}
	root.Flags().AddFlagSet(listCmd.Flags())
	root.AddCommand(listCmd, showCmd)
	return root
}

func newContextCmd(c *runtimeContext) *cobra.Command {
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show resolved CLI context and precedence results.",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}
			payload := map[string]any{
				"program":        c.program,
				"legacyMode":     c.legacy,
				"configFile":     c.loadedConfigFile,
				"specFile":       c.cfg.SpecFile,
				"stateDir":       c.cfg.StateDir,
				"artifactsDir":   c.cfg.ArtifactsDir,
				"nonInteractive": c.cfg.NonInteractive,
				"yes":            c.cfg.Yes,
				"output":         c.cfgResolver.OutputFromConfig(),
			}
			if format == output.FormatTable {
				rows := [][]string{
					{"program", c.program},
					{"legacyMode", fmt.Sprintf("%t", c.legacy)},
					{"configFile", fallback(c.loadedConfigFile, "<none>")},
					{"specFile", c.cfg.SpecFile},
					{"stateDir", c.cfg.StateDir},
					{"artifactsDir", c.cfg.ArtifactsDir},
					{"nonInteractive", fmt.Sprintf("%t", c.cfg.NonInteractive)},
					{"yes", fmt.Sprintf("%t", c.cfg.Yes)},
					{"output", fallback(c.cfgResolver.OutputFromConfig(), "table")},
				}
				output.WriteTable(c.out, []string{"Field", "Value"}, rows)
				_, _ = fmt.Fprintln(c.out)
				_, _ = fmt.Fprintln(c.out, "Precedence: defaults < config file < env vars < flags")
				return nil
			}
			return output.Encode(c.out, format, payload)
		},
	}
	showCmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")

	root := &cobra.Command{
		Use:     "context",
		Short:   "Show effective CLI context and resolution.",
		Long:    "Context reports effective values after precedence resolution and loaded config source.",
		Example: "chainops context show",
		RunE:    showCmd.RunE,
	}
	root.Flags().AddFlagSet(showCmd.Flags())
	root.AddCommand(showCmd)
	return root
}

func newValidateCmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "validate",
		Hidden: false,
		Short:  "Validate spec + plugin/backend constraints.",
		Long:   "validate performs schema/defaulting + plugin/backend-specific validation without side effects.",
		Example: `  chainops validate -f chainops.yaml
  chainops validate -f chainops.yaml --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := c.resolveSpecPath(cmd)
			if err != nil {
				return err
			}
			format, err := c.resolveOutputFormat(cmd, "table")
			if err != nil {
				return usageError(err)
			}

			cluster, diags, validateErr := c.engine.Validate(specPath)
			if validateErr != nil {
				return runtimeError(validateErr)
			}
			c.printDiagnostics(diags)
			if c.legacy {
				c.printDiagnosticsTo(c.out, diags)
			}
			if format == output.FormatTable {
				if len(diags) == 0 {
					_, _ = fmt.Fprintln(c.out, "validation passed")
				} else if c.legacy {
					_, _ = fmt.Fprintln(c.out, "validation passed")
				} else {
					rows := make([][]string, 0, len(diags))
					for _, d := range diags {
						rows = append(rows, []string{strings.ToUpper(string(d.Severity)), d.Path, d.Message, d.Hint})
					}
					output.WriteTable(c.out, []string{"Severity", "Path", "Message", "Hint"}, rows)
				}
				if hasErrors(diags) {
					return runtimeError(fmt.Errorf("validation failed"))
				}
				if !c.legacy {
					_, _ = fmt.Fprintf(c.out, "Spec: %s (%s)\n", cluster.Metadata.Name, cluster.Spec.Runtime.Backend)
				}
				return nil
			}
			payload := map[string]any{"spec": cluster, "diagnostics": diags}
			if err := output.Encode(c.out, format, payload); err != nil {
				return runtimeError(err)
			}
			if hasErrors(diags) {
				return runtimeError(fmt.Errorf("validation failed"))
			}
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format: table|json|yaml.")
	return cmd
}

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts.",
		Long:  "Generate autocompletion scripts for supported shells.",
		Example: `  chainops completion bash > /etc/bash_completion.d/chainops
  chainops completion zsh > "${fpath[1]}/_chainops"
  chainops completion fish > ~/.config/fish/completions/chainops.fish
  chainops completion powershell > chainops.ps1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return usageError(output.ActionableError(
					"Unsupported shell.",
					fmt.Sprintf("%q is not supported.", args[0]),
					"Use bash, zsh, fish, or powershell.",
					"chainops completion zsh",
				))
			}
		},
	}
}

func newVersionCmd(program string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show CLI version and build identity.",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", program, version)
		},
	}
}

func newTUICmd(c *runtimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "tui",
		Hidden: false,
		Short:  "Launch legacy interactive TUI (bgorch compatibility mode).",
		RunE: func(cmd *cobra.Command, args []string) error {
			legacyApp := app.New(app.Options{StateDir: c.cfg.StateDir})
			if err := tui.Run(legacyApp); err != nil {
				return runtimeError(fmt.Errorf("tui failed: %w", err))
			}
			return nil
		},
	}
	alias := &cobra.Command{
		Use:    "ui",
		Hidden: true,
		RunE:   cmd.RunE,
	}
	cmd.AddCommand(alias)
	return cmd
}

func renderSchemaDoc(outWriter io.Writer, format string, doc schema.Doc) error {
	if format == output.FormatTable {
		rows := [][]string{{"path", doc.Path}, {"summary", doc.Summary}, {"description", doc.Description}}
		output.WriteTable(outWriter, []string{"Field", "Value"}, rows)
		if len(doc.Fields) > 0 {
			_, _ = fmt.Fprintln(outWriter)
			fieldRows := make([][]string, 0, len(doc.Fields))
			for _, field := range doc.Fields {
				fieldRows = append(fieldRows, []string{field.Name, field.Type, fmt.Sprintf("%t", field.Required), field.Description})
			}
			output.WriteTable(outWriter, []string{"Field", "Type", "Required", "Description"}, fieldRows)
		}
		if len(doc.Examples) > 0 {
			_, _ = fmt.Fprintln(outWriter)
			for _, ex := range doc.Examples {
				_, _ = fmt.Fprintf(outWriter, "Example: %s\n", ex)
			}
		}
		if len(doc.SeeAlso) > 0 {
			_, _ = fmt.Fprintln(outWriter)
			_, _ = fmt.Fprintf(outWriter, "See also: %s\n", strings.Join(doc.SeeAlso, ", "))
		}
		return nil
	}
	return output.Encode(outWriter, format, doc)
}

func hasErrors(diags []domain.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == domain.SeverityError {
			return true
		}
	}
	return false
}

func nonNoopChanges(changes []domain.PlanChange) []domain.PlanChange {
	out := make([]domain.PlanChange, 0, len(changes))
	for _, c := range changes {
		if c.Type != domain.ChangeNoop {
			out = append(out, c)
		}
	}
	return out
}

func summarizePlan(plan domain.Plan) (create, update, del, noop int) {
	for _, c := range plan.Changes {
		switch c.Type {
		case domain.ChangeCreate:
			create++
		case domain.ChangeUpdate:
			update++
		case domain.ChangeDelete:
			del++
		case domain.ChangeNoop:
			noop++
		}
	}
	return create, update, del, noop
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func promptWithDefault(label, current, defaultValue string) string {
	current = strings.TrimSpace(current)
	if current != "" {
		defaultValue = current
	}
	if !isInteractiveSession() {
		return defaultValue
	}
	reader := bufio.NewReader(os.Stdin)
	if defaultValue != "" {
		_, _ = fmt.Fprintf(os.Stdout, "%s [%s]: ", label, defaultValue)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "%s: ", label)
	}
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func isInteractiveSession() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func looksLikeUsageError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unknown command") ||
		strings.Contains(text, "flag needs an argument") ||
		strings.Contains(text, "required flag") ||
		strings.Contains(text, "accepts")
}

func writeEncodedToFile(path, format string, payload any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()
	if format == output.FormatTable {
		format = output.FormatYAML
	}
	if err := output.Encode(f, format, payload); err != nil {
		return fmt.Errorf("encode output file %q: %w", path, err)
	}
	return nil
}
