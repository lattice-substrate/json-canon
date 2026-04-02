Forensic Audit: AWS Native Conformance Gating Infrastructure                                
                                                                                              
  Branch: feat/amzn-s3-conformance-testing (9 commits, 56 files, +7,391 / -509 lines)         
  Scope: Everything on this branch intended to strengthen conformance evidence via native AWS 
  EC2 instances for adversarial audit under EU DORA, NIS2, CRA, and US SEC cybersecurity      
  regimes.                                                                                    
                                                                                              
  ---             
  Executive Assessment
                      
  The architectural intent is correct and well-motivated. Evidence.v2, infra-manifest binding,
   pinned toolchains, and Go-native orchestration are the right primitives for                
  infrastructure-grade conformance gating. The ADRs (0005-0007) are sound. The requirement
  registries and citation index are exceptionally strong.                                     
                  
  However, the implementation has critical gaps that would invalidate the evidence chain under
   adversarial audit. The issues fall into categories that matter for regulatory regimes:
  evidence integrity, infrastructure lifecycle safety, determinism, test coverage, and        
  operational security.

  ---
  CATEGORY 1: EVIDENCE INTEGRITY (Audit-Invalidating)
                                                                                              
  1.1 CRITICAL — No Evidence Integrity Verification After SSH Transfer
                                                                                              
  File: server_evidence.go:914-919                                                            
                                                                                              
  Evidence JSON is downloaded via cat over SSH and written to disk with no integrity check. No
   hash verification, no JSON validation, no size check. A corrupt transfer, truncated stream,
   or MITM produces silently invalid evidence that passes downstream gates if the corruption  
  is structurally valid JSON.

  Regulatory impact: Any auditor examining the evidence chain can ask "how do you prove the   
  evidence file received matches what was generated on the remote host?" There is currently no
   answer.                                                                                    
                  
  Remedy: Compute SHA-256 on the remote host before download, transfer both the file and the  
  digest, verify locally before accepting.
                                                                                              
  1.2 CRITICAL — SSH Host Key Verification is Trust-On-First-Use (TOFU)                       
  
  File: server_evidence.go:1085-1099                                                          
                  
  The first SSH connection to each host accepts any key and stores it. Only subsequent        
  connections detect key changes. For freshly provisioned EC2 instances, this means every
  single connection is a first connection — TOFU provides zero protection. An attacker        
  performing a network-level MITM during provisioning could intercept all evidence generation.

  Regulatory impact: Under DORA/NIS2, the communication channel for evidence gathering must   
  have verifiable integrity. TOFU on ephemeral infrastructure provides no verifiable trust
  anchor.                                                                                     
                  
  Remedy: Retrieve EC2 instance console output to obtain the SSH host key fingerprint         
  published by cloud-init, then verify against it before first connection. AWS provides
  GetConsoleOutput API for this.                                                              
                  
  1.3 MAJOR — Non-Atomic Evidence File Writes                                                 
  
  File: server_evidence.go:1012-1024                                                          
                  
  Evidence is downloaded, directory created, file written, then permissions set — in 4        
  separate operations. A crash between write and chmod leaves evidence with wrong permissions.
   A crash during write leaves partial evidence.                                              
                  
  Remedy: Write to a temp file in the same directory, then os.Rename (atomic on same          
  filesystem), then verify.
                                                                                              
  1.4 MAJOR — Unbounded SSH Output Buffer                                                     
  
  File: server_evidence.go:1046-1048                                                          
                  
  SSH command stdout/stderr captured in bytes.Buffer with no size limit. A compromised or     
  malfunctioning remote host can OOM the orchestrator by sending unbounded output. This is
  particularly dangerous because evidence bundles are transferred via cat over SSH.           
                  
  Remedy: Use io.LimitedReader with a sane maximum (e.g., 256 MiB for evidence, 64 KiB for    
  discovery commands).
                                                                                              
  ---             
  CATEGORY 2: INFRASTRUCTURE LIFECYCLE (Operational Risk)
                                                         
  2.1 CRITICAL — Infrastructure Leak When SSH Setup Fails After Provisioning
                                                                                              
  File: server_evidence.go:300-326                                                            
                                                                                              
  If provisionServerInfrastructure() succeeds (line 304) but newServerSSHRunner() fails (line 
  308-310) or SSH wait fails (line 322-324), the function returns an error. The caller
  runServerEvidence() (line 173-174) returns immediately. The defer-destroy at line 176 only  
  runs after provision() returns successfully. Result: provisioned AWS instances are orphaned,
   running indefinitely, incurring cost.

  Remedy: Move the defer-destroy to run unconditionally whenever infrastructure exists:       
  
  if err = runtimeState.provision(stdout); err != nil {                                       
      if runtimeState.infra != nil {                                                          
          _ = runtimeState.destroy() // best-effort cleanup
      }                                                                                       
      return err  
  }                                                                                           
                  
  2.2 MAJOR — No SIGTERM/SIGINT Handler During Long Operations                                
  
  The orchestrator runs provision (20min), replays (hours), and gates (10min) with no signal  
  handling. If the operator presses Ctrl-C during provisioning, the Go process exits, leaving
  AWS infrastructure alive with no cleanup. There is no signal.Notify anywhere in the file.   
                  
  Remedy: Install signal handler that triggers graceful shutdown with infrastructure destroy. 
  
  2.3 MAJOR — No Terraform Remote State Backend                                               
                  
  File: infra/main.tf                                                                         
                  
  State is stored locally. This means: no state locking (concurrent runs can corrupt), no     
  audit trail, no shared state, and state loss means orphaned resources with no way to destroy
   them programmatically.                                                                     
                  
  Remedy: Configure S3+DynamoDB backend (or Terraform Cloud) with state locking. For ephemeral
   per-release infrastructure, even a per-tag state key is sufficient.
                                                                                              
  2.4 MAJOR — Security Group Allows Unrestricted Egress                                       
  
  File: infra/main.tf:32-37                                                                   
                  
  All outbound traffic is allowed. A compromised conformance host can exfiltrate data, join   
  botnets, or communicate with C2. Since these instances execute arbitrary binaries as part of
   conformance testing, egress should be restricted to the minimum required (likely just      
  responses to SSH, and possibly Go module downloads if building on-host).

  2.5 MAJOR — No IMDSv2 Enforcement                                                           
  
  File: infra/instances.tf                                                                    
                  
  No metadata_options block. Instances default to IMDSv1, which is vulnerable to SSRF-based   
  credential theft. While these instances don't have IAM roles attached, this is a
  defense-in-depth failure that auditors will flag.                                           
                  
  Remedy: Add metadata_options { http_tokens = "required" }.

  2.6 MAJOR — No SSH Ingress CIDR Validation                                                  
  
  Files: server_evidence.go:155, infra/variables.tf                                           
                  
  SSH ingress CIDR is passed through to OpenTofu without validation. If accidentally set to   
  0.0.0.0/0, all instances are SSH-accessible from the internet. No Terraform variable
  validation block exists.                                                                    
                  
  Remedy: Add validation { condition = can(cidrhost(var.ssh_ingress_cidr, 0)) } in            
  variables.tf and reject /0 masks.
                                                                                              
  ---             
  CATEGORY 3: DETERMINISM AND REPRODUCIBILITY
                                                                                              
  3.1 MAJOR — context.Background() Used Instead of Propagated Context
                                                                                              
  Files: server_evidence.go:322, 357, 449                                                     
                                                                                              
  Three critical operations use context.Background() instead of propagating the parent        
  context:        
  - SSH wait (line 322)                                                                       
  - Remote fact discovery (line 357)
  - Matrix replay execution (line 449)
                                                                                              
  This means parent timeouts and cancellations are ignored. The 12-hour serverRuntimeTimeout
  constant (line 35) is never enforced because nothing creates a context with that timeout.   
                  
  Regulatory impact: Unbounded runtime means an operator cannot guarantee when evidence       
  generation terminates. An auditor can question whether evidence was generated in a
  controlled timeframe.                                                                       
                  
  Remedy: Create a root context with serverRuntimeTimeout in runServerEvidence() and propagate
   it through all operations.
                                                                                              
  3.2 MAJOR — AMI Wildcards Allow Version Drift                                               
  
  File: infra/instances.tf (via aws_release_hosts.json)                                       
                  
  Debian hosts use ami_name = "debian-13-amd64-*" wildcards. Each tofu apply can select a     
  different AMI if Debian publishes a new image. Two runs of the same release may execute on
  different OS images.                                                                        
                  
  Regulatory impact: Conformance evidence claims "we tested on Debian 13" but cannot prove    
  which Debian 13 image. The image_id is recorded in the infra manifest, but the input is
  non-deterministic.                                                                          
                  
  Remedy: Either pin to specific AMI IDs in the host catalog or add the resolved AMI ID to the
   infra manifest with an explicit "resolved from pattern" annotation. The infra manifest
  already records image_id — this mitigates but doesn't eliminate the issue.                  
                  
  3.3 MAJOR — Manifest Timestamp Has No Clock Integrity                                       
  
  File: server_evidence.go:406                                                                
                  
  GeneratedAtUTC uses manifestNowUTC().Format(time.RFC3339Nano). No NTP verification, no clock
   skew detection. An incorrect system clock produces incorrect timestamps in the evidence
  chain.                                                                                      
                  
  3.4 MODERATE — Non-Deterministic Temporary Directory Names                                  
  
  File: server_evidence.go:893                                                                
                  
  Remote temp directories include randomSuffix(). While not recorded in evidence, this means  
  log files contain non-reproducible paths, complicating forensic reconstruction.
                                                                                              
  ---             
  CATEGORY 4: TEST COVERAGE (Critically Insufficient)
                                                     
  4.1 CRITICAL — 89 Lines of Tests for 1,284 Lines of Orchestration
                                                                                              
  Files: server_evidence_test.go (89 lines) vs server_evidence.go (1,284 lines) — 6.9% test   
  ratio                                                                                       
                                                                                              
  The 4 tests cover:                                                                          
  1. TestParseSSHTarget — parses user@host strings
  2. TestResolveGitHeadCommitLooseGitDir — reads .git indirection                             
  3. TestBuildRemoteReplayCommand — builds shell command string  
  4. TestServerSSHTargetEnvKey — constructs env var name                                      
                                                                                              
  Not tested at all:                                                                          
  - runServerEvidence() — the main lifecycle                                                  
  - provision() / destroy() — infrastructure management
  - execute() — the entire evidence generation flow                                           
  - discoverRemoteFacts() — substrate identity collection                                     
  - writeInfraManifest() — manifest generation           
  - buildArtifacts() — cross-architecture binary building                                     
  - runReplays() / runReplayForArch() — replay execution                                      
  - runReleaseGates() — gate validation                                                       
  - SSH Wait(), uploadFile(), downloadFile() — transport layer                                
  - tofuOutputHosts() — Terraform output parsing                                              
  - Error paths, cleanup paths, partial failure recovery                                      
                                                                                              
  Regulatory impact: Under adversarial audit, the auditor can ask "what evidence do you have  
  that the evidence-generation tool itself works correctly?" The answer is: almost none. The  
  tool that generates the conformance evidence is itself essentially untested.                
                                                                                              
  4.2 MAJOR — No End-to-End Integration Test for Evidence Chain                               
  
  No test validates the complete flow: evidence.v2 + infra-manifest.v1 +                      
  infra-substrate-binding profile + actual validation. Components are tested independently but
   never together.                                                                            
                  
  4.3 MAJOR — Evidence Validation Tests Use Identical Digests                                 
  
  File: evidence_test.go:131-183                                                              
                  
  All test fixtures use identical digests across all nodes. This hides bugs in baseline       
  selection logic (line 274-277 of evidence.go) where the baseline is selected from the first
  replay encountered — order-dependent in a language with non-deterministic map iteration.    
                  
  4.4 MAJOR — Toolchain Lock Parsing Tests Minimal                                            
  
  File: toolchain_lock_test.go (127 lines for 268 lines of implementation)                    
                  
  Missing tests for: malformed lines, wrong field counts, empty fields, duplicate IDs,        
  non-HTTPS URLs, missing schema header, Windows line endings.
                                                                                              
  4.5 MAJOR — No Negative Test Cases for Infrastructure Manifest                              
  
  Missing tests for: non-HTTPS infra_repo_url, invalid SHA-256 format, duplicate node IDs     
  across hosts, path traversal in tool paths, empty host arrays.
                                                                                              
  ---             
  CATEGORY 5: SUPPLY CHAIN AND SECURITY
                                                                                              
  5.1 MAJOR — Tar/ZIP Extraction Doesn't Reject Symlinks
                                                                                              
  File: toolchain.go:386-415
                                                                                              
  extractTarEntry() handles tar.TypeDir and tar.TypeReg. Unknown types (symlinks, hardlinks,  
  character devices) silently fall through without error. A malicious tarball could contain a
  symlink go/bin/go -> /tmp/malicious-binary that passes safeArchivePath() validation.        
                  
  Remedy: Explicitly reject all entry types except TypeDir and TypeReg with an error.         
  
  5.2 MAJOR — TOCTOU in SHA-256 Verification                                                  
                  
  File: toolchain.go:232-248                                                                  
  
  Downloads check if the file already exists with correct SHA-256, then skip the download.    
  Between the check and subsequent use, the file could be replaced. More critically, after
  downloading to a temp file and renaming, the SHA-256 of the final file is never verified.   
                  
  Remedy: Verify SHA-256 after the atomic rename, not before.                                 
  
  5.3 MAJOR — No GPG/Sigstore Signature Verification on Toolchain Downloads                   
                  
  Only SHA-256 is verified. If the download source is compromised (e.g., Go CDN or GitHub     
  releases), SHA-256 in the lock file could be updated by the attacker. No independent
  signature verification.                                                                     
                  
  5.4 MODERATE — Passphrase-Protected SSH Keys Silently Fail                                  
  
  File: server_evidence.go:804                                                                
                  
  ssh.ParsePrivateKey() fails on passphrase-protected keys with a generic error. No hint to   
  the operator about the actual cause. Should detect and suggest
  ssh.ParsePrivateKeyWithPassphrase() or key decryption.                                      
                  
  5.5 MODERATE — Terraform Provider Version Too Loose

  File: infra/main.tf:5                                                                       
  
  version = "~> 5.0" allows any 5.x AWS provider. The lock file pins to 5.100.0, but the      
  constraint should match: ~> 5.100 prevents accidental drift.
                                                                                              
  ---             
  CATEGORY 6: DOCUMENTATION AND OPERATIONAL GAPS
                                                                                              
  6.1 MAJOR — No Unified AWS Release Runbook
                                                                                              
  The AWS release process is scattered across 4 documents (offline/README.md, ADR-0005/6/7,   
  CONTRIBUTING.md, scripts/release-server.sh). An operator has no single-source guide for     
  end-to-end execution.                                                                       
                  
  6.2 MAJOR — No Infrastructure Failure Recovery Guide

  What happens when: SSH times out on 2 of 24 hosts? One AMI is unavailable? OpenTofu apply   
  partially succeeds? Network partition during replay? No documentation covers partial failure
   scenarios.                                                                                 
                  
  6.3 MAJOR — No Evidence Audit Guide

  No documentation explains how an external auditor should verify: infra_manifest_sha256      
  binding, cross-architecture parity, discovered field accuracy, toolchain artifact
  provenance.                                                                                 
                  
  6.4 MODERATE — Evidence v1 vs v2 Deprecation Unclear                                        
  
  v1 is "supported offline development format" but no timeline for when v1 becomes            
  insufficient. An auditor could question whether v1-only evidence for pre-release builds
  weakens the overall trust story.                                                            
                  
  ---
  PROS: What Has Been Done Well
                                                                                              
  1. Architecture is fundamentally sound. Evidence.v2 + infra-manifest separation with SHA-256
   binding is the right design. Trust boundaries between provisioning (OpenTofu) and execution
   (Go) are clean.
  2. ADRs are thorough. ADR-0005/6/7 document the "why" behind every decision. This is exactly
   what adversarial auditors want to see.                                                     
  3. Requirement registries are exemplary. 103 requirement IDs mapped to implementation
  symbols and test functions with citation index back to RFC clauses. This is rare and        
  valuable.       
  4. Pinned toolchain policy is correct. Eliminating ambient binaries from the evidence path  
  is essential. The toolchain.lock.tsv format with SHA-256 is appropriate.                    
  5. Go-native orchestration (ADR-0007) is the right call. Minimizing shell surface area in
  the trust-critical path reduces injection risk and improves auditability.                   
  6. Suite-driven policy is elegant. Using existing required_suites mechanism rather than
  adding boolean flags keeps the schema clean and extensible.                                 
  7. Dual-architecture coverage (x86_64 + arm64, 12 lanes each, 5 replays = 120 total) with
  diverse Linux distributions is comprehensive.                                               
  8. SSH transport in Go eliminates dependency on host SSH binaries with their
  configuration-dependent behavior.                                                           
  9. Sensitive output gating in Terraform prevents SSM-resolved AMI IDs from leaking into
  logs.                                                                                       
  10. No reverts or architectural rework in the commit history — the 9 commits show
  systematic, iterative development.                                                          
                  
  ---                                                                                         
  CONS: What Is Broken or Incomplete
                                    
  ┌─────┬────────────────┬──────────┬─────────────────────────────┬──────────────────────┐ 
  │  #  │    Category    │ Severity │            Issue            │     Audit Impact     │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 1   │ Evidence       │ CRITICAL │ No hash verification after  │ Evidence chain       │    
  │     │ Integrity      │          │ SSH transfer                │ unverifiable         │ 
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤    
  │ 2   │ Evidence       │ CRITICAL │ TOFU SSH on ephemeral hosts │ Communication        │ 
  │     │ Integrity      │          │  = no trust                 │ channel unverified   │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 3   │ Infra          │ CRITICAL │ Infrastructure leak on SSH  │ Uncontrolled cloud   │    
  │     │ Lifecycle      │          │ setup failure               │ spend                │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 4   │ Test Coverage  │ CRITICAL │ 6.9% test ratio on          │ Evidence tool itself │    
  │     │                │          │ orchestrator                │  untested            │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 5   │ Infra          │ MAJOR    │ No signal handler →         │ Uncontrolled         │    
  │     │ Lifecycle      │          │ orphaned infra on Ctrl-C    │ teardown             │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 6   │ Infra          │ MAJOR    │ No remote Terraform state → │ State management     │    
  │     │ Lifecycle      │          │  no locking/audit           │ failure              │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 7   │ Determinism    │ MAJOR    │ context.Background()        │ Unbounded runtime    │    
  │     │                │          │ ignoring timeouts           │                      │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 8   │ Security       │ MAJOR    │ Tar extraction doesn't      │ Supply chain         │    
  │     │                │          │ reject symlinks             │ injection vector     │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 9   │ Security       │ MAJOR    │ No egress restriction on    │ Data exfiltration    │    
  │     │                │          │ conformance hosts           │ possible             │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 10  │ Security       │ MAJOR    │ No IMDSv2 enforcement       │ SSRF credential      │    
  │     │                │          │                             │ theft                │ 
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 11  │ Test Coverage  │ MAJOR    │ No end-to-end integration   │ Component isolation  │ 
  │     │                │          │ test                        │ masks bugs           │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 12  │ Determinism    │ MAJOR    │ AMI wildcards allow version │ Non-reproducible     │    
  │     │                │          │  drift                      │ environments         │    
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤ 
  │ 13  │ Security       │ MAJOR    │ CIDR validation missing     │ Accidental public    │    
  │     │                │          │                             │ SSH                  │
  ├─────┼────────────────┼──────────┼─────────────────────────────┼──────────────────────┤    
  │ 14  │ Documentation  │ MAJOR    │ No unified runbook or audit │ Operational          │
  │     │                │          │  guide                      │ fragility            │    
  └─────┴────────────────┴──────────┴─────────────────────────────┴──────────────────────┘

  ---
  PRIORITIZED REMEDIATION PLAN
                              
  Phase 1 — Audit-Blocking (Must fix before any release claim)
                                                                                              
  1. Add post-transfer SHA-256 verification for evidence files downloaded over SSH            
  2. Retrieve and verify SSH host key fingerprints from EC2 console output before connecting  
  3. Fix infrastructure leak when SSH setup fails after provisioning                          
  4. Install SIGTERM/SIGINT handler with infrastructure cleanup                               
  5. Add comprehensive unit tests for the orchestration layer (target: 60%+ coverage on
  server_evidence.go)                                                                         
  6. Add at least one end-to-end integration test that validates the complete evidence.v2 +
  infra-manifest + profile binding chain                                                      
                  
  Phase 2 — Security Hardening                                                                
                  
  7. Reject non-regular file types in tar/ZIP extraction                                      
  8. Verify SHA-256 after atomic rename, not before download
  9. Restrict security group egress to necessary destinations                                 
  10. Enforce IMDSv2 with http_tokens = "required"                                            
  11. Add CIDR validation in both Go code and Terraform variable validation                   
  12. Tighten provider constraint to ~> 5.100                                                 
                                                                                              
  Phase 3 — Determinism and Operational Maturity                                              
                  
  13. Propagate context with serverRuntimeTimeout through all operations                      
  14. Configure S3 + DynamoDB remote Terraform state backend
  15. Document AMI resolution strategy — either pin AMIs or annotate "resolved from pattern"  
  in manifest                                                                                 
  16. Write unified AWS Release Runbook consolidating all operational guidance                
  17. Write Evidence Audit Guide for external reviewers                                       
  18. Add negative tests for infra manifest validation, toolchain lock parsing, and evidence  
  chain binding                                                                               
                                                                                              
  ---                                                                                         
  Bottom Line     
             
  The engineering vision and architecture are strong — this is the right approach for
  infrastructure-grade conformance evidence. But the implementation shipped the happy path    
  without the defensive infrastructure required for adversarial audit. The three critical
  issues (evidence integrity, SSH trust, and infrastructure lifecycle) each independently     
  provide an auditor with grounds to reject the entire evidence chain. The 6.9% test coverage
  on the evidence-generation tool means you cannot claim the tool that generates conformance
  evidence is itself conformant.

  Fix Phase 1 before claiming this infrastructure strengthens the evidence story. In its      
  current state, it weakens it — because it introduces a complex new attack surface (AWS
  infrastructure, SSH transport, remote execution) without the verification mechanisms needed 
  to defend it.   