# `internal/planner`

Deterministic planning over desired state vs stored snapshot.

## Responsibility

- compute `create|update|delete|noop` changes from hashed snapshots,
- preserve stable ordering for deterministic output,
- persist/reload plan-file envelopes for review/handoff workflows.

## Entrypoints

- `Build(desired, current)`
- `NewFile`, `WriteFile`, `ReadFile`

## Caveats

- planner is intentionally runtime-agnostic,
- behavior depends on snapshot fidelity from `internal/state`.
