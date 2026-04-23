# `internal/backend`

Backends translate plugin output + spec into runtime-specific desired state and optional runtime actions.

## Contracts

### Required (`Backend`)

- `Name() string`
- `ValidateTarget(spec)`
- `BuildDesired(ctx, spec, pluginOut)`

### Optional runtime capabilities

- `RuntimeExecutor` (`ExecuteRuntime`)
- `RuntimeObserver` (`ObserveRuntime`)

## Implementations

- `compose`: container-mode backend; render + runtime exec/observe.
- `sshsystemd`: host-mode backend; render + runtime exec/observe via SSH/systemctl.
- `kubernetes`: manifest render + runtime observe (`kubectl`), without runtime exec.
- `terraform`: deterministic infrastructure adapter scaffold (artifact mode).
- `ansible`: deterministic bootstrap adapter scaffold (artifact mode).

## Interaction Model

1. plugin emits normalized output (artifacts/metadata),
2. backend validates runtime constraints,
3. backend builds `domain.DesiredState`,
4. runtime interfaces are invoked only when command flags request them.

## Extension Guidance

When adding a backend:

1. keep output deterministic and sorted,
2. return actionable diagnostics in `ValidateTarget`,
3. keep chain-specific semantics out of backend code,
4. implement runtime interfaces only with clear capability/preflight behavior,
5. add backend-focused tests (golden + runtime behavior/fallbacks).
