# CLI Principles

## 1. Least Surprise

- The main command is always predictable.
- `render` shows canonical config by default.
- `apply` never runs silently in non-interactive mode without `--yes`.

## 2. Progressive Disclosure

- Short basic path: `init -> doctor -> render -> plan -> apply`.
- Advanced features live in flags/subcommands (`plan --out`, `apply <plan-file>`, `explain`).

## 3. Safe by Default

- `plan`/`render` side-effect free.
- `apply` uses explicit confirmation.
- `destroy` is explicit and local-only in the current implementation.

## 4. Explainability

- `explain` covers schema/plugins/profiles.
- `doctor` returns checks plus actionable hints.
- Error messages follow the title/cause/fix/next-command format.

## 5. Discoverability

- `--help` includes practical examples.
- `completion` is available for major shells.
- `profile list/show` and `plugin list` support guided discovery.

## 6. Automation Friendly

- Structured output for main commands: `table|json|yaml`.
- Explicit handoff via `plan --out`.
- Config precedence is explicit and observable (`context show`).
