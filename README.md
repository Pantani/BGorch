# Chainops

Chainops é um orquestrador declarativo, Go-first, para operações multi-blockchain.

`bgorch` permanece suportado como alias legado para migração incremental.

## Visão Geral

Problema que o projeto resolve:

- convergência determinística de topologias blockchain multi-processo,
- separação clara entre semântica de chain (plugin) e semântica de runtime (backend),
- fluxo previsível para operadores: `init -> doctor -> render -> plan -> apply -> status`.

Pilares:

- desired state com diff determinístico,
- UX orientada a operador (saídas acionáveis, confirmação explícita, formato estável),
- extensibilidade por plugins/backends,
- suporte a múltiplos alvos (compose, ssh+systemd, kubernetes, terraform, ansible).

## Features Principais

- API versionada `bgorch.io/v1alpha1` (`ChainCluster`).
- CLI principal `chainops` com comandos de onboarding, explainability, plan/apply, observabilidade e gestão contextual.
- Plugins de família:
  - `generic-process`
  - `cometbft-family`
  - `evm-family`
  - `solana-family`
  - `bitcoin-family`
  - `cosmos-family`
- Backends:
  - `docker-compose`
  - `ssh-systemd`
  - `kubernetes` (render + observe)
  - `terraform` (adapter em modo artifact)
  - `ansible` (adapter em modo artifact)
- Runtime gates explícitos:
  - `--runtime-exec`
  - `--observe-runtime`
  - `--require-runtime`
- Snapshot + lock por `(cluster, backend)` para segurança de `apply`.

## Arquitetura (High-Level)

```text
ChainCluster spec
  -> load + defaults
  -> validate (core + plugin + backend)
  -> plugin.Normalize + plugin.Build
  -> backend.BuildDesired
  -> DesiredState
     -> render (canonical/artifacts)
     -> plan (desired vs snapshot)
     -> apply (lock + render + runtime opcional + snapshot)
     -> status/doctor (convergência + runtime observe opcional)
```

Fronteiras arquiteturais:

- `internal/cli`: UX, parse de flags, mensagens e comando.
- `internal/engine`/`internal/app`: orquestração de pipelines.
- `internal/chain`: comportamento específico de família.
- `internal/backend`: tradução/execução específica de runtime.
- `internal/planner` + `internal/state`: diff e persistência local determinística.

## Stack Técnica

- Go `1.22+`
- CLI: [Cobra](https://github.com/spf13/cobra)
- Config precedence: [Viper](https://github.com/spf13/viper)
- YAML: `gopkg.in/yaml.v3`
- TUI (modo legado `bgorch tui`): [Bubble Tea](https://github.com/charmbracelet/bubbletea)

## Setup Local

Pré-requisitos:

- Go `1.22+`
- opcional: Docker + Compose plugin
- opcional: `ssh` + `systemctl` remoto para runtime ssh-systemd
- opcional: `kubectl` para observação runtime kubernetes

Instalação rápida:

```bash
go mod download
```

## Como Rodar

```bash
# help raiz
chainops --help

# onboarding inicial
chainops init --profile local-dev --name demo

# fluxo principal
chainops doctor -f chainops.yaml
chainops render -f chainops.yaml -o yaml
chainops plan -f chainops.yaml --out plan.json
chainops apply plan.json --yes
```

Alias legado:

```bash
bgorch --help
```

## Variáveis de Ambiente (Overview)

Prefixo padrão: `CHAINOPS_`.

Mapeamento direto de chaves CLI:

- `CHAINOPS_CONFIG`
- `CHAINOPS_FILE`
- `CHAINOPS_STATE_DIR`
- `CHAINOPS_ARTIFACTS_DIR`
- `CHAINOPS_OUTPUT`
- `CHAINOPS_NON_INTERACTIVE`
- `CHAINOPS_YES`

Precedência efetiva:

```text
defaults < config file < env vars < flags
```

Referência completa:

- [docs/reference/config-precedence.md](docs/reference/config-precedence.md)
- [docs/reference/configuration-model.md](docs/reference/configuration-model.md)

## Testes

```bash
go test ./...
# ou
make test
```

## Lint e Format

```bash
make fmt        # gofmt -w
make format     # check gofmt
make vet
make lint       # requer golangci-lint instalado
```

## Build e Deploy

Build local:

```bash
make build
# binário: ./bin/chainops
```

Instalar no GOPATH/bin:

```bash
make install
```

Deploy/release:

- não há pipeline de release automatizado no repositório;
- fluxo atual é manual (verify -> build -> publicação externa de artefatos/changelog).

## Diretórios Importantes

- `cmd/chainops`: entrypoint principal.
- `cmd/bgorch`: alias legado.
- `internal/cli`: árvore de comandos e UX.
- `internal/config`: resolução de config/env/flags.
- `internal/engine`: façade para pipelines do core.
- `internal/app`: pipelines declarativos (`validate/render/plan/apply/status/doctor`).
- `internal/api/v1alpha1`: tipos do schema.
- `internal/chain`: plugins por família.
- `internal/backend`: backends/adapters por runtime.
- `internal/planner`: diff deterministic.
- `internal/state`: snapshots + locks.
- `internal/output`: serialização table/json/yaml e erros acionáveis.
- `internal/workspace`: profiles e templates de `init`.
- `pkg/pluginapi`: contrato versionável para SDK/plugin ecosystem.
- `examples`: specs de referência.
- `test`: integração/regressão/golden.
- `docs`: arquitetura, ADRs, operação, schema e UX.

## Assunções e Riscos Operacionais

- lock/snapshot são locais (`.chainops/state` por default), não distribuídos;
- `plan` e render canônico são side-effect free por design;
- runtime ops são backend-gated e falham rápido em preflight/capability mismatch;
- `kubernetes` hoje observa runtime mas não executa runtime apply;
- adapters `terraform`/`ansible` estão em modo artifact (sem runtime exec/observe).

## Mapa de Documentação

- [CLI reference](docs/reference/cli.md)
- [Developer workflows](docs/development/developer-workflows.md)
- [Request lifecycle](docs/architecture/request-lifecycle.md)
- [Architecture target](docs/architecture/target-architecture.md)
- [Operational guide](docs/operations/commands-and-integration.md)
- [ADRs](docs/adr)
