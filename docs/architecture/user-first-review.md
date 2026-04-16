# User-First Review (Estado Atual)

## Resumo

O core declarativo do projeto já estava tecnicamente sólido para MVP (plugins, backends, planner, snapshot, lock). O principal gap era UX operacional: comandos pouco descobríveis, sem narrativa de onboarding, sem precedência de config explícita e com semântica de flags inconsistente para humanos e automação.

## O Que Já Estava Bom

- Core declarativo separado de plugins e backends (`internal/app`, `internal/chain`, `internal/backend`).
- Planner determinístico com diff por hash e idempotência de apply.
- Lock local por `(cluster, backend)` e snapshot versionado.
- Testes de integração cobrindo múltiplos backends e regressões de lock/runtime.

## Tecnicamente Bom, Mas Ruim Para o Usuário

- CLI manual com `flag` era funcional, porém pouco descobrível e inconsistente.
- `render` era focado em artifacts; faltava render canônico da configuração resolvida.
- Fluxo de segurança no `apply` não era explícito para modo não interativo.
- `help` não contava uma história de produto nem guiava o primeiro sucesso.

## Confusões de Arquitetura

- Camada de UX e core estavam acopladas em `internal/cli/root.go` antigo.
- Resolução de config não estava separada nem documentada.
- Explainability de schema/plugins/perfis era indireta (docs externas sem comando dedicado).

## Confusões de CLI

- Naming legado (`bgorch`) e árvore curta para autores, não para operadores.
- Ausência de `explain`, `diff`, `completion`, `plan --out`, `apply <plan-file>`.
- Erros sem padrão consistente de causa/correção/próximo comando.

## Excesso de Abstração Cedo Demais

- Modelo de backend amplo sem affordances equivalentes na CLI para operadores.
- Políticas (`backup/upgrade`) no schema sem comandos de inspeção equivalentes.

## Affordances de UX Ausentes

- Onboarding guiado (`init`) e presets de perfil.
- Caminho principal simples e explícito para novos usuários.
- Saída estruturada uniforme (`table|json|yaml`) entre comandos principais.
- Explain command para schema/plugins/perfis.

## Fluxos Longos/Frágeis/Obscuros

- Primeiro sucesso dependia de conhecer internals do spec.
- Automação dependia de parsing textual heterogêneo.
- Aplicação de plano não tinha artefato explícito de handoff (`plan --out`).

## Decisões que Exigem ADR

- Confirmação padrão no `apply` (interativo e não interativo).
- Semântica oficial de `render` (canônico vs artifacts) e compat legada.
- Papel de `diff` como comando próprio vs alias semântico de `plan`.
- Introdução de `pkg/pluginapi` versionável para desacoplamento futuro.

## Matriz de Mudanças

| Área | Problema atual | Impacto no usuário | Mudança proposta | Prioridade |
|------|----------------|-------------------|------------------|-----------|
| Modelo de comandos | CLI manual, sem narrativa de produto | Curva de aprendizado alta | Migrar para Cobra com árvore user-first | P0 |
| Descoberta | Sem `explain`, completion limitado | Baixa discoverability | `explain` + `completion` + help com exemplos | P0 |
| Segurança operacional | `apply` sem gate forte em modo não interativo | Risco operacional | Confirmar por padrão e exigir `--yes` no não interativo | P0 |
| Automação | Saídas heterogêneas | Scripts frágeis | Saída estável `table/json/yaml` | P0 |
| Configuração | Precedência implícita | Comportamento surpresa | `defaults < file < env < flags` explícito + docs | P0 |
| Handoff de mudança | Sem `plan --out` / `apply <plan>` | Menor auditabilidade | Arquivo de plano versionado | P1 |
| Onboarding | Sem preset claro | Tempo até primeiro sucesso alto | `init` interativo + não interativo + profiles | P0 |
| Arquitetura UX/Core | CLI e core acoplados | Evolução difícil | Novas camadas: `config/engine/output/schema/workspace` | P1 |
| Erros | Mensagens inconsistentes | Troubleshooting lento | Error style guide + erro acionável padronizado | P1 |
| Migração | Renomeação de produto sem plano | Quebra de fluxo | `bgorch` como alias legado + guia de migração | P1 |
