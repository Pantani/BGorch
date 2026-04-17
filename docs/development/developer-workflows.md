# Developer Workflows

This document is implementation-focused and aligned with current repository behavior.

## Local Setup

### Prerequisites

- Go `1.22+`
- Optional: Docker + Compose plugin for compose runtime flags
- Optional: SSH connectivity + `systemctl` on targets for ssh-systemd runtime flags
- Optional: `kubectl` for kubernetes runtime observation

### Bootstrap

```bash
go mod download
```

## Common Development Loops

### Validate + render + plan loop

```bash
chainops validate -f examples/generic-single-compose.yaml
chainops render   -f examples/generic-single-compose.yaml -o yaml
chainops plan     -f examples/generic-single-compose.yaml
```

### Apply loop

```bash
# Dry-run first
chainops apply -f examples/generic-single-compose.yaml --dry-run

# Real apply (requires explicit confirmation in non-interactive mode)
chainops apply -f examples/generic-single-compose.yaml --yes
```

### Runtime compose loop (optional)

```bash
chainops apply  -f examples/generic-single-compose.yaml --runtime-exec --yes
chainops status -f examples/generic-single-compose.yaml --observe-runtime
chainops doctor -f examples/generic-single-compose.yaml --observe-runtime
```

### Runtime SSH/systemd loop (optional)

```bash
chainops apply  -f examples/generic-single-ssh-systemd.yaml --runtime-exec --yes
chainops status -f examples/generic-single-ssh-systemd.yaml --observe-runtime
chainops doctor -f examples/generic-single-ssh-systemd.yaml --observe-runtime
```

### Runtime kubernetes observation loop (optional)

```bash
chainops status -f examples/generic-single-kubernetes.yaml --observe-runtime
chainops doctor -f examples/generic-single-kubernetes.yaml --observe-runtime
```

## Running Specific Subsystems

### Plugin behavior only

```bash
go test ./internal/chain/... -v
```

### Backend rendering/observation only

```bash
go test ./internal/backend/... -v
```

### Planner/state semantics only

```bash
go test ./internal/planner ./internal/state -v
```

### Config precedence only

```bash
go test ./internal/config -v
```

### CLI end-to-end smoke/regression

```bash
go test ./test/integration ./test/regression -v
```

## Debugging Guide

### Snapshot and lock inspection

- state directory (default): `.chainops/state`
- snapshot files: `<cluster>--<backend>.json`
- lock files: `<cluster>--<backend>.lock`

Inspect with:

```bash
ls -la .chainops/state
cat .chainops/state/<cluster>--<backend>.json
```

### Rendered artifact inspection

```bash
find .chainops/render -type f | sort
```

### Runtime failures

When runtime flags fail (`--runtime-exec` / `--observe-runtime` / `--require-runtime`):

- confirm backend actually implements the required capability,
- confirm rendered artifacts exist,
- confirm runtime binaries are available (`docker`, `ssh`, `kubectl`),
- rerun without runtime flags to isolate local-state/core pipeline behavior.

## Testing Workflow

### Fast local checks

```bash
go test ./...
```

### Full verification pipeline

```bash
make verify
```

`make verify` runs:

1. gofmt check
2. build
3. tests
4. vet
5. race tests when platform/CGO support is available

## CI Expectations

Repository scripts expect deterministic outputs:

- sorted artifacts/services for stable tests,
- stable golden outputs for renderers and help text,
- no unformatted Go files,
- no side effects in `plan` and canonical `render` mode.

## Migrations / Seed Data

There are no database migrations or seed workflows in the current implementation.

## Release Process (Current)

No automated release pipeline is implemented in this repository.

Current release practice is manual:

1. run `make verify`,
2. build binaries from `cmd/chainops` (and optionally alias `cmd/bgorch`),
3. publish artifacts/changelog through external workflow.
