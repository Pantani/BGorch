# ADR 0007: Expansão de Backends (Kubernetes + adapters Terraform/Ansible)

- Status: Accepted
- Date: 2026-04-16

## Contexto

O código atual já possui:

- contrato mínimo de backend (`ValidateTarget`, `BuildDesired`);
- contratos opcionais de runtime (`RuntimeExecutor`, `RuntimeObserver`);
- backends implementados: `docker-compose`, `ssh-systemd`, `kubernetes`, `terraform`, `ansible`.

Na fase atual, a expansão foi iniciada com:

1. `kubernetes` como backend de runtime em modo artifact-first;
2. `terraform` como adapter de provisionamento em modo artifact-first;
3. `ansible` como adapter de bootstrap/configuração em modo artifact-first.

Sem essas fronteiras, há risco de:

- acoplar o core a ferramentas específicas;
- empurrar orchestration de processos para Terraform/Ansible;
- aumentar blast radius de `apply` com responsabilidades misturadas.

## Decisão

Adotar modelo explícito de três classes de backend, com responsabilidades separadas:

1. **Runtime backend** (`docker-compose`, `ssh-systemd`, futuro `kubernetes`)
2. **Infra adapter** (`terraform`)
3. **Host config adapter** (`ansible`)

### Fronteiras obrigatórias

- `kubernetes`:
  - responsável por materializar e observar workloads/state no cluster;
  - não assume semântica de chain/protocolo.
- `terraform`:
  - responsável por provisionar infraestrutura (VM, rede, disco, cluster);
  - não executa reconciliação de processos de node blockchain.
- `ansible`:
  - responsável por bootstrap/configuração de host (dirs, units, arquivos, handlers);
  - não substitui planner/reconciler do core.

### Semântica de integração

- `plan` continua obrigatório no core antes de mutações.
- `apply` do core mantém lock/snapshot local como mecanismo mínimo de segurança.
- execução runtime permanece opt-in por capability do backend.
- adapters de infra/config devem expor resultados observáveis ao `status/doctor` sem assumir ownership do estado do core.

## Racional

- Preserva separação de concerns (core vs runtime vs infra vs bootstrap).
- Permite evolução incremental sem reescrever contratos atuais.
- Mantém idempotência e diagnósticos no control plane principal (Go core).

## Consequências

### Positivas

- crescimento para Kubernetes/Terraform/Ansible com baixo acoplamento;
- compatibilidade futura com múltiplos ambientes operacionais;
- menor risco de lock-in arquitetural.

### Negativas

- diferença de capability entre backends aumenta complexidade de UX;
- maior esforço de testes de integração cruzada (backend x plugin x runtime);
- necessidade de documentação forte de limites por backend.

## Riscos operacionais e de segurança

1. **Drift multi-camada**: infra provisionada, mas runtime divergente.
2. **Credenciais sensíveis**: tokens cloud, chaves SSH, kubeconfig, secrets de chain.
3. **Comandos destrutivos**: `terraform apply/destroy` e handlers remotos amplos.
4. **Escalada de privilégio**: `ansible become`, acesso cluster-admin em Kubernetes.
5. **Diagnóstico inconsistente**: `status/doctor` sem distinguir snapshot local e estado remoto observado.

Mitigações mínimas propostas:

- execução explícita por flags/capabilities (sem efeitos implícitos);
- mascaramento de segredos em logs e erros;
- validação prévia de target/contexto antes de mutar;
- observabilidade padronizada por backend (resumo + detalhes);
- gates de verificação (`verify`) antes de integração.

## Limites do MVP

Estado atual da implementação:

- `kubernetes` implementa `BuildDesired` e validação mínima (sem runtime exec/observe);
- `terraform` implementa `BuildDesired` e validação mínima (sem runtime exec/observe);
- `ansible` implementa `BuildDesired` e validação mínima (sem runtime exec/observe);
- lock distribuído e reconciler contínuo seguem fora de escopo.

## Próximos passos (propostos)

1. Definir pacote `internal/backend/kubernetes` com build/render e observe básico.
2. Definir contrato de adapter de infraestrutura para `terraform` (plan/apply/output import).
3. Definir contrato de adapter de host config para `ansible` (inventory/playbook/result mapping).
4. Adicionar matriz de testes de integração por capability (runtime/infra/config).
5. Evoluir `status/doctor` para separar explicitamente:
   - estado local (snapshot),
   - estado observado em runtime/adapters.
