# Backend Expansion Architecture (Kubernetes, Terraform, Ansible)

Status date: **2026-04-16**.

## Goal

Define the backend expansion design while keeping the Go declarative core as the primary control plane.

This document describes boundaries and the target flow, and already reflects the artifact-first MVP implementation of `kubernetes`, `terraform`, and `ansible`.

## State Verified Today

Implemented:

- runtime/artifact backends: `docker-compose`, `ssh-systemd`, `kubernetes`, `terraform`, `ansible`;
- optional contracts: `RuntimeExecutor`, `RuntimeObserver`;
- core commands: `validate`, `render`, `plan`, `apply`, `status`, `doctor`;
- local lock/snapshot in the core.

Not implemented:

- runtime exec for `kubernetes`, `terraform`, and `ansible`;
- runtime observe for `terraform` and `ansible`;
- distributed lock / continuous reconciler.

## Responsibility Model

| Layer | Owner | Responsibility | Must Not Do |
|---|---|---|---|
| Core (`internal/app`, planner/state) | Chainops core | final validation, plan, lock, apply orchestration, status/doctor | chain-specific logic or internal runtime logic of external tools |
| Chain plugin | `internal/chain/*` | family validation/normalization, chain-specific rendering | infrastructure provisioning or backend runtime management |
| Runtime backend | `docker-compose`, `ssh-systemd`, `kubernetes` | translate desired state into runtime resources and observe execution | become protocol/chain semantics |
| Infra adapter | `terraform` | provision base resources (network, VM, disk, cluster) | replace the process reconciler |
| Host config adapter | `ansible` | host bootstrap/config, file distribution, handlers | replace the declarative control plane |

## Boundaries per Target Backend

### Kubernetes (Runtime Backend)

Scope:

- translate workloads into Kubernetes resources suitable for stateful systems;
- support execution observation in the cluster.

Out of scope:

- provisioning the cluster;
- embedding chain logic in the backend.

### Terraform (Infra Adapter)

Scope:

- create/update infrastructure resources;
- return outputs consumable by runtime/config layers.

Out of scope:

- blockchain process operation;
- host/container service lifecycle management.

### Ansible (Host Config Adapter)

Scope:

- host bootstrap, directories, templates, units, and handlers;
- idempotent host configuration application.

Out of scope:

- the central planner/reconciler;
- the platform's global authoritative state.

## Proposed Integration Flow

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

## Rollout Strategy (Current State and Next Steps)

1. **Kubernetes backend (current state)**:
   - deterministic desired-state translation implemented;
   - optional runtime observation implemented (`status/doctor` via `kubectl`);
   - next step: optional runtime execution in the `apply` flow.
2. **Terraform adapter (current state)**:
   - deterministic artifact scaffold implemented;
   - next step: explicit plan/apply stages and output import.
3. **Ansible adapter (current state)**:
   - deterministic inventory/group_vars/playbook rendering implemented;
   - next step: controlled execution and structured result collection.

## Operational Risks

1. Divergence between infrastructure state and runtime state.
2. Increased convergence time with multiple stages.
3. Partial failures with complex cross-tool rollback.
4. Troubleshooting difficulty without event correlation across layers.

## Security Risks

1. Credential exposure (cloud tokens, SSH keys, kubeconfig).
2. Remote execution with excessive privileges.
3. Secret leakage in logs/subprocess errors.
4. Dependence on external tools without uniform hardening.

Recommended controls:

- secret masking in command output;
- least-privilege credentials;
- context/target validation before mutation;
- minimum audit trail per command/target.

## MVP Limits and Open Items

Current limits:

- local state/lock (no distributed coordination);
- no background continuous reconciler;
- capabilities vary across backends.

Open items:

1. Explicit contract for infra/config stages (`terraform`/`ansible`).
2. Cross-backend rollback/repair policy.
3. Unified observation model (local snapshot vs remote state).
4. External secret-store strategy (Vault/KMS/SOPS) integrated into the flow.
