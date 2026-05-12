# INSTALLER_TODO.md

Changes required in the separate `1Panel-dev/installer` repo to complete the
multi-node SSH-push enrollment flow. The core/agent side is implemented in
this repo (Fase 5 of PLAN.md); these are the open items on the installer
side.

## Required: agent-only install mode

Today `install.sh` provisions a full master (core + agent + UI). Enrollment
needs a flag that skips core and writes the bootstrap files we already
deliver via SCP.

Expected interface:

```bash
INSTALL_MODE=agent \
NODE_NAME=prod-web-1 \
NODE_PORT=9999 \
bash install.sh
```

When `INSTALL_MODE=agent`:

1. Skip download/install of `1panel-core` binary, `1panel-core.service`, and
   the web entry.
2. Install only `1panel-agent` binary + `1panel-agent.service`.
3. Do NOT generate a self-signed cert pair — the bootstrap dir written by
   the master (`/etc/1panel/bootstrap/{scope,port,proxy_id,server.crt,
   server.key,root.crt}`) is the source of truth. The agent's
   `xpack.LoadNodeInfo()` picks them up at first boot.
4. Skip the "open the panel at http://… " final message.
5. Skip creation of the master's `EncryptKey` setting; the agent generates
   its own on first migration.
6. Open the agent's listening port (default 9999) in the host firewall
   (firewalld / ufw / nftables) — without this, the master cannot reach
   `https://<addr>:9999/api/v2/health/check` and enrollment will leave the
   node in `Pending` indefinitely.

## Required: handle pre-staged bootstrap dir

The master writes `/etc/1panel/bootstrap/` over SSH **before** kicking off
the installer. The installer must:

1. Detect that the directory exists.
2. Not clobber it.
3. Skip the interactive prompts (port, username, password) that
   `install.sh` shows in interactive mode — pull values from
   `/etc/1panel/bootstrap/port` instead.

## Required: chown / chmod consistency

The agent runs as root and reads `/etc/1panel/bootstrap/*` mode 0600.
Installer must not relax those permissions.

## Required: 1pctl modes

`1pctl user-info`, `1pctl reset`, etc. are master-only commands. In agent
mode either hide them, or have them print a clear "this command is only
available on the master" message.

## Nice-to-have

- Architecture detection so the master's `EnrollViaSSH` can pre-flight
  reject (e.g. master on amd64, slave on arm64 — should still work, but
  surface a warning).
- Surface the bootstrap version so we can refuse cross-major-version
  enrollment in the SSH flow before the cert is even minted.

## Reference: bootstrap files written by master

| Path | Content | Mode |
|------|---------|------|
| `/etc/1panel/bootstrap/scope` | literal `slave` | 0600 |
| `/etc/1panel/bootstrap/port` | decimal port, e.g. `9999` | 0600 |
| `/etc/1panel/bootstrap/proxy_id` | 32-char alnum secret | 0600 |
| `/etc/1panel/bootstrap/server.crt` | PEM, signed by master CA, SAN=node.Name + addr | 0600 |
| `/etc/1panel/bootstrap/server.key` | EC private key PEM | 0600 |
| `/etc/1panel/bootstrap/root.crt` | master CA cert PEM (for ClientCAs) | 0600 |
| `/etc/1panel/agent.sock` | NOT created in slave mode (agent listens on TCP+mTLS) | — |
| `/etc/1panel/.nodeProxyID` | copy of `bootstrap/proxy_id` written by agent at first boot | 0600 |

After `xpack.LoadNodeInfo()` consumes the bootstrap dir, the installer may
optionally `rm -rf /etc/1panel/bootstrap/` — the agent has already
persisted the values into its setting table (encrypted) and
`/etc/1panel/.nodeProxyID`.
