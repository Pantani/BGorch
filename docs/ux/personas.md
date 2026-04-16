# Personas

## 1. Desenvolvedor Local

Objetivo:

- subir nó/stack rápido
- iterar e destruir com baixo custo

Fluxo esperado:

- `init` com profile local-dev
- `render` canônico para sanity check
- `apply --yes` e depois `destroy`

## 2. Operador / DevOps / SRE

Objetivo:

- previsibilidade operacional e automação robusta

Fluxo esperado:

- `doctor`
- `plan --out`
- revisão/aprovação
- `apply <plan-file> --yes`
- `status`/`logs`

## 3. Operador de Blockchain

Objetivo:

- escolher família/profile/backend sem aprender internals do engine

Fluxo esperado:

- `profile list`
- `init --profile ...`
- `explain` para campos críticos
- `plan`/`apply`

## 4. Autor de Plugin / Integrador

Objetivo:

- ampliar suporte de famílias sem quebrar core

Fluxo esperado:

- contrato estável via `pkg/pluginapi`
- validação clara de plugin/backend
- documentação e ADRs para evolução compatível
