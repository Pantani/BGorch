# ADR 0008: User-First CLI Redesign

- Status: Accepted
- Date: 2026-04-16

## Context

The declarative technical foundation was mature enough for an MVP, but the operational and onboarding UX still had high friction.

## Decisions

1. **Should `apply` require interactive confirmation by default?**
   - Yes, in `chainops`.
2. **In non-interactive mode, should `--yes` be required?**
   - Yes, to avoid silent mutation.
3. **Should `plan` and `render` be side-effect free?**
   - Yes, by UX and automation contract.
4. **Should `diff` be its own command or part of `plan`?**
   - Separate command as a focused view of `plan` (without noops), reusing the same engine.
5. **Should `init` be a wizard, a template generator, or both?**
   - Both: optional interactive mode plus strong non-interactive behavior for CI.
6. **How should plugins/profiles/backends be represented without polluting the core?**
   - The CLI queries registries/profiles; the core remains in `internal/app`/`engine`.
7. **How do we support "any blockchain" without vague abstraction?**
   - Typed schema + explain + explicit plugin capabilities; avoid `map[string]any` in the core.
8. **How do we keep the basic path simple while preserving advanced power?**
   - Short main flow plus advanced commands (`plan --out`, `apply <plan-file>`, structured outputs).

## Consequences

- Lower operational risk in CI.
- Better discoverability and onboarding.
- Incremental compatibility through the `bgorch` alias.
- Foundation ready for a versioned plugin SDK evolution (`pkg/pluginapi`).
