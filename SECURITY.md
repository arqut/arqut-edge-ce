# Security Policy

Arqut Edge runs inside the user's own network and brokers remote access to
services on it. A vulnerability here is a vulnerability in someone's home or
office network, so we take reports seriously and would rather hear about a
suspected issue than not.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report privately through either channel:

1. **GitHub private vulnerability reporting** (preferred) — use the *Report a
   vulnerability* button under this repository's **Security** tab. This keeps
   the report, the discussion, and the eventual advisory in one place.
2. **Email** — contact@semilimes.com, with `SECURITY` in the subject line.

Please include:

- The affected version or commit.
- A description of the issue and its impact.
- Reproduction steps or a proof of concept, if you have one.
- How Edge was deployed (Home Assistant add-on, Docker, or bare metal) and
  which services were exposed, where relevant.

## What to expect

| Stage | Target |
| --- | --- |
| Acknowledgement of your report | within 3 business days |
| Initial assessment and severity rating | within 10 business days |
| Fix or documented mitigation for confirmed issues | within 90 days |

We will keep you updated as the assessment progresses, credit you in the
advisory unless you prefer otherwise, and let you know when a fix ships. If we
conclude a report is not a vulnerability, we will explain why.

We ask that you give us reasonable time to ship a fix before public disclosure.
We do not currently operate a paid bug bounty.

## Supported versions

Security fixes land on the latest minor release. Older releases are not
backported.

| Version | Supported |
| --- | --- |
| 0.5.x | Yes |
| < 0.5 | No |

## Scope

In scope: the code in this repository, and in particular anything that could
let a remote party reach a local service they should not, or reach Edge itself.
That includes the WireGuard tunnel setup and key handling, the WebRTC peer
connection and its signaling, the proxy service layer and its per-service
authentication, the REST API and its authorization, the embedded management UI
(including the bundled JavaScript in `ui/`), local service discovery, and the
access and event logging that users rely on to detect misuse.

Out of scope: findings that require an attacker to already have local access to
the host running Edge; vulnerabilities in third-party dependencies that are
already public and awaiting an upstream release (report those upstream, though
we appreciate a heads-up); denial of service through raw traffic volume; and
missing hardening that has no demonstrable impact.

The Arqut Server component lives in
[arqut-server-ce](https://github.com/arqut/arqut-server-ce) and has its own
policy. The hosted Arqut service at arqut.com is operated separately; reports
about it can go to the same email address.

## Deployment hardening

Edge is only as isolated as the network you run it on. Keep it updated, require
authentication on services that expose anything sensitive, review the access
logs, and share access only with people you intend to have it.
