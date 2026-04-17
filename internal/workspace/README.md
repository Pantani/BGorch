# `internal/workspace`

Onboarding helpers for `chainops init`.

## Responsibility

- define built-in starter profiles,
- resolve profile metadata (`list/show` flows),
- generate starter specs with family/plugin/backend defaults.

## Entrypoints

- `Profiles()`
- `GetProfile(name)`
- `BuildSpec(req InitRequest)`

## Caveats

- init templates are intentionally opinionated bootstrap artifacts,
- unsupported backend/profile combinations fail fast with actionable errors.
