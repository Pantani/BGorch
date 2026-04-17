# `internal/app`

`app.App` is the core application service orchestrating command pipelines.

## Responsibility

- run canonical pipelines for `validate`, `render`, `plan`, `apply`, `status`, `doctor`;
- centralize plugin/backend resolution and diagnostics merge;
- coordinate renderer, planner, state lock/snapshot, and runtime hooks;
- enforce safety semantics (`dry-run`, `runtime-exec`, `observe-runtime`, `require-runtime`).

## Key Entrypoints

- `New(opts Options) *App`
- `LoadSpec(path)`
- `ValidateSpec(path)`
- `Render(ctx, specPath, outputDir, writeState)`
- `Plan(ctx, specPath)`
- `Apply(ctx, specPath, opts)`
- `Status(ctx, specPath, opts)`
- `Doctor(ctx, specPath, opts)`

## Data Flow

1. load + default spec
2. resolve plugin/backend
3. run validations
4. plugin normalize/build
5. backend build desired
6. planner/state/runtime handling per command

## Operational Semantics

- diagnostics are returned separately from hard errors,
- `apply` always acquires lock before mutable actions,
- snapshot persists only after successful apply pipeline,
- runtime capability mismatches surface as typed unsupported errors,
- `status`/`doctor` can run in graceful mode or strict runtime mode (`RequireRuntime`).
