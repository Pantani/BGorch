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

Comandos principais aceitam `--output` com:

- `table` (default para humanos)
- `json` (automação)
- `yaml` (auditoria/leitura)

## Important Flags

Globais:

- `--config`
- `-f, --file`
- `--state-dir`
- `--artifacts-dir`
- `--non-interactive`
- `--yes`

Comandos:

- `plan --out <plan.json|plan.yaml>`
- `apply [plan-file]`
- `status --observe-runtime`
- `doctor --observe-runtime`
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

Compat legado:

```bash
chainops render -f chainops.yaml -o .chainops/render
```

## Plan / Apply

### Plan side-effect free

```bash
chainops plan -f chainops.yaml
```

### Persist plan for handoff

```bash
chainops plan -f chainops.yaml --out plan.json
```

### Apply using saved plan

```bash
chainops apply plan.json --yes
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

`bgorch` permanece funcional para migração gradual. Consulte [docs/migrations/cli-redesign.md](../migrations/cli-redesign.md).
