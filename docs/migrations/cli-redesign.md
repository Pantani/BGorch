# CLI Redesign Migration

## Context

The CLI evolved from `bgorch` (technical MVP) to `chainops` (user-first UX).

Goal:

- reduce cognitive load
- increase operational safety
- improve automation and explainability

## Compatibility

- `cmd/bgorch` remains available as a legacy alias.
- Old flows continue to work with compatibility for critical flags (`-o` as output-dir in legacy commands).

## Main Changes

1. New root command: `chainops`
2. New recommended flow: `init -> doctor -> render -> plan -> apply`
3. `render` now prioritizes resolved canonical configuration
4. `plan --out` and `apply <plan-file>`
5. `apply` with safety gating (`--yes` in non-interactive mode)
6. `explain`, `diff`, `plugin/profile/context`, `completion`

## Command Mapping

| Legacy (`bgorch`) | New (`chainops`) | Notes |
|-------------------|-------------------|------------|
| `validate` | `validate` | Kept |
| `render -o <dir>` | `render --write-artifacts --artifacts-dir <dir>` | `-o <dir>` is still accepted for compatibility |
| `plan` | `plan` | `--out` is new |
| `apply -f spec` | `apply -f spec --yes` | `--yes` required in non-interactive mode |
| `status` | `status` | `--output` table/json/yaml |
| `doctor` | `doctor` | actionable checks |
| `tui/ui` | `bgorch tui/ui` | legacy |

## Pipeline Recommendations

Before:

```bash
bgorch plan -f cluster.yaml
bgorch apply -f cluster.yaml
```

After:

```bash
chainops plan -f chainops.yaml --out plan.json --output json
chainops apply plan.json --yes --output json
```

## Planned Deprecations

- `bgorch` will be kept during the transition phase.
- New functionality will be added primarily under the `chainops` namespace.
