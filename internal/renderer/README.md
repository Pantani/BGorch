# `internal/renderer`

Artifact persistence utilities for desired state outputs.

## Responsibility

- write rendered artifacts to disk deterministically;
- enforce output-path safety constraints to prevent directory traversal;
- isolate filesystem write behavior from planner/app orchestration logic.

## Entrypoint

- `WriteArtifacts(baseDir, artifacts)`

## Safety Rules

- absolute artifact paths are rejected;
- empty/current-directory paths are rejected;
- paths escaping `baseDir` via `..` are rejected.

## Why this boundary exists

Keeping path validation and file IO here avoids duplicating security-sensitive checks across commands/backends.
