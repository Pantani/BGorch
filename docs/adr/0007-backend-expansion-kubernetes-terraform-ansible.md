# ADR 0007: Backend Expansion (Kubernetes + Terraform/Ansible Adapters)

- Status: Accepted
- Date: 2026-04-16

## Context

The current codebase already has:

- a minimum backend contract (`ValidateTarget`, `BuildDesired`);
- optional runtime contracts (`RuntimeExecutor`, `RuntimeObserver`);
- implemented backends: `docker-compose`, `ssh-systemd`, `kubernetes`, `terraform`, `ansible`.

At the current stage, expansion started with:

1. `kubernetes` as an artifact-first runtime backend;
2. `terraform` as an artifact-first provisioning adapter;
3. `ansible` as an artifact-first bootstrap/configuration adapter.

Without these boundaries, there is a risk of:

- coupling the core to specific tools;
- pushing process orchestration into Terraform/Ansible;
- increasing the `apply` blast radius through mixed responsibilities.

## Decision

Adopt an explicit model with three backend classes and separate responsibilities:

1. **Runtime backend** (`docker-compose`, `ssh-systemd`, `kubernetes`)
2. **Infra adapter** (`terraform`)
3. **Host config adapter** (`ansible`)

### Mandatory Boundaries

- `kubernetes`:
  - responsible for materializing and observing workloads/state in the cluster;
  - does not assume chain/protocol semantics.
- `terraform`:
  - responsible for provisioning infrastructure (VM, network, disk, cluster);
  - does not reconcile blockchain node processes.
- `ansible`:
  - responsible for host bootstrap/configuration (dirs, units, files, handlers);
  - does not replace the core planner/reconciler.

### Integration Semantics

- `plan` remains mandatory in the core before mutations.
- Core `apply` keeps local lock/snapshot as the minimum safety mechanism.
- Runtime execution remains opt-in through backend capability.
- Infra/config adapters must expose observable results to `status/doctor` without taking ownership of core state.

## Rationale

- Preserves separation of concerns (core vs runtime vs infra vs bootstrap).
- Enables incremental evolution without rewriting current contracts.
- Keeps idempotency and diagnostics in the primary control plane (Go core).

## Consequences

### Positive

- growth toward Kubernetes/Terraform/Ansible with low coupling;
- future compatibility with multiple operational environments;
- lower risk of architectural lock-in.

### Negative

- backend capability differences increase UX complexity;
- higher cross-integration testing effort (backend x plugin x runtime);
- need for strong documentation of backend limits.

## Operational and Security Risks

1. **Multi-layer drift**: infrastructure is provisioned, but runtime diverges.
2. **Sensitive credentials**: cloud tokens, SSH keys, kubeconfig, chain secrets.
3. **Destructive commands**: broad `terraform apply/destroy` and remote handlers.
4. **Privilege escalation**: `ansible become`, cluster-admin access in Kubernetes.
5. **Inconsistent diagnostics**: `status/doctor` does not distinguish local snapshot from observed remote state.

Proposed minimum mitigations:

- explicit execution through flags/capabilities (no implicit effects);
- secret masking in logs and errors;
- target/context validation before mutation;
- standardized observability per backend (summary + details);
- verification gates (`verify`) before integration.

## MVP Limits

Current implementation state:

- `kubernetes` implements `BuildDesired`, minimum validation, and runtime observe (no runtime exec);
- `terraform` implements `BuildDesired` and minimum validation (no runtime exec/observe);
- `ansible` implements `BuildDesired` and minimum validation (no runtime exec/observe);
- distributed locking and a continuous reconciler remain out of scope.

## Next Steps (Proposed)

1. Evolve `kubernetes` toward runtime-execution capability with explicit operational semantics (runtime observe is already implemented).
2. Define a staged contract for `terraform` (`plan/apply/output import`) without coupling it to the core reconciler.
3. Define a staged contract for `ansible` (inventory/playbook/result mapping) with structured output for `status/doctor`.
4. Add an integration-test matrix by capability (runtime/infra/config).
5. Evolve `status/doctor` to explicitly separate:
   - local state (snapshot),
   - state observed in runtimes/adapters.
