# Chainops

Chainops is a Go-first declarative orchestrator for multi-blockchain operations.

`bgorch` remains supported as a legacy alias for incremental migration.

## Overview

Problems the project solves:

- deterministic convergence for multi-process blockchain topologies,
- clear separation between chain semantics (plugin) and runtime semantics (backend),
- a predictable operator flow: `init -> doctor -> render -> plan -> apply -> status`.

Pillars:

- desired state with deterministic diff,
- operator-focused UX (actionable output, explicit confirmation, stable formatting),
- plugin/backend extensibility,
- support for multiple targets (compose, ssh+systemd, kubernetes, terraform, ansible).

## Main Features

- Versioned API `bgorch.io/v1alpha1` (`ChainCluster`).
- Primary CLI `chainops` with onboarding, explainability, plan/apply, observability, and context-management commands.
- Family plugins:
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
  - `terraform` (artifact-mode adapter)
  - `ansible` (artifact-mode adapter)
- Explicit runtime gates:
  - `--runtime-exec`
  - `--observe-runtime`
  - `--require-runtime`
- Snapshot + lock per `(cluster, backend)` for `apply` safety.

## Architecture (High-Level)

```text
ChainCluster spec
  -> load + defaults
  -> validate (core + plugin + backend)
  -> plugin.Normalize + plugin.Build
  -> backend.BuildDesired
  -> DesiredState
     -> render (canonical/artifacts)
     -> plan (desired vs snapshot)
     -> apply (lock + render + optional runtime + snapshot)
     -> status/doctor (convergence + optional runtime observe)
```

Architectural boundaries:

- `internal/cli`: UX, flag parsing, messaging, and command routing.
- `internal/engine`/`internal/app`: pipeline orchestration.
- `internal/chain`: family-specific behavior.
- `internal/backend`: runtime-specific translation and execution.
- `internal/planner` + `internal/state`: deterministic diff and local persistence.

## Technical Stack

- Go `1.22+`
- CLI: [Cobra](https://github.com/spf13/cobra)
- Config precedence: [Viper](https://github.com/spf13/viper)
- YAML: `gopkg.in/yaml.v3`
- TUI (legacy `bgorch tui` mode): [Bubble Tea](https://github.com/charmbracelet/bubbletea)

## Local Setup

Prerequisites:

- Go `1.22+`
- optional: Docker + Compose plugin
- optional: remote `ssh` + `systemctl` for the `ssh-systemd` runtime
- optional: `kubectl` for Kubernetes runtime observation

Quick install:

```bash
go mod download
```

## How To Run

```bash
# root help
chainops --help

# initial onboarding
chainops init --profile local-dev --name demo

# main flow
chainops doctor -f chainops.yaml
chainops render -f chainops.yaml -o yaml
chainops plan -f chainops.yaml --out plan.json
chainops apply plan.json --yes
```

Legacy alias:

```bash
bgorch --help
```

## Environment Variables (Overview)

Default prefix: `CHAINOPS_`.

Direct mapping of CLI keys:

- `CHAINOPS_CONFIG`
- `CHAINOPS_FILE`
- `CHAINOPS_STATE_DIR`
- `CHAINOPS_ARTIFACTS_DIR`
- `CHAINOPS_OUTPUT`
- `CHAINOPS_NON_INTERACTIVE`
- `CHAINOPS_YES`

Effective precedence:

```text
defaults < config file < env vars < flags
```

Full reference:

- [docs/reference/config-precedence.md](docs/reference/config-precedence.md)
- [docs/reference/configuration-model.md](docs/reference/configuration-model.md)

## Tests

```bash
go test ./...
# or
make test
```

## Lint and Format

```bash
make fmt        # gofmt -w
make format     # check gofmt
make vet
make lint       # requires golangci-lint to be installed
```

## Build and Deploy

Local build:

```bash
make build
# binary: ./bin/chainops
```

Install into `GOPATH/bin`:

```bash
make install
```

Deploy/release:

- there is no automated release pipeline in the repository;
- the current flow is manual (`verify -> build -> external artifact/changelog publishing`).

## Important Directories

- `cmd/chainops`: primary entrypoint.
- `cmd/bgorch`: legacy alias.
- `internal/cli`: command tree and UX.
- `internal/config`: config/env/flag resolution.
- `internal/engine`: facade for core pipelines.
- `internal/app`: declarative pipelines (`validate/render/plan/apply/status/doctor`).
- `internal/api/v1alpha1`: schema types.
- `internal/chain`: family plugins.
- `internal/backend`: backend/runtime adapters.
- `internal/planner`: deterministic diff engine.
- `internal/state`: snapshots + locks.
- `internal/output`: table/json/yaml serialization and actionable errors.
- `internal/workspace`: `init` profiles and templates.
- `pkg/pluginapi`: versioned contract for the SDK/plugin ecosystem.
- `examples`: reference specs.
- `test`: integration/regression/golden tests.
- `docs`: architecture, ADRs, operations, schema, and UX.

## Operational Assumptions and Risks

- lock/snapshot are local (`.chainops/state` by default), not distributed;
- `plan` and canonical render are side-effect free by design;
- runtime operations are backend-gated and fail fast on preflight/capability mismatch;
- `kubernetes` currently observes runtime but does not execute runtime apply;
- `terraform`/`ansible` adapters are currently artifact-mode only (no runtime exec/observe).

## Documentation Map

- [CLI reference](docs/reference/cli.md)
- [Developer workflows](docs/development/developer-workflows.md)
- [Request lifecycle](docs/architecture/request-lifecycle.md)
- [Architecture target](docs/architecture/target-architecture.md)
- [Operational guide](docs/operations/commands-and-integration.md)
- [ADRs](docs/adr)
