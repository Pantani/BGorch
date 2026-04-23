# Target Architecture (User-First)

## Goal

Keep the declarative core/reconciler while reducing UX coupling, with explicit boundaries for CLI, automation, and plugin/backend evolution.

## Target Structure

```text
cmd/chainops/
cmd/bgorch/                    # legacy alias
internal/cli/
internal/config/
internal/schema/
internal/planner/
internal/renderer/
internal/engine/
internal/state/
internal/output/
internal/doctor/
internal/workspace/
pkg/pluginapi/
plugins/
backends/
profiles/
examples/
docs/
```

## Responsibilities

- `internal/cli`: command model, help/UX, error messages, interaction.
- `internal/config`: defaults/config/env/flag resolution and precedence.
- `internal/engine`: stable facade for core pipelines.
- `internal/schema`: explainability catalog for the schema.
- `internal/output`: uniform serialization and rendering (`table/json/yaml`).
- `internal/workspace`: onboarding and starter spec/profile generation.
- `internal/app`: declarative orchestration of validate/render/plan/apply/status/doctor.
- `internal/planner`: deterministic diff between desired state and snapshot.
- `internal/state`: snapshot + lock.
- `pkg/pluginapi`: minimum versioned contract for plugin SDK evolution.

## Architectural Rules

1. The CLI does not contain reconciliation logic.
2. The core does not depend on a specific blockchain.
3. Planner and executor remain separate.
4. Canonical render and plan are side-effect free.
5. Apply concentrates mutations and safety gating.
6. The plugin API must be small, typed, and versionable.

## Flows

### Human Flow (Default)

```text
init -> doctor -> render -> plan -> apply -> status
```

### CI/CD Flow

```text
validate -> render -o yaml/json -> plan --out -> apply <plan-file> --yes
```

## Key Decisions

- `apply` confirms by default in `chainops`.
- In non-interactive mode, `--yes` is required for destructive apply.
- `diff` exists as a focused view of `plan` (without noops).
- `render` prioritizes canonical config while preserving legacy artifact mode.

## Future Evolution

- Extract built-in plugins/backends into root-level `plugins/` and `backends/` directories as external modules.
- Expand `pkg/pluginapi` to cover the full lifecycle (backup/restore/upgrade/logs).
- Add distributed observed state, not only local snapshot state.
