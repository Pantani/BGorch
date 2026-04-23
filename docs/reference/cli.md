# CLI Reference

## Command Tree

```text
chainops init
chainops explain
chainops render
chainops plan
chainops diff
chainops apply
chainops status
chainops logs
chainops doctor
chainops destroy
chainops backup
chainops restore
chainops upgrade
chainops plugin list
chainops profile list|show
chainops context show
chainops validate
chainops completion
chainops version
```

## Core Flow

```bash
chainops init
chainops doctor
chainops render -f chainops.yaml -o yaml
chainops plan -f chainops.yaml --out plan.json
chainops apply plan.json --yes
```

## Output Modes

Main commands accept `--output` with:

- `table` (default for humans)
- `json` (automation)
- `yaml` (auditability/readability)

## Important Flags

Global:

- `--config`
- `-f, --file`
- `--state-dir`
- `--artifacts-dir`
- `--non-interactive`
- `--yes`

Per-command:

- `plan --out <plan.json|plan.yaml>`
- `apply [plan-file]`
- `apply --runtime-exec`
- `apply --require-runtime` (implies runtime-exec and fails if runtime is unavailable)
- `status --observe-runtime`
- `status --require-runtime` (implies observe-runtime and fails if runtime is unavailable)
- `doctor --observe-runtime`
- `doctor --require-runtime` (implies observe-runtime and fails if runtime is unavailable)
- `render --write-artifacts`

## Render Modes

### Canonical Config (default)

```bash
chainops render -f chainops.yaml -o yaml
chainops render -f chainops.yaml --format json
```

### Artifacts Mode

```bash
chainops render -f chainops.yaml --write-artifacts --artifacts-dir .chainops/render
```

Legacy compatibility:

```bash
chainops render -f chainops.yaml -o .chainops/render
```

## Plan / Apply

### Side-Effect-Free Plan

```bash
chainops plan -f chainops.yaml
```

### Persist a Plan for Handoff

```bash
chainops plan -f chainops.yaml --out plan.json
```

### Apply Using a Saved Plan

```bash
chainops apply plan.json --yes
```

### Strict Runtime Mode

```bash
# apply requires and executes runtime
chainops apply -f chainops.yaml --require-runtime --yes

# status/doctor require runtime observation
chainops status -f chainops.yaml --require-runtime
chainops doctor -f chainops.yaml --require-runtime
```

## Explain Examples

```bash
chainops explain ChainCluster
chainops explain ChainCluster.spec.runtime
chainops explain plugin generic-process
chainops explain profile local-dev
```

## Completion

```bash
chainops completion bash > /etc/bash_completion.d/chainops
chainops completion zsh > "${fpath[1]}/_chainops"
chainops completion fish > ~/.config/fish/completions/chainops.fish
chainops completion powershell > chainops.ps1
```

## Legacy Compatibility

`bgorch` remains functional for gradual migration. See [docs/migrations/cli-redesign.md](../migrations/cli-redesign.md).
