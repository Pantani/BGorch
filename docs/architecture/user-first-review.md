# User-First Review (Current State)

## Summary

The project's declarative core was already technically solid for an MVP (plugins, backends, planner, snapshot, lock). The main gap was operational UX: commands were hard to discover, onboarding had no narrative, config precedence was implicit, and flag semantics were inconsistent across human and automation workflows.

## What Was Already Good

- Declarative core separated from plugins and backends (`internal/app`, `internal/chain`, `internal/backend`).
- Deterministic planner with hash-based diff and apply idempotency.
- Local lock per `(cluster, backend)` and versioned snapshot.
- Integration tests covering multiple backends and lock/runtime regressions.

## Technically Good, but Poor for Users

- Manual `flag`-based CLI was functional, but hard to discover and inconsistent.
- `render` was artifact-focused; canonical render of resolved config was missing.
- The `apply` safety flow was not explicit for non-interactive use.
- `help` did not tell a product story or guide users toward first success.

## Architectural Confusion

- UX and core layers were coupled in the old `internal/cli/root.go`.
- Config resolution was neither separated nor documented.
- Schema/plugin/profile explainability was indirect (external docs with no dedicated command).

## CLI Confusion

- Legacy naming (`bgorch`) and a short command tree optimized for authors, not operators.
- Missing `explain`, `diff`, `completion`, `plan --out`, and `apply <plan-file>`.
- Errors lacked a consistent cause/fix/next-command pattern.

## Too Much Abstraction Too Early

- Broad backend model without matching CLI affordances for operators.
- Policies (`backup/upgrade`) existed in the schema without equivalent inspection commands.

## Missing UX Affordances

- Guided onboarding (`init`) and profile presets.
- A simple, explicit main path for new users.
- Uniform structured output (`table|json|yaml`) across primary commands.
- An explain command for schema/plugins/profiles.

## Long, Fragile, and Opaque Flows

- First success depended on understanding spec internals.
- Automation depended on heterogeneous text parsing.
- Plan application had no explicit handoff artifact (`plan --out`).

## Decisions That Required an ADR

- Default confirmation behavior in `apply` (interactive and non-interactive).
- Official `render` semantics (canonical vs artifacts) and legacy compatibility.
- Role of `diff` as its own command vs a semantic alias of `plan`.
- Introduction of versioned `pkg/pluginapi` for future decoupling.

## Change Matrix

| Area | Current Problem | User Impact | Proposed Change | Priority |
|------|----------------|-------------------|------------------|-----------|
| Command model | Manual CLI, no product narrative | High learning curve | Migrate to Cobra with a user-first tree | P0 |
| Discovery | No `explain`, limited completion | Low discoverability | `explain` + `completion` + example-driven help | P0 |
| Operational safety | `apply` without strong non-interactive guardrails | Operational risk | Confirm by default and require `--yes` in non-interactive mode | P0 |
| Automation | Heterogeneous outputs | Fragile scripts | Stable `table/json/yaml` output | P0 |
| Configuration | Implicit precedence | Surprise behavior | Explicit `defaults < file < env < flags` plus docs | P0 |
| Change handoff | No `plan --out` / `apply <plan>` | Lower auditability | Versioned plan file | P1 |
| Onboarding | No clear preset | High time-to-first-success | Interactive + non-interactive `init` plus profiles | P0 |
| UX/Core architecture | CLI and core coupled | Harder evolution | New layers: `config/engine/output/schema/workspace` | P1 |
| Errors | Inconsistent messages | Slower troubleshooting | Error style guide + standardized actionable error | P1 |
| Migration | Product rename without a plan | Workflow breakage | `bgorch` as legacy alias + migration guide | P1 |
