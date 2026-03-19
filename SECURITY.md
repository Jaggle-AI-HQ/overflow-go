# Security Policy

## Supported Versions

| Package     | Supported |
| ----------- | --------- |
| overflow-go | Latest    |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly.

**Do not open a public issue.**

Instead, email **<tashi.dakpa@jaggle.tech>** with:

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Any potential impact

We will acknowledge your report within 48 hours and aim to release a fix within 7 days for critical issues.

## Scope

The Overflow Go SDK runs in server-side Go applications. Security concerns include but are not limited to:

- DSN or API key exposure beyond intended scope
- Sensitive data leaking into error payloads
- Denial of service through SDK behavior
- Race conditions in concurrent usage
