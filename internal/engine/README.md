# `internal/engine`

Facade layer consumed by CLI/TUI.

## Responsibility

- expose stable service methods wrapping `internal/app`,
- hide direct package graph complexity from command handlers,
- provide utility operations around snapshots/artifact directories/registry listing.

## Entrypoints

- `New(opts Options)`
- `Validate`, `LoadSpec`, `RenderArtifacts`, `Plan`, `Apply`, `Status`, `Doctor`
- `DeleteStateSnapshot`, `ResolveSnapshotPath`, `StateDir`, `RemoveArtifactsDir`
- `PluginNames`, `BackendNames`, `Registries`

## Why it exists

Keeps CLI focused on UX and flag semantics, while core orchestration remains concentrated in `internal/app`.
