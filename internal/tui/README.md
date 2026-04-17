# `internal/tui`

Legacy interactive terminal UI exposed by `bgorch tui`.

## Responsibility

- provide guided local workflows for core actions,
- render plan/doctor/diagnostics views,
- route actions through the same `internal/app` pipelines.

## Scope

- available via legacy alias path,
- `chainops` UX is CLI-first; TUI is compatibility/developer aid.
