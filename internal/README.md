# Internal Package Map

This directory contains Chainops implementation packages (non-stable internal API).

## Package Responsibilities

- `api`: versioned schema package namespace.
- `api/v1alpha1`: schema types (`ChainCluster`) and typed extension blocks.
- `app`: core command pipelines and runtime gating semantics.
- `backend`: backend interfaces + implementations.
- `chain`: plugin interfaces + family implementations.
- `cli`: Cobra command tree and user-facing behavior.
- `config`: config precedence resolver (defaults/config/env/flags).
- `doctor`: doctor report model.
- `domain`: shared DTOs (diagnostics, desired state, plan).
- `engine`: façade layer used by CLI/TUI.
- `output`: table/json/yaml rendering + actionable error formatter.
- `planner`: diff engine + persisted plan file envelope.
- `registry`: plugin/backend registration and aliasing.
- `renderer`: safe artifact persistence.
- `schema`: explain catalog for `chainops explain`.
- `spec`: YAML loader/defaulting/node expansion.
- `state`: local snapshot + lock primitives.
- `tui`: legacy `bgorch tui` interaction model.
- `validate`: core semantic validation.
- `workspace`: built-in profiles and `init` starter spec generation.

## Command Pipeline Ownership

- `cli`: parse flags, resolve config, route command behavior.
- `engine` + `app`: orchestrate load/validate/build/plan/apply/status/doctor.
- plugins/backends: produce deterministic desired state.
- planner/state: converge desired vs snapshot with lock safety.

## Extension Points

1. Add plugin: implement `internal/chain.Plugin`, register in `internal/registry`.
2. Add backend: implement `internal/backend.Backend`; optionally runtime interfaces.
3. Add command UX: extend `internal/cli` and map to `internal/engine` operations.
4. Extend onboarding: add workspace profile/template in `internal/workspace`.

## Design Constraints

- chain semantics stay in plugins,
- runtime semantics stay in backends,
- planner/state remain deterministic and backend-agnostic,
- side effects are isolated to renderer/state/runtime-capable backends.
