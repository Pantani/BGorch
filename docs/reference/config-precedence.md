# Config Precedence

Chainops resolves configuration in this order (lowest to highest precedence):

```text
defaults < config file < env vars < flags
```

## Sources

- Internal defaults (`internal/config.DefaultValues`).
- Config file via `--config`.
- Environment variables with the `CHAINOPS_` prefix.
- Command-line flags.

## Supported Keys

- `config`
- `file`
- `state-dir`
- `artifacts-dir`
- `output`
- `non-interactive`
- `yes`

## Examples

### 1) Config file base

```yaml
# chainops-cli.yaml
file: examples/starter-local-dev.yaml
state-dir: .chainops/state
artifacts-dir: .chainops/render
output: table
yes: false
```

```bash
chainops --config chainops-cli.yaml plan
```

### 2) Env overrides config file

```bash
export CHAINOPS_OUTPUT=json
chainops --config chainops-cli.yaml status
```

### 3) Flag overrides env/config

```bash
CHAINOPS_STATE_DIR=/tmp/state-env \
chainops --config chainops-cli.yaml --state-dir /tmp/state-flag plan
```

Result: `state-dir = /tmp/state-flag`.

## Resolution Observability

Use `chainops context show` to inspect effective values and the loaded file.
