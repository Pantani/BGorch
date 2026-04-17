# Request Lifecycle and Runtime Flows

This document describes command execution flow as implemented in `internal/app`, `internal/engine`, and related packages.

## 1. End-to-end command lifecycle

### `validate`

1. Load YAML via `spec.LoadFromFile`.
2. Apply defaults (`spec.ApplyDefaults`).
3. Resolve plugin/backend from registries.
4. Execute validations:
   - plugin-level (`plugin.Validate`),
   - backend-level (`backend.ValidateTarget`),
   - core schema/domain (`validate.Cluster`).
5. Return diagnostics only (no artifact/state mutation).

### `render`

1. Run validation + resolution pipeline.
2. Build desired state:
   - `plugin.Normalize`,
   - `plugin.Build`,
   - `backend.BuildDesired`.
3. By default, return canonical resolved config (`chainops render -o yaml|json`).
4. In artifact mode (`--write-artifacts`), write artifacts via `renderer.WriteArtifacts`.
5. Optionally persist snapshot when `--write-state` is enabled.

### `plan` / `diff`

1. Build desired state.
2. Load snapshot from `<state-dir>/<cluster>--<backend>.json`.
3. Compare desired vs snapshot hashes (`planner.Build`).
4. Return ordered change list (`create|update|delete|noop`).
5. `diff` is a filtered non-noop view of plan.

### `apply`

1. Resolve input (`-f <spec>` or `<plan-file>` with `sourceSpec`).
2. Build desired state.
3. Acquire exclusive lock `<state-dir>/<cluster>--<backend>.lock`.
4. Load current snapshot.
5. Build plan.
6. If `--dry-run`, return plan without writing artifacts/snapshot.
7. Write artifacts.
8. If `--runtime-exec` or `--require-runtime`, execute backend runtime action.
9. `--require-runtime` fails on missing capability/preconditions/runtime errors.
10. Save new snapshot.
11. Release lock.

Failure semantics:

- lock acquisition failure aborts apply,
- runtime execution failure aborts apply and does not persist snapshot,
- lock release runs in `defer`; release errors surface when no prior error exists.

### `status`

1. Validate spec.
2. Build desired state.
3. Load snapshot and compute plan diff.
4. Produce convergence observations.
5. If `--observe-runtime`, call runtime observer when backend supports it.
6. If `--require-runtime`, runtime observe is mandatory (`--observe-runtime` implied).

Observation failure semantics:

- by default: runtime observe errors are non-fatal and surfaced in output,
- strict mode (`--require-runtime`): runtime observe errors fail the command.

### `doctor`

`doctor` aggregates checks across:

- spec loading/validation,
- plugin/backend resolution,
- state directory accessibility,
- snapshot readability,
- desired vs snapshot drift,
- optional runtime observation checks,
- strict mode (`--require-runtime`) for mandatory runtime observation.

`doctor` statuses:

- `pass`: healthy,
- `warn`: degraded/non-blocking,
- `fail`: blocking/actionable.

## 2. State model and transitions

State backend is filesystem-based under CLI `state-dir`:

- default `chainops`: `.chainops/state`
- default `bgorch` alias: `.bgorch/state`

Artifacts:

- snapshot: `<cluster>--<backend>.json`
- lock: `<cluster>--<backend>.lock`

Snapshot model:

- `services`: map `serviceName -> hash(json(service))`
- `artifacts`: map `artifactPath -> hash(content)`
- metadata: `version`, `clusterName`, `backend`, `updatedAt`

Transition model:

1. missing snapshot + desired => `create`
2. hash match => `noop`
3. hash mismatch => `update`
4. missing in desired/current => `delete`/`create`

## 3. Concurrency and locking

Lock scope: `(clusterName, backend)`.

Implementation:

- atomic acquisition via `os.O_CREATE|os.O_EXCL`,
- lock metadata stores pid + timestamp,
- release is idempotent (`sync.Once`).

Guarantee:

- protects local concurrent `apply` on a single machine.

Non-goal:

- distributed lock coordination across multiple hosts.

## 4. Plugin/backend interaction flow

```text
ChainCluster
  -> Plugin.Validate / Normalize / Build
  -> plugin Output (artifacts + metadata)
  -> Backend.ValidateTarget / BuildDesired
  -> DesiredState
```

Ownership boundaries:

- plugin: chain-family semantics,
- backend: runtime translation/execution semantics,
- core: orchestration, diagnostics, planning, state persistence.

## 5. Runtime integrations

### Compose (`docker-compose`)

- always renders compose artifacts,
- runtime exec: `docker compose ... up -d`,
- runtime observe: `docker compose ... ps --all`.

### SSH/systemd (`ssh-systemd`)

- validates host-mode workloads,
- renders unit/env/layout artifacts,
- runtime exec: SSH preflight (`systemctl --version`),
- runtime observe: SSH `systemctl list-units`.

### Kubernetes (`kubernetes`)

- validates container workloads with image constraints,
- renders deterministic manifests (`Service` + `StatefulSet` + `PVC`),
- runtime observe via `kubectl get` with cluster label selector,
- no runtime exec in current implementation.

### Terraform (`terraform`) and Ansible (`ansible`)

- deterministic adapter artifact generation,
- no runtime exec/observe in current implementation.

## 6. Configuration loading and override rules

Spec defaults (`spec.ApplyDefaults`):

- `apiVersion`: `bgorch.io/v1alpha1`
- `kind`: `ChainCluster`
- plugin auto-selection by family alias map,
- compose output file default: `compose.yaml`,
- node/workload defaults: replicas/mode/restart/protocol.

CLI config precedence:

```text
defaults < config file < env vars < flags
```

Relevant flags:

- `--file`, `--state-dir`, `--artifacts-dir`, `--output`, `--yes`, `--non-interactive`.

## 7. Error handling strategy

Two channels:

- diagnostics (`[]domain.Diagnostic`) for validation/resolution issues,
- returned `error` for operational/internal failures.

Practical effects:

- validation issues are visible and structured,
- runtime/filesystem/process failures short-circuit command execution,
- actionable errors include probable cause + fix hint + next command.

## 8. Background jobs / workers / eventing

Current implementation has no persistent background workers, event bus, or scheduler.

- execution is synchronous per CLI invocation,
- no long-lived reconciliation controller loop exists yet.

## 9. AuthN/AuthZ

No built-in authentication/authorization layer exists.

- commands execute with local process permissions,
- runtime integrations inherit host/Kubernetes credentials from execution environment,
- secret references are modeled in schema but secret store integrations are not implemented end-to-end.

## 10. Testing strategy (implemented)

- unit tests for validation/planner/lock/model utilities,
- golden tests for renderer/backends/plugins,
- app-layer tests for dry-run/idempotence/runtime gates/fallbacks,
- integration/regression tests for CLI user flows and lock contention.
