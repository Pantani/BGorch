# Backend Expansion Architecture (Kubernetes, Terraform, Ansible)

Status date: **2026-04-16**.

## Objetivo

Definir o desenho de expansão de backend mantendo o core declarativo em Go como control plane principal.

Este documento descreve fronteiras e fluxo-alvo e já reflete a implementação MVP artifact-first de `kubernetes`, `terraform` e `ansible`.

## Estado verificado hoje

Implementado:

- runtime/artifact backends: `docker-compose`, `ssh-systemd`, `kubernetes`, `terraform`, `ansible`;
- contratos opcionais: `RuntimeExecutor`, `RuntimeObserver`;
- comandos core: `validate`, `render`, `plan`, `apply`, `status`, `doctor`;
- lock/snapshot local no core.

Não implementado:

- runtime exec/observe para `kubernetes`, `terraform` e `ansible`;
- lock distribuído/reconciler contínuo.

## Modelo de responsabilidades

| Camada | Dono | Responsabilidade | Não deve fazer |
|---|---|---|---|
| Core (`internal/app`, planner/state) | BGorch core | validação final, plan, lock, apply orchestration, status/doctor | lógica específica de chain ou runtime interno de ferramenta externa |
| Plugin de chain | `internal/chain/*` | validação/normalização de família, render específico de chain | provisionamento de infra ou gestão de runtime backend |
| Runtime backend | `docker-compose`, `ssh-systemd`, `kubernetes` | traduzir desired state para runtime e observar execução | virar semântica de protocolo/chain |
| Infra adapter | `terraform` | provisionar recursos base (rede, VM, disco, cluster) | substituir reconciler de processos |
| Host config adapter | `ansible` | bootstrap/config de host, distribuição de arquivos, handlers | substituir control plane declarativo |

## Fronteiras por backend alvo

### Kubernetes (runtime backend)

Escopo:

- traduzir workloads para recursos Kubernetes adequados a stateful;
- suportar observação de execução no cluster.

Fora de escopo:

- provisionar cluster;
- assumir lógica de chain no backend.

### Terraform (infra adapter)

Escopo:

- criar/atualizar recursos de infraestrutura;
- devolver outputs consumíveis por runtime/config.

Fora de escopo:

- operação de processos blockchain;
- gerenciamento de lifecycle de serviço no host/container.

### Ansible (host config adapter)

Escopo:

- bootstrap de host, diretórios, templates, units e handlers;
- aplicação idempotente de configuração de host.

Fora de escopo:

- planner/reconciler central;
- estado autoritativo global da plataforma.

## Fluxo de integração proposto

```text
Spec -> Validate -> Plugin Build -> Backend BuildDesired -> Plan
  -> Apply (lock)
     -> Render artifacts
     -> Optional runtime execution (backend capability)
     -> Optional infra/config adapter run (explicit stage)
     -> Snapshot update
  -> Status/Doctor
     -> local snapshot analysis
     -> optional runtime/adapter observation
```

## Estratégia de rollout (estado atual e próximos passos)

1. **Kubernetes backend (estado atual)**:
   - tradução determinística de desired state implementada;
   - próximo passo: observação/execução runtime opcional no fluxo `status/doctor/apply`.
2. **Terraform adapter (estado atual)**:
   - scaffold determinístico de artefatos implementado;
   - próximo passo: estágios explícitos de plan/apply e import de outputs.
3. **Ansible adapter (estado atual)**:
   - render determinístico de inventory/group_vars/playbook implementado;
   - próximo passo: execução controlada e coleta estruturada de resultados.

## Riscos operacionais

1. Divergência entre estado de infra e estado de runtime.
2. Aumento de tempo de convergência com múltiplos estágios.
3. Falhas parciais com rollback complexo entre ferramentas.
4. Dificuldade de troubleshooting sem correlação de eventos entre camadas.

## Riscos de segurança

1. Exposição de credenciais (cloud tokens, SSH keys, kubeconfig).
2. Execução remota com privilégios excessivos.
3. Vazamento de segredo em logs/erro de subprocesso.
4. Dependência em ferramentas externas sem hardening uniforme.

Controles recomendados:

- mascaramento de segredo em output de comando;
- princípio de menor privilégio para credenciais;
- validação de contexto/target antes de mutação;
- trilha de auditoria mínima por comando/target.

## Limites do MVP e pendências

Limites atuais:

- estado/lock local (sem coordenação distribuída);
- sem reconciler contínuo em background;
- capabilities variam entre backends.

Pendências abertas:

1. Contrato explícito para estágios de infra/config (`terraform`/`ansible`).
2. Política de rollback/repair cross-backend.
3. Modelo unificado de observação (snapshot local vs estado remoto).
4. Estratégia de secret stores externos (Vault/KMS/SOPS) integrada ao fluxo.
