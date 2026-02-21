# Security Policy

## Reporting a Vulnerability

**Use GitHub's private vulnerability reporting** to report security issues:
navigate to the repository's **Security** tab -> **Advisories** -> **Report a vulnerability**.

If GitHub private reporting is unavailable, email:
`SolutionsExcite@gmail.com` (listed in `NOTICE`).

Do **not** open public issues for unpatched vulnerabilities.

Include:
- affected version and environment
- reproduction steps and input sample
- observed impact
- suggested mitigation (if known)

## Response Targets

| Stage | Target |
|-------|--------|
| Initial acknowledgment | 5 business days |
| Severity triage | 10 business days |
| Fix available (Critical/High) | 30 calendar days |
| Fix available (Medium/Low) | 90 calendar days |

## Supported Versions

Security fixes are provided for:
- latest release on the default branch
- previous minor release line (when one exists)

Older versions receive no security updates.

## Disclosure Process

1. Maintainers acknowledge receipt and triage severity.
2. A fix is developed and validated in CI.
3. A coordinated release is published with notes and upgrade guidance.
4. Public disclosure follows release availability.
