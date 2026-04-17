# `internal/config`

Resolves CLI runtime configuration with explicit precedence.

## Responsibility

- define defaults for `chainops` and legacy `bgorch` modes,
- bind persistent CLI flags,
- load optional config file,
- merge config file/env/flags deterministically.

## Precedence

```text
defaults < config file < env vars < flags
```

## Entrypoints

- `DefaultValues(legacy bool)`
- `New(legacy bool)`
- `(*Config).BindRootFlags(...)`
- `(*Config).Resolve()`
- `(*Config).OutputFromConfig()`

## Caveats

- environment keys use prefix `CHAINOPS_` with `-` converted to `_`,
- resolver is CLI-runtime config only (not cluster desired-state config).
