# `internal/state`

Local snapshot persistence and apply lock primitives.

## Snapshot Model

`Snapshot` stores hashes for:

- desired services (`hash(json(service))`)
- desired artifacts (`hash(content)`)

Planner consumes this snapshot to compute `create/update/delete/noop` diff.

## Lock Model

- lock key: `(cluster, backend)`
- lock file: `<state-dir>/<cluster>--<backend>.lock`
- acquisition: atomic (`O_CREATE|O_EXCL`)
- release: idempotent (`sync.Once`)

## Default State Dirs

- `chainops`: `.chainops/state`
- legacy `bgorch`: `.bgorch/state`

## Scope

This protects concurrent applies on one machine.
It does not provide distributed lock/state coordination.
