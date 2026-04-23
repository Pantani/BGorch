# Personas

## 1. Local Developer

Goal:

- bring up a node/stack quickly
- iterate and tear it down at low cost

Expected flow:

- `init` with the `local-dev` profile
- canonical `render` for sanity checking
- `apply --yes` e depois `destroy`

## 2. Operator / DevOps / SRE

Goal:

- operational predictability and robust automation

Expected flow:

- `doctor`
- `plan --out`
- review/approval
- `apply <plan-file> --yes`
- `status`/`logs`

## 3. Blockchain Operator

Goal:

- choose a family/profile/backend without learning engine internals

Expected flow:

- `profile list`
- `init --profile ...`
- `explain` for critical fields
- `plan`/`apply`

## 4. Plugin Author / Integrator

Goal:

- expand family support without breaking the core

Expected flow:

- stable contract via `pkg/pluginapi`
- clear plugin/backend validation
- documentation and ADRs for compatible evolution
