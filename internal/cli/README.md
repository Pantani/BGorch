# `internal/cli`

Command tree and user-facing behavior for `chainops` (and legacy `bgorch` alias).

## Responsibility

- define command structure, help text, and flags;
- resolve CLI runtime config (`defaults/config/env/flags`);
- map command intents to `internal/engine`/`internal/app` pipelines;
- format diagnostics/results for table/json/yaml output;
- enforce operator safety semantics (`--yes`, interactive confirmation, runtime gates).

## Key Entrypoints

- `Run(args)` / `RunProgram(program, args)`
- `NewRootCommand(program)`
- command builders (`newRenderCmd`, `newPlanCmd`, `newApplyCmd`, `newStatusCmd`, `newDoctorCmd`, ...).

## Important Flows

1. pre-run resolves effective config and state/artifact directories.
2. command handler resolves spec/plan input.
3. engine call executes deterministic pipeline.
4. CLI renders output and maps failures to actionable errors.

## Caveats

- `-o` remains overloaded for legacy compatibility in some commands:
  - output format (`table/json/yaml`) in current UX;
  - output directory path fallback for legacy flows.
- runtime flags are backend-capability gated:
  - `--runtime-exec`, `--observe-runtime`,
  - strict mode via `--require-runtime`.
