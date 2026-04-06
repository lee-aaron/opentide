# OpenTide Security Model

## Overview

OpenTide is designed from the ground up with security as the architecture, not a bolt-on layer. Every design decision starts from: "what if this component is compromised?"

This document describes the threat model, security architecture, and specific defenses OpenTide implements.

## Threat Model

### Actors

| Actor | Trust Level | Goal |
|-------|------------|------|
| End user | Trusted input, untrusted intent | Use the AI assistant |
| Skill author | Untrusted | Distribute skills via registry |
| Skill code | Untrusted | Execute in sandbox |
| LLM provider | Trusted transport, untrusted output | Generate responses |
| Network attacker | Untrusted | Intercept, inject, exfiltrate |

### Attack Surfaces

1. **Skill execution**: Malicious or compromised skill code
2. **LLM output**: Prompt injection causing unintended actions
3. **Supply chain**: Compromised skill packages in the registry
4. **Network**: Data exfiltration through undeclared egress
5. **Approval bypass**: TOCTOU attacks on the approval flow
6. **Message injection**: Oversized or malformed messages

## Security Architecture

### 1. Mandatory Container Isolation

Every skill invocation runs in an isolated Docker container. There is no opt-out.

**Container security profile:**
- `--read-only`: Root filesystem is read-only
- `--network=none`: All network blocked by default
- `--cap-drop=ALL`: All Linux capabilities dropped
- `--security-opt=no-new-privileges:true`: No privilege escalation
- `--memory`: Hard memory limit from manifest
- `--cpus`: CPU limit from manifest
- Timeout enforcement: skill killed after declared timeout

Skills that need writable storage get a size-limited tmpfs at `/tmp`:
```
--tmpfs=/tmp:rw,noexec,nosuid,size=64m
```

The `noexec` flag prevents executing binaries written to tmpfs.

**Comparison:**
| | OpenClaw | NemoClaw | OpenTide |
|---|---|---|---|
| Default isolation | Host process | Optional container | Mandatory container |
| Filesystem | Full host access | Configurable | Read-only root, tmpfs only |
| Capabilities | All | Configurable | All dropped |
| Privilege escalation | Possible | Possible | Blocked |

### 2. Network Egress Allowlisting

OpenClaw allows all outbound network by default. This enabled the EchoLeak zero-click attack, which exfiltrated data through trusted domains.

OpenTide blocks ALL outbound network by default. Skills must declare their egress needs:

```yaml
security:
  egress:
    - "api.search.brave.com:443"
```

**Enforcement:**
- No egress declared: `--network=none` (total isolation)
- Egress declared: isolated Docker network + iptables rules allowing only declared host:port pairs
- Wildcard egress (`*`) is rejected at manifest validation
- All egress violations are logged

**iptables rules applied per skill:**
```
iptables -P OUTPUT DROP                           # default deny
iptables -A OUTPUT -o lo -j ACCEPT                # loopback
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT    # DNS
iptables -A OUTPUT -p tcp -d <IP> --dport 443 -j ACCEPT  # declared host
```

### 3. TOCTOU-Safe Approval Engine

OpenClaw's CVE-2026-29607 exploits a Time-of-Check-Time-of-Use vulnerability: an attacker can swap the action payload after the user approves it.

**OpenTide's defense:**

1. When a skill requests an action, the approval engine computes a SHA-256 hash of the full action (skill name, version, action type, target, payload)
2. The user sees the action details and approves
3. At enforcement time, the security boundary independently re-computes the hash from the actual syscall/network request
4. If the actual hash differs from the approved hash, the action is blocked and logged as a potential TOCTOU attack

```go
func HashAction(a Action) string {
    h := sha256.New()
    fmt.Fprintf(h, "%s|%s|%s|%s|%s",
        a.SkillName, a.SkillVer, a.ActionType, a.Target, a.Payload)
    return hex.EncodeToString(h.Sum(nil))
}
```

The hash is computed from the actual request at the security boundary, not from the skill's self-reported payload. This closes the TOCTOU window.

**Approval scoping:**
- Approvals are scoped per skill + action type + target
- Skill A's approval does not grant Skill B access
- Approvals expire after a configurable TTL (default 5 minutes)
- There is no global "allow all" option

### 4. Skill Supply Chain Security

OpenClaw's skill marketplace (ClawHub) has no signing or vetting.

**OpenTide requires:**

- **Ed25519 signed manifests**: Every skill published to the registry must be signed with the author's private key. Unsigned skills are rejected.
- **Signature verification at install**: The registry verifies signatures on publish. The CLI verifies on install.
- **Tamper detection**: If a manifest is modified after signing (name, version, egress rules, anything), verification fails.
- **Dependency scanning**: Before publish, skills are scanned with govulncheck (Go dependencies) and Trivy (container image). Critical and high severity findings block publication.
- **Digest pinning**: Container images are stored and referenced by SHA-256 digest, not mutable tags.

### 5. Input Validation

All channel adapters enforce:
- **Max message size**: 64KB (configurable). Oversized messages are dropped and logged.
- **UTF-8 validation**: Malformed Unicode is rejected.
- **Rate limiting**: Per-user rate limits at the adapter layer (Postgres advisory locks for durability).

### 6. Secrets Management

Skill secrets (API keys, credentials) are never passed directly:
- Non-secret config vars are passed as environment variables
- Secret config vars are accessed via a scoped secret reference API
- Per-invocation tokens, one-time-read for sensitive secrets
- All secret accesses are audit logged

### 7. Audit Trail

Every security-relevant event is logged:
- Approval requests and decisions
- Hash mismatches (TOCTOU detection)
- Egress violations
- Skill invocations (name, version, duration)
- Failed signature verifications
- Rate limit hits

The audit log is append-only.

## OpenClaw CVE Analysis

OpenTide's architecture addresses these specific vulnerabilities:

| CVE | OpenClaw Issue | OpenTide Defense |
|-----|---------------|-----------------|
| CVE-2026-29607 | Approval bypass via payload swap | TOCTOU-safe hash verification at security boundary |
| EchoLeak | Data exfiltration through trusted domains | Block-all-by-default egress with iptables |
| CVE-2026-XXXX (multiple) | Host process skill execution | Mandatory container isolation |
| Supply chain | No skill signing | Ed25519 signed manifests + dependency scanning |
| Privilege escalation | Skills run as host user | --cap-drop=ALL, --no-new-privileges, non-root user |

## Security Roadmap

### Phase 2 (current): Docker + seccomp
- Container isolation with Docker
- seccomp profiles for syscall filtering
- Network namespace isolation with iptables

### Phase 3 (next): gVisor upgrade
- Replace Docker runtime with gVisor (runsc)
- Kernel-level syscall interception
- Stronger isolation than seccomp alone

### Phase 4: Hardening
- OpenTelemetry security event tracing
- Anomaly detection on skill behavior
- Multi-tenant isolation
- Security audit by external firm
