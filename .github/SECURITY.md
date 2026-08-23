# Security Policy

## 🛡️ Supported Versions

We actively provide security patches and bug fixes for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.3.x   | :white_check_mark: |
| 1.2.x   | :white_check_mark: |
| < 1.2.0 | :x:                |

---

## 🔒 Reporting a Vulnerability

The security and privacy of our users are of paramount importance. Because **NexVPN Pro** interacts directly with operating system network stacks, TUN devices, routing tables, and firewall rules, we take all vulnerabilities seriously.

### How to Report

1. **Do NOT open a public GitHub issue** for sensitive vulnerabilities or potential exploits.
2. Please disclose the vulnerability confidentially by:
   - Creating a private **GitHub Security Advisory** on our repository: [Security Advisories](https://github.com/Rpg-420/NexVPN/security/advisories/new)
   - Or contacting the core maintainers via secure email/Telegram.

### What to Include in Your Report

To help us investigate and patch the issue promptly, please include:
- A detailed description of the vulnerability.
- Steps to reproduce the issue (proof of concept code, configuration, or environment details).
- The operating system and NexVPN version tested.
- Any suggested mitigations or patches if available.

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours.
- **Investigation & Patching**: We will analyze the report, keep you informed of our progress, and develop a coordinated fix.
- **Credit**: Upon public release of the patch, we will gladly credit you in our release notes (unless you prefer to remain anonymous).

---

## 🔐 Security Architecture Best Practices in NexVPN Pro

1. **Zero-Leak Kill Switch**: Enforces strict outbound firewall blocking rules at OS level to guarantee real IP addresses are never exposed during tunnel drops.
2. **Local Encrypted Storage**: All sensitive credentials, tokens, and server secrets are AES-GCM encrypted on disk.
3. **No External Telemetry**: NexVPN Pro does not log, collect, or transmit user browsing traffic, visited domains, or personal identifiers.
4. **Sandboxed Processes**: Core sub-processes are managed with minimized privileges and hidden window handles.
