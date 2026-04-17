# `internal/domain`

Shared DTOs exchanged between core pipelines, plugins/backends, planner, and output layers.

## Responsibility

- define stable in-process models for diagnostics, desired state, and plans;
- keep planner/app/backend contracts explicit and decoupled from CLI/UI rendering;
- centralize change semantics (`create|update|delete|noop`).

## Core Models

- `Diagnostic`: structured validation/resolution feedback (`severity`, `path`, `hint`).
- `DesiredState`: normalized runtime intent (services, volumes, networks, artifacts, metadata).
- `Plan` / `PlanChange`: ordered reconciliation delta against snapshot.

## Interaction Model

1. validators/plugin/backend emit `Diagnostic`.
2. plugin/backend build `DesiredState`.
3. planner compares desired state to snapshot and emits `Plan`.
4. CLI/output encodes these models for operator workflows.

## Constraint

Domain package must stay backend/plugin agnostic and avoid command/runtime side effects.
