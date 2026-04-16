# Config Precedence

Chainops resolve configuração nesta ordem (menor para maior precedência):

```text
defaults < config file < env vars < flags
```

## Fontes

- Defaults internos (`internal/config.DefaultValues`).
- Arquivo de config via `--config`.
- Variáveis de ambiente com prefixo `CHAINOPS_`.
- Flags da linha de comando.

## Chaves suportadas

- `config`
- `file`
- `state-dir`
- `artifacts-dir`
- `output`
- `non-interactive`
- `yes`

## Exemplos

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

### 2) Env sobrescreve config file

```bash
export CHAINOPS_OUTPUT=json
chainops --config chainops-cli.yaml status
```

### 3) Flag sobrescreve env/config

```bash
CHAINOPS_STATE_DIR=/tmp/state-env \
chainops --config chainops-cli.yaml --state-dir /tmp/state-flag plan
```

Resultado: `state-dir = /tmp/state-flag`.

## Observabilidade da resolução

Use `chainops context show` para inspecionar valores efetivos e arquivo carregado.
