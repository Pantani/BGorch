# ADR 0008: User-First CLI Redesign

- Status: Accepted
- Date: 2026-04-16

## Context

A base técnica declarativa estava madura para MVP, porém a UX de operação e onboarding ainda tinha alta fricção.

## Decisões

1. **`apply` exige confirmação interativa por padrão?**
   - Sim, no `chainops`.
2. **No modo não interativo, exigir `--yes`?**
   - Sim, para evitar mutação silenciosa.
3. **`plan` e `render` side-effect free?**
   - Sim, por contrato de UX e automação.
4. **`diff` comando próprio ou parte de `plan`?**
   - Comando próprio como visão focada de `plan` (sem noops), reaproveitando mesmo motor.
5. **`init`: wizard, template generator, ou ambos?**
   - Ambos: interativo opcional + não interativo excelente para CI.
6. **Representar plugins/profiles/backends sem poluir core?**
   - Sim: CLI consulta registries/perfis; core permanece em `internal/app`/`engine`.
7. **UX para “qualquer blockchain” sem abstração vaga?**
   - Schema tipado + explain + plugin capabilities explícitas; evitar `map[string]any` no core.
8. **Caminho básico simples e avançado poderoso?**
   - Fluxo principal curto + comandos avançados (`plan --out`, `apply <plan-file>`, outputs estruturados).

## Consequências

- Menor risco operacional em CI.
- Melhora de discoverability e onboarding.
- Compatibilidade incremental via alias `bgorch`.
- Base pronta para evoluir plugin SDK versionável (`pkg/pluginapi`).
