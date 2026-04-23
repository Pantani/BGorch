# Error Style Guide

## Goal

Messages should answer, in this order:

1. What failed
2. Probable cause
3. How to fix it
4. Recommended next command

## Template

```text
<Clear title>
Cause: <probable cause>
Hint: <how to fix it>
Next: <recommended command>
```

## Rules

- Avoid generic messages (`invalid config`, `failed`).
- Include the field/path when applicable.
- Prioritize concrete action, not theory.
- Do not hide the impact of conflicting flags.

## Examples

### Bad

```text
invalid config
```

### Good

```text
Invalid CLI configuration.
Cause: read config file "chainops-cli.yaml": no such file or directory
Hint: Check --config path, environment variables, and flag values.
Next: chainops context show
```

### Bad

```text
unsupported output
```

### Good

```text
Unsupported output format.
Cause: "xml" is not supported.
Hint: Use table, json, or yaml.
Next: chainops plan -f chainops.yaml --output json
```

## Critical Situations

- Destructive operations must explicitly mention confirmation/`--yes`.
- Preflight errors should always suggest `doctor` when applicable.

## Code Conventions

- Use `internal/output.ActionableError(...)` for UX errors.
- Preserve complete technical messages in internal logs/stacks when necessary.
