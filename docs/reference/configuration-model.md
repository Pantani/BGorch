# Configuration Model

## 1. Desired State Spec (`ChainCluster`)

Primary operations file (for example, `chainops.yaml`) using the `bgorch.io/v1alpha1` schema.

This file defines:

- family and plugin (`spec.family`, `spec.plugin`)
- backend/runtime (`spec.runtime`)
- node/workload topology (`spec.nodePools`)
- policies (`backup`, `upgrade`, `observe`)
- typed extensions (`pluginConfig`, `backendConfig`)

## 2. CLI Runtime Config

Configuration for CLI behavior (not the cluster), via:

- `--config` (file)
- `CHAINOPS_*` (env)
- flags

Examples: `state-dir`, `artifacts-dir`, `output`, `yes`, `non-interactive`.

## 3. Canonical Render

`chainops render -f chainops.yaml -o yaml` shows the fully resolved configuration with defaults applied.

Guarantees:

- side-effect free by default
- deterministic for the same input
- useful for review/audit before `plan`/`apply`

## 4. Plan File (Handoff)

`chainops plan --out plan.json` generates a versioned plan file.

`chainops apply plan.json --yes` applies the plan using the plan's `sourceSpec`.

Recommended use:

- human plan review
- pipeline approval
- later apply with an audit trail

## 5. Safety Defaults

- `apply` requires confirmation in interactive mode.
- In non-interactive mode, it requires `--yes`.
- `plan` and `render` must remain side-effect free.
