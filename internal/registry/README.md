# `internal/registry`

In-memory registries for plugins and backends.

## Responsibility

- register/get/list plugin and backend implementations,
- enforce unique names,
- provide default built-in registrations with aliases.

## Entrypoints

- `New`, `NewDefault`
- `(*PluginRegistry).Register/Get/Names`
- `(*BackendRegistry).Register/Get/Names`
- `(*Registries).MustRegisterPlugin/MustRegisterBackend`

## Caveats

- alias behavior (for example `compose`, `sshsystemd`) is implemented here,
- registration order affects discoverability output but not runtime determinism.
