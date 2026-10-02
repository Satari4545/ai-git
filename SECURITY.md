# Security Policy

## Supported Versions

The latest `v0.x` release is supported. Older releases receive no security
updates; upgrade to pick up fixes.

## Reporting a Vulnerability

**Do not open a public issue** for a suspected vulnerability. Instead, open
a private security advisory:

**https://github.com/Satari4545/ai-git/security/advisories/new**

Include:

- what is affected (command, version, platform)
- what you expected vs. what happened
- steps to reproduce, if you have them

We will acknowledge within 72 hours and aim to ship a fix or a mitigation
within 30 days.

## What We Protect

- **Your code stays yours.** ai-git stores context as git notes bound to your
  own commits; nothing is sent to any ai-git server (there is none).
- **Supply chain.** Release binaries are built by GitHub Actions from tagged
  source, published with SHA-256 checksums, and carry SLSA build provenance
  attestations. The npm installer verifies the checksum and refuses to
  install on mismatch.
- **Secrets.** ai-git redacts common secret patterns from recorded context
  before writing notes. Treat context notes with the same care as commit
  messages: they travel with your repo.

## Scope Notes

ai-git shells out to `git` and to your configured agent CLIs. A malicious
repo, hook script, or agent CLI is outside what ai-git can defend against;
review those the same way you would for plain git.
