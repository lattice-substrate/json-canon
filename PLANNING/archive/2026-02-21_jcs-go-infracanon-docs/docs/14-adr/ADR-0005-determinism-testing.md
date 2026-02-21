# ADR-0005: Determinism Testing Using Containers + Native Multi-Arch + VMs

- **ADR ID:** ADR-0005
- **Date:** 2026-02-21
- **Status:** Accepted

## Context
Canonical output must be byte-identical across Linux systems. Containers provide userspace diversity but share the host kernel; VMs vary the kernel.  
Sources:
- Docker blog (containers share OS kernel): https://www.docker.com/blog/the-10-most-common-questions-it-admins-ask-about-docker/
- Red Hat docs (containers share host kernel): https://docs.redhat.com/en/documentation/red_hat_enterprise_linux_atomic_host/7/html/overview_of_containers_in_red_hat_systems/introduction_to_linux_containers
- Docker multi-platform build docs: https://docs.docker.com/build/building/multi-platform/

## Decision
Run determinism evidence in CI across:
- distro container matrix
- native amd64 + arm64 runners
- VM-based kernel diversity where feasible

## Rationale
Produces credible evidence for auditors while keeping the matrix maintainable.

## Consequences
More CI complexity; higher confidence in cross-environment determinism.
