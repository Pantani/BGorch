# Operational Guide: Commands and Plugin/Backend Integration

This guide reflects the current implementation in the repository.

## Command Matrix

| Command | Implemented | Notes |
|---|---|---|
| `validate -f <spec>` | Yes | Runs plugin + backend + core validation. |
| `render -f <spec>` | Yes | Canonical resolved config by default. |
| `render --write-artifacts` | Yes | Writes desired artifacts to artifacts dir. |
| `render --write-state` | Yes | Persists desired snapshot without runtime execution. |
| `plan -f <spec>` | Yes | Diff is snapshot-based (local filesystem). |
| `plan --out <plan.{json|yaml}>` | Yes | Persists plan envelope for handoff/apply. |
| `diff -f <spec>` | Yes | Focused non-noop view of plan. |
| `apply -f <spec>` | Yes | Lock + plan + artifact write + snapshot save. |
| `apply <plan-file>` | Yes | Uses `sourceSpec` from persisted plan file. |
| `apply --runtime-exec` | Yes (backend-gated) | Executes backend runtime operation after successful render. |
| `apply --require-runtime` | Yes | Implies runtime execution and fails when capability/prerequisites are unavailable. |
| `status --observe-runtime` | Yes (backend-gated) | Runs runtime observe when backend supports it and prerequisites are met. |
| `status --require-runtime` | Yes | Implies observe-runtime and fails when capability/prerequisites are unavailable. |
| `doctor --observe-runtime` | Yes (backend-gated) | Adds runtime checks when backend supports observation. |
| `doctor --require-runtime` | Yes | Implies observe-runtime and fails when capability/prerequisites are unavailable. |

## Recommended Flow

```bash
chainops doctor -f chainops.yaml
chainops render -f chainops.yaml -o yaml
chainops plan -f chainops.yaml --out plan.json
chainops apply plan.json --yes
chainops status -f chainops.yaml
```

Compose runtime execution/observation:

```bash
chainops apply  -f examples/generic-single-compose.yaml --runtime-exec --yes
chainops status -f examples/generic-single-compose.yaml --observe-runtime
chainops doctor -f examples/generic-single-compose.yaml --observe-runtime
```

SSH/systemd runtime execution/observation (requires `spec.runtime.target` + SSH/systemctl reachability):

```bash
chainops apply  -f examples/generic-single-ssh-systemd.yaml --runtime-exec --yes
chainops status -f examples/generic-single-ssh-systemd.yaml --observe-runtime
chainops doctor -f examples/generic-single-ssh-systemd.yaml --observe-runtime
```

## Current Semantics

### `apply`

- resolves plugin/backend and builds desired state first;
- acquires lock by `(cluster, backend)`;
- computes plan against snapshot;
- writes artifacts unless `--dry-run`;
- optionally executes backend runtime if runtime flags are enabled;
- `--require-runtime` implies runtime execution and returns non-zero on capability/preflight/runtime failures;
- persists snapshot only after successful artifact write and optional runtime execution.

### `status`

- validates spec and computes desired-vs-snapshot diff;
- reports observations even when runtime observation is not requested;
- runtime observation errors are non-fatal by default;
- with `--require-runtime`, runtime observation is mandatory and failures become command errors.

### `doctor`

- emits `pass/warn/fail` checks for validation, resolution, state access, snapshot, drift;
- runtime observation checks are optional by default;
- with `--require-runtime`, runtime observation is mandatory and failures become command errors.

## Backend Capability Matrix

| Backend | BuildDesired | Runtime Exec | Runtime Observe | Notes |
|---|---|---|---|---|
| `docker-compose` | Yes | Yes | Yes | Requires Docker/Compose and rendered compose file. |
| `ssh-systemd` | Yes | Yes | Yes | Requires rendered artifacts, runtime targets, `ssh`, and remote `systemctl`. |
| `kubernetes` | Yes | No | Yes | Observe via `kubectl` against resources labeled `chainops.io/cluster=<name>`. |
| `terraform` | Yes | No | No | Deterministic infra scaffold only (`terraform/*`). |
| `ansible` | Yes | No | No | Deterministic bootstrap scaffold only (`ansible/*`). |

## Plugin and Backend Contracts

### Plugin contract (`internal/chain.Plugin`)

- `Validate(spec)`
- `Normalize(spec)`
- `Build(ctx, spec) -> Output`
- `Capabilities()`

Responsibilities:

- chain-family specific semantics,
- plugin-specific artifact generation,
- typed extension validation/defaulting inside chain scope.

### Backend contract (`internal/backend.Backend`)

- `ValidateTarget(spec)`
- `BuildDesired(ctx, spec, pluginOut)`

Optional runtime capabilities:

- `RuntimeExecutor` (`ExecuteRuntime`)
- `RuntimeObserver` (`ObserveRuntime`)

Responsibilities:

- runtime-specific validation and artifact translation,
- optional runtime command execution/observation,
- no chain-protocol business logic.

## Known Operational Limits

- snapshot/lock model is local filesystem based;
- no distributed lock/state backend;
- runtime operations are synchronous and command-driven (no reconcile loop);
- `kubernetes` does not expose runtime execution yet;
- `terraform` and `ansible` are still artifact-mode adapters.
