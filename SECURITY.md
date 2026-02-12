# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No |

Only the latest release receives security fixes. There are no LTS branches.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Use [GitHub Private Vulnerability Reporting](https://github.com/fabienpiette/docker-stats-exporter/security/advisories/new) to submit a report. This keeps the discussion private until a fix is available.

Please include:

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Impact assessment (if known)

## Response

- **Acknowledgment** within 7 days
- **Fix or mitigation** targeting 30 days, depending on severity
- Credit in the release notes (unless you prefer anonymity)

## Scope

This policy covers the docker-stats-exporter binary and its Docker image.
Third-party dependencies are managed via Go modules; vulnerabilities in
upstream packages should be reported to their respective maintainers.

### In scope

- Authentication bypass (basic auth)
- Information disclosure via metrics endpoint
- Denial of service through crafted Docker API responses
- Container escape or privilege escalation
- TLS configuration weaknesses

### Out of scope

- Docker daemon security (report to Docker/Moby)
- Prometheus or Grafana vulnerabilities
- Attacks requiring access to the Docker socket (this is by design)
