# `pkg/pluginapi`

Versioned public contract for plugin ecosystem evolution.

## Responsibility

- define minimal stable payloads for plugin diagnostics/artifacts/capabilities,
- decouple future external plugin SDK from internal package layout,
- provide compatibility marker (`Version`) for plugin API negotiations.

## Current Scope

- data model types only (no plugin runtime loading framework yet),
- built-in plugins still live in `internal/chain/*`.

## Why this package exists

It allows moving from internal-only built-ins to external/plugin marketplace models without forcing consumers to import unstable internal paths.
