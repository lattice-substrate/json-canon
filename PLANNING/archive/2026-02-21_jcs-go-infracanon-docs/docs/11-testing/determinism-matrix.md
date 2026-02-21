# Determinism Matrix: Containers, VMs, and Multi-Arch Evidence

**Status:** Draft  
**Objective:** prove that canonical output bytes are invariant across Linux systems.

## 1. What to vary
### 1.1 Userspace / distribution
Use containers for breadth:
- Debian/Ubuntu (glibc variants)
- Alpine (musl)
- Fedora (newer toolchains)

Containers are useful because they provide controlled userspace environments; however, they run on the host kernel.

Docker documentation and vendor guidance commonly note containers share the host OS kernel.  
Sources:
- Docker blog (“Containers share the OS kernel”): https://www.docker.com/blog/the-10-most-common-questions-it-admins-ask-about-docker/
- Red Hat documentation (“Linux containers share the kernel of the host OS”): https://docs.redhat.com/en/documentation/red_hat_enterprise_linux_atomic_host/7/html/overview_of_containers_in_red_hat_systems/introduction_to_linux_containers

### 1.2 Architecture
Run determinism tests on:
- linux/amd64 (native)
- linux/arm64 (native)

Docker Buildx can build multi-platform images, but emulation should be treated as a convenience smoke test rather than a replacement for native runs.  
Source: Docker multi-platform builds documentation: https://docs.docker.com/build/building/multi-platform/

### 1.3 Kernel
Use VMs to vary kernel versions and ensure the canonicalizer is not accidentally relying on kernel-dependent behavior.

## 2. What to record
Each run records:
- go toolchain version (`go env -json`)
- OS identity (distro ID, kernel `uname -a`)
- corpus manifest hash
- output hashes (sha256)

## 3. Pass/fail rule
For every corpus vector:
- output sha256 MUST be identical across all environments
- invalid vectors MUST produce the same error code across all environments

## 4. Optional: reproducible build evidence
If you also build container images reproducibly, BuildKit can consume SOURCE_DATE_EPOCH to make image timestamps deterministic.  
Source: Docker docs on reproducible builds: https://docs.docker.com/build/ci/github-actions/reproducible-builds/  
Also see SOURCE_DATE_EPOCH specification: https://reproducible-builds.org/docs/source-date-epoch/

## References
- RFC 8785 “hashable representation” / UTF‑8 output: https://www.rfc-editor.org/rfc/rfc8785
