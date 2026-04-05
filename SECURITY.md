# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Brockley, please report it responsibly. Do not open a public GitHub issue.

Send an email to **security@brockleyai.com** with:

- A description of the vulnerability
- Steps to reproduce or a proof of concept
- The affected component(s) and version(s), if known
- Any potential impact you have identified

## What Constitutes a Security Issue

- Authentication or authorization bypasses
- Remote code execution
- SQL injection, command injection, or other injection attacks
- Sensitive data exposure (credentials, tokens, secrets)
- Path traversal or file access outside intended boundaries
- Cryptographic weaknesses
- Privilege escalation
- Denial of service vulnerabilities in server components

## Response Timeline

- **Acknowledgment:** Within 48 hours of your report
- **Fix assessment:** Within 7 days we will provide an initial assessment, including severity classification and an estimated timeline for a fix
- **Resolution:** Varies by severity, but we aim to release patches for critical issues as quickly as possible

## Responsible Disclosure

We ask that you:

- Do not publicly disclose the vulnerability until a fix is available and has been released
- Do not exploit the vulnerability beyond what is necessary to demonstrate it
- Do not access, modify, or delete data belonging to other users

We will coordinate with you on disclosure timing and will credit you in the advisory unless you prefer to remain anonymous.

## Scope

The following components are in scope:

- Brockley Go server
- Brockley CLI
- Terraform provider
- Official Docker images
- Official Helm charts
- Web UI (the graph editor)

## Out of Scope

The following are out of scope for this security policy:

- Vulnerabilities in third-party LLM providers (OpenAI, Anthropic, etc.)
- Vulnerabilities in third-party MCP servers
- Social engineering attacks
- Phishing
- Physical security
- Denial of service via volumetric/network-level flooding
- Issues in dependencies that do not have a demonstrated exploit path in Brockley

If you are unsure whether something is in scope, report it anyway and we will let you know.
