# ADR 0001: Core Architecture (Declarative Reconciler in Go)

- Status: Accepted
- Date: 2026-04-16

## Context

`bgorch` must operate across multiple blockchain families, multiple processes per logical node, and multiple runtimes (Compose, host/systemd, Kubernetes), without coupling the core to any specific chain.

It is also an explicit requirement that the product not be only:
- a Terraform provider,
- a collection of Ansible playbooks,
- a thin Docker Compose wrapper.

## Decision

Adopt a first-party declarative core in Go with these building blocks:

1. **Versioned API** (`v1alpha1`) for desired state.
2. **Validation + normalization** before any render/plan step.
3. **Planner** that compares desired state with observed/stored snapshot state.
4. **Deterministic renderer** for backend artifacts.
5. **Plugin abstraction** (chain family) separated from the backend abstraction (runtime/target).
6. **Initial local state** stored in JSON files with room to evolve toward a transactional backend (SQLite/Postgres/etcd).

## Rationale

- Enables idempotent convergence and drift detection within the product itself.
- Preserves separation between chain semantics and runtime semantics.
- Keeps clear boundaries so Terraform/Ansible integrate as adapters, not as the core.

## Consequences

### Positive
- Extensible architecture through plugins/backends without rewriting the core.
- Clear `plan`-first contract.
- Better testability (validation, planner, render, golden tests).

### Negative
- More upfront code than coupling directly to existing tools.
- Need to preserve API/versioning compatibility early.

## Rejected alternatives

1. **Terraform-first core**
   - Rejected: Terraform is excellent for provisioning, but inadequate as a runtime reconciler for blockchain processes.
2. **Ansible-first core**
   - Rejected: playbooks do not replace a declarative domain with cross-backend state/plan/drift.
3. **Compose-first product**
   - Rejected: it would bind the model to the local runtime and limit evolution toward host/Kubernetes targets.
