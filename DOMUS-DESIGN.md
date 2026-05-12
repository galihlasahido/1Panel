# Domus — Architecture & Product Design

Status: **DRAFT v0.3** (2026-05-12) — Codename **Domus** (Latin "house" — root of "domain")
Audience: founding engineers + early adopters

## Changelog
- **v0.3 (2026-05-12)** — Rebranded `1Panel-Host` → **Domus** (Latin
  for "house"; root of "domain"). Solo-developer reality applied:
  v0.1 ~9-11 months, v1.0 ~3-4 years. **WAF + curated app catalog
  moved from v0.4 → v0.5** (deferred) to keep MVP shippable. **cpmove
  import promoted from v0.3 → v0.1** (per operator decision: need
  migration story from day 1). **Rocky/Alma support deferred from
  v0.1 → v0.3** (Ubuntu LTS only for v0.1). Repo strategy starts with
  `libpanel` extraction PR against current 1Panel repo before forking
  `domus` project.
- **v0.2 (2026-05-12)** — Locked open-question answers from Section 16:
  mail smart-relay; WAF in-tree (Coraza/OWASP CRS); curated tenant
  app catalog; IPv6 dual-stack from v0.1; HA master deferred to v1.x.
  PostgreSQL alongside MySQL/MariaDB as first-class. Sections 20-22
  added (DB strategy, WAF design, app catalog design).
- **v0.1 (2026-05-12)** — Initial draft.

A separate product from 1Panel core. Where 1Panel-core is a single-admin
VPS management panel optimized for AI agents and modern self-hosting,
Domus is a **multi-tenant shared-hosting control panel** in the
cPanel/Plesk lineage — designed for hosting providers to run on Linux
servers with strict tenant isolation, email, DNS, and lifecycle tooling
out of the box.

## 1. Product positioning

| Axis | 1Panel (existing) | **Domus (this doc)** | cPanel/Plesk |
|---|---|---|---|
| Primary user | VPS owner / homelab | **Hosting provider** | Hosting provider |
| Tenants per install | 1 (admin) | **Many (10–1000+)** | Many |
| Trust model | Root-equivalent admin | **Sandboxed tenants** | Sandboxed tenants |
| App marketplace | Yes (165+ apps via Docker) | **No** — tenants don't install OS apps | No (Softaculous addon) |
| AI agents | Yes (Ollama, OpenClaw) | **No** | No |
| OS-level isolation | None needed | **chroot + cgroups + quota** | CageFS/LVE (proprietary) |
| Email stack | None | **Postfix + Dovecot + Rspamd** | Exim/cPanel-mail |
| Authoritative DNS | None | **PowerDNS** | BIND/PowerDNS |
| Multi-node | Yes (managed slaves) | **Yes (account placement)** | Add-on (WHM) |
| License | OSS (GPL-3.0) | **OSS (GPL-3.0)** | Commercial |
| Target market | Self-hosters, agencies | **Hosting providers tier 1-3** | All tiers |

Strategic differentiator vs cPanel/Plesk: **free, modern UX, container-aware,
multi-node native, OSS**. Not trying to match 25 years of feature breadth —
target 80% feature parity at v1.0 covering the workflows hosting
providers actually use daily.

Strategic non-goals (v1):
- Not a replacement for CageFS-grade kernel-level isolation (no proprietary
  kernel modules; tenants are trusted to the level Linux user accounts
  allow, hardened by chroot + cgroups + quota).
- Not a billing system. Integration with WHMCS/Blesta/Hostbill via webhook
  API only.
- Not Windows hosting. Linux only.
- Not a domain registrar. Integration with registrar APIs only.

## 2. Tech stack decisions

| Component | Choice | Rationale |
|---|---|---|
| Backend language | Go 1.25+ | Reuse 1Panel ecosystem; matches `libpanel` shared module |
| HTTP framework | Gin | Reuse from 1Panel |
| ORM | GORM | Reuse from 1Panel |
| Database (panel state) | PostgreSQL 15+ | More appropriate for multi-tenant query patterns than SQLite; supports row-level security for defense-in-depth |
| Cache | Redis 7+ | Session sharing across nodes, rate limit, mailbox auth lookup |
| Web frontend | Vue 3 + Vite + Element Plus | Reuse from 1Panel; consistent operator UX |
| Web server (tenant sites) | Nginx | Industry standard, reuse vhost templates from 1Panel |
| Web Application Firewall | **Coraza** + **OWASP CRS 4.x** | Go-native (matches our stack), embeds in Nginx via spoa or as Caddy plugin, ModSecurity v3 rule-compatible. Section 21. |
| MTA | Postfix | Mature, secure defaults, deep Linux integration |
| IMAP/POP/LMTP | Dovecot | LMTP integration with Postfix, Sieve+ManageSieve for filters |
| Anti-spam | Rspamd | Modern replacement for SpamAssassin; native DKIM/ARC; HTTP API |
| Webmail | SnappyMail | Modern fork of RainLoop, IMAP-only, fast, ~10MB |
| DNS server | PowerDNS Authoritative | API-driven (REST), DNSSEC built-in, SQL backend |
| FTP server | Pure-FTPD | Virtual users in SQL backend, TLS, chroot |
| Tenant DB: MySQL | **MySQL 8.0 / MariaDB 10.11** | Per-tenant user + DB; `resource_group` for CPU cap; `max_user_connections` per role. Detail at Section 20. |
| Tenant DB: PostgreSQL | **PostgreSQL 16+** | Per-tenant role + DB; PgBouncer for connection pooling; `statement_timeout` and `idle_in_transaction_session_timeout` caps. Detail at Section 20. |
| App catalog | **In-tree YAML manifests** + tenant-uid installer | WordPress, Joomla, Drupal, Ghost, NextCloud-lite, etc. Runs as tenant UID, NOT in Docker. Section 22. |
| PHP runtime | PHP-FPM 8.1+ | Pool per tenant for isolation |
| Network stack | **IPv4 + IPv6 dual-stack** | All daemons bind both families; vhost templates emit `listen [::]:443 ssl http2 ipv6only=off`; DNS produces A + AAAA by default. |
| Process supervision | systemd | Native Linux; per-tenant slices for cgroup limits |
| Disk quota | XFS project quota | More flexible than ext4 user quota; works with per-dir quotas |
| Resource limits | cgroups v2 + systemd slices | CPU/RAM/IO/pids per tenant |
| Sandbox | chroot + bind mounts + user namespaces | OS-native; no proprietary kernel module |
| Log aggregation | rsyslog → JSON files | Reuse 1Panel log viewer pattern |
| Stats per domain | GoAccess (real-time) | Modern, gzip-aware, JSON output |
| Backup | restic | De-duplicating, encrypted, S3-native, OSS |
| TLS | Let's Encrypt + ZeroSSL via lego (Go) | Reuse from 1Panel |

## 3. Repo strategy

Solo developer + fork workflow (current 1Panel repo origin =
`github.com/galihlasahido/1Panel`, upstream = `github.com/1Panel-dev/1Panel`).
Two-stage approach:

**Stage 1 — libpanel extraction PR against 1Panel current** (NOW):

```
github.com/galihlasahido/1Panel/        (this repo, fork of 1Panel-dev/1Panel)
├── agent/                              (existing — also lightly refactored)
├── core/                               (existing — refactored to import libpanel)
├── frontend/                           (existing)
└── libpanel/                           (NEW)
    ├── encrypt/                        # moved from {agent,core}/utils/encrypt
    ├── pki/                            # moved from core/utils/pki
    ├── ssh/                            # moved from core/utils/ssh
    └── (others added incrementally)
```

libpanel starts as a directory inside 1Panel module (`github.com/1Panel-dev/1Panel/libpanel/...`)
to avoid go.work complexity for solo dev. This deduplicates the
encrypt code currently duplicated between agent and core. Net win
for 1Panel itself even before Domus exists.

**Stage 2 — Domus repo created** (after Stage 1 lands):

```
github.com/galihlasahido/domus/                  (new repo)
├── core/                               # Master node binary
├── agent/                              # Slave node binary
├── frontend/                           # Vue UI (separate from 1Panel's)
├── installer/                          # install.sh
├── migrations/                         # SQL migrations
└── docs/
```

At this point libpanel is promoted to its own Go module
(`github.com/galihlasahido/libpanel`) and both 1Panel + Domus consume
via `go get`. The promotion is a small additional refactor — defer
until Domus actually needs to import it.

**Files in scope for Stage 1 libpanel extraction**:
- `core/utils/encrypt/*.go` → `libpanel/encrypt/*.go` (single source)
- `agent/utils/encrypt/*.go` → DELETED, agent imports `libpanel/encrypt`
- `core/utils/pki/*.go` → `libpanel/pki/*.go`
- `core/utils/ssh/*.go` → `libpanel/ssh/*.go`
- Multi-node mTLS material from `core/utils/xpack/node_proxy.go` stays
  in core for now (still has tight coupling to model + repo); extract
  to libpanel later when Domus needs it.

This refactor first benefits 1Panel (less duplication agent↔core) and
gives Domus a head start.

## 4. Threat model

The single hardest property of a multi-tenant panel: **tenant A cannot read,
modify, or denial-of-service tenant B's data**.

Top attack vectors and mitigations:

| Vector | Mitigation |
|---|---|
| **Tenant uploads PHP that reads `/etc/shadow`** | chroot + open_basedir + disable_functions + AppArmor/SELinux profile |
| **Tenant uploads PHP that connects to internal services** | nftables outbound policy per-tenant: block 127.0.0.0/8 except own services, block RFC1918 |
| **Tenant fills disk → DoS others** | XFS project quota; hard fail on overflow, alert admin |
| **Tenant forks bombs → DoS others** | systemd slice `TasksMax`, cgroups `pids.max` |
| **Tenant CPU-busy-loops → DoS others** | systemd slice `CPUQuota`, `IOWeight` |
| **Tenant reads other tenant's files via symlink** | chroot prevents traversal; nginx `disable_symlinks if_not_owner` |
| **Tenant uses misconfigured cron to escalate** | Cron runs as tenant user; sanity-check shell metachars in UI |
| **Tenant accesses MySQL across schemas** | `GRANT ... ON tenant_X.* TO tenant_X` only, no wildcard |
| **Tenant reads /proc to enumerate other processes** | `hidepid=2` mount option on /proc |
| **Tenant DNS-amplification via panel API** | PowerDNS rate limit, DNS recursion disabled |
| **Tenant exfils through mail relay** | Postfix smtpd_sender_restrictions; per-tenant send rate limit |
| **Tenant via FTP escapes chroot** | Pure-FTPD with `ChrootEveryone` + `BrokenClientsCompatibility no` |
| **Admin compromise via panel SQL injection** | Parameterized queries enforced; gosec in CI |
| **Admin compromise via panel XSS** | Vue auto-escapes; CSP header strict; DOMPurify on user-content render |
| **Panel takeover via session hijack** | HttpOnly + Secure + SameSite=Strict cookies; rotate session on privilege change |
| **Backup tampering** | restic with encrypted repo + immutable S3 object lock optional |

**Audit cadence**: third-party security audit before v1.0 stable. Bug
bounty after first 100 hosting deployments.

## 5. Tenant data model

PostgreSQL schema (simplified):

```sql
-- The root admin (the hosting provider operator)
CREATE TABLE operators (
    id BIGSERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,           -- Argon2id
    role TEXT NOT NULL,                    -- 'admin' | 'reseller'
    parent_id BIGINT REFERENCES operators(id),  -- for reseller hierarchy
    quota_accounts INT,                    -- max tenant accounts this reseller can create
    suspended BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- One tenant = one "hosting account" = one Linux user on the slave node
CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,
    operator_id BIGINT NOT NULL REFERENCES operators(id),   -- which reseller owns this
    node_id BIGINT NOT NULL REFERENCES nodes(id),           -- which slave this account lives on
    username TEXT UNIQUE NOT NULL,                          -- Linux user, e.g. "tenant_a3f9"
    unix_uid INT UNIQUE NOT NULL,                           -- assigned at create time
    email TEXT NOT NULL,                                    -- contact email (NOT login)
    plan_id BIGINT REFERENCES plans(id),                    -- resource plan
    status TEXT NOT NULL,                                   -- 'active' | 'suspended' | 'terminated'
    home_dir TEXT NOT NULL,                                 -- e.g. /home/tenant_a3f9
    created_at TIMESTAMPTZ DEFAULT NOW(),
    suspended_at TIMESTAMPTZ,
    terminated_at TIMESTAMPTZ
);

CREATE TABLE plans (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    disk_quota_mb BIGINT,           -- 0 = unlimited
    inode_quota BIGINT,
    bandwidth_mb BIGINT,            -- monthly
    cpu_quota_pct INT,              -- cgroup CPUQuota %
    memory_max_mb BIGINT,
    tasks_max INT,
    email_accounts_max INT,
    databases_max INT,
    ftp_accounts_max INT,
    subdomains_max INT,
    addon_domains_max INT
);

-- A "site" attached to an account; can be primary domain, addon, subdomain, parked
CREATE TABLE sites (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,             -- 'primary' | 'addon' | 'subdomain' | 'parked'
    domain TEXT UNIQUE NOT NULL,
    doc_root TEXT NOT NULL,         -- absolute path inside chroot
    php_version TEXT,               -- e.g. '8.2'; NULL = no PHP
    ssl_id BIGINT REFERENCES ssl_certs(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- DNS zones managed by PowerDNS, mirrored here for UI fast path
CREATE TABLE dns_zones (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT UNIQUE NOT NULL,      -- e.g. example.com.
    dnssec_enabled BOOLEAN DEFAULT FALSE
);

CREATE TABLE mailboxes (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    address TEXT UNIQUE NOT NULL,   -- e.g. info@example.com
    password_hash TEXT NOT NULL,    -- SHA512-CRYPT or BCRYPT; checked by Dovecot SQL auth
    quota_mb BIGINT,
    sieve_rules JSONB
);

CREATE TABLE mail_aliases (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source TEXT NOT NULL,           -- forwarder source
    destination TEXT NOT NULL,
    UNIQUE(source, destination)
);

CREATE TABLE databases (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,             -- 'mysql' | 'postgres'
    db_name TEXT NOT NULL,
    db_user TEXT NOT NULL,          -- DB user (not Linux user)
    UNIQUE(kind, db_name)
);

CREATE TABLE ftp_accounts (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    username TEXT UNIQUE NOT NULL,  -- virtual FTP user
    password_hash TEXT NOT NULL,
    home_dir TEXT NOT NULL,
    quota_mb BIGINT
);

-- Real-time resource usage (updated by per-node collector)
CREATE TABLE account_usage (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    disk_used_mb BIGINT,
    inode_used BIGINT,
    bandwidth_used_mb BIGINT,
    cpu_seconds_last_hour BIGINT,
    memory_peak_mb BIGINT,
    updated_at TIMESTAMPTZ
);
```

Row-level security: enable Postgres RLS so even a SQL injection in a
reseller-scoped query can't read other resellers' rows.

## 6. OS-level isolation design

Each tenant `tenant_a3f9` gets:

### 6.1 Linux user
```
useradd --uid 5001 --no-create-home --home /home/tenant_a3f9 \
  --shell /usr/sbin/jk_chrootsh tenant_a3f9
```

### 6.2 Home directory layout (chrooted view)
```
/home/tenant_a3f9/                  ← chroot root from tenant's perspective
├── bin/                            ← bind-mount from /var/jail/skel/bin
├── usr/bin/                        ← bind-mount from /var/jail/skel/usr/bin (subset)
├── lib/, lib64/                    ← bind-mount read-only system libs needed
├── etc/                            ← minimal: passwd, group, resolv.conf (templated)
├── tmp/                            ← private tmpfs, size-limited
├── public_html/                    ← primary site doc root (real path)
├── public_ftp/
├── mail/                           ← Maildir
├── domains/<addon>/public_html/    ← addon domain sites
├── etc/php-fpm.d/                  ← per-pool config
├── logs/
└── backup/
```

The bind mounts are set up once via systemd `mount` unit; tenants
cannot break out.

### 6.3 systemd slice for resource limits

`/etc/systemd/system/tenant-tenant_a3f9.slice`:

```ini
[Unit]
Description=Slice for tenant tenant_a3f9
DefaultDependencies=no

[Slice]
CPUWeight=100
CPUQuota=50%
MemoryMax=512M
TasksMax=200
IOWeight=100
```

Every PHP-FPM pool, cron job, SSH session etc. for that tenant launches
into this slice via `systemd-run --slice=tenant-tenant_a3f9.slice ...`.

### 6.4 PHP-FPM pool template

`/etc/php-fpm.d/tenant_a3f9.conf`:

```ini
[tenant_a3f9]
user = tenant_a3f9
group = tenant_a3f9
listen = /run/php-fpm/tenant_a3f9.sock
listen.owner = tenant_a3f9
listen.group = www-data
chroot = /home/tenant_a3f9
chdir = /
php_admin_value[open_basedir] = /public_html:/tmp
php_admin_value[disable_functions] = exec,passthru,shell_exec,system,proc_open,popen,...
php_admin_value[upload_tmp_dir] = /tmp
slowlog = /home/tenant_a3f9/logs/php-slow.log
```

Nginx vhost talks to this socket. PHP code runs as tenant UID inside
chroot. No access to other tenants' files.

### 6.5 Disk quota (XFS project quota)

```bash
# Mount /home with prjquota (one-time, /etc/fstab):
UUID=... /home xfs rw,noatime,nodiratime,prjquota 0 0

# Per-tenant project:
echo "5001:/home/tenant_a3f9" >> /etc/projects
echo "tenant_a3f9:5001" >> /etc/projid
xfs_quota -x -c 'project -s tenant_a3f9' /home
xfs_quota -x -c 'limit -p bhard=5g ihard=100000 tenant_a3f9' /home
```

### 6.6 Network policy (nftables)

```nft
table inet tenant_isolation {
    set tenant_uids {
        type uid; flags interval;
    }
    chain output {
        type filter hook output priority 0;
        # Block tenant access to RFC1918 except own loopback services
        meta skuid @tenant_uids ip daddr 10.0.0.0/8 drop
        meta skuid @tenant_uids ip daddr 172.16.0.0/12 drop
        meta skuid @tenant_uids ip daddr 192.168.0.0/16 drop
        # Allow localhost for own DB
        meta skuid @tenant_uids ip daddr 127.0.0.1 accept
    }
}
```

### 6.7 Distro differences

| Concern | Ubuntu 22.04/24.04 | Rocky/Alma 9 |
|---|---|---|
| Package manager | apt | dnf |
| MAC | AppArmor | SELinux (enforcing) |
| Firewall | ufw / nftables | firewalld (nftables backend) |
| PHP path | /usr/lib/php-fpm | /usr/sbin/php-fpm |
| Postfix conf | /etc/postfix | /etc/postfix |
| Dovecot conf | /etc/dovecot | /etc/dovecot |
| systemd | yes | yes |
| Default Python | 3.10/3.12 | 3.9 |

Installer detects distro via `/etc/os-release` and selects appropriate
package set + paths. Single binary, distro-aware runtime.

SELinux complication: must ship policy module
`1panel-host.pp` with allow rules for our daemons. Plan: 2-3 weeks
focused effort, test on Rocky 9 from day 1.

## 7. Auth & permissions

Three actor types:

1. **Operator (admin)** — root of the panel. Manages nodes, resellers,
   plans, system-level config. Full power.
2. **Operator (reseller)** — sub-operator with quota on accounts. Can
   create/manage their own tenant accounts but not see others'.
3. **Account (tenant)** — end user of hosting. Logs into the panel,
   manages their own sites/email/DB.

Authentication:
- All three use Argon2id password hash.
- Session in Redis, keyed by random 256-bit token in HttpOnly+Secure cookie.
- Optional MFA: TOTP (RFC 6238) or WebAuthn/passkey.
- Per-actor type rate limit on login (5 fails / 15 min lockout).

Authorization:
- RBAC matrix encoded in middleware: `permission_map[action][role] = bool`
- Tenant API requests go through a tenant-scoping middleware that
  attaches `account_id` from session, rejects any request that touches
  resources outside that account.
- Defense-in-depth: Postgres RLS as second gate.

API surface (versioned `/api/v1/`):
```
/api/v1/admin/...        # operator admin
/api/v1/reseller/...     # operator reseller
/api/v1/account/...      # tenant
/api/v1/public/...       # webmail login, reset password, etc.
```

## 8. Email stack architecture

```
                  (incoming SMTP, port 25)
                          |
                          v
                  +---------------+
                  |    Postfix    |
                  | smtpd (25)    |
                  | submission(587)|
                  +---+-----------+
                      |
            +---------+-----------+
            |                     |
            v                     v
       Rspamd HTTP API       (auth: Dovecot SASL)
       (spam/virus check)
            |
            v
       Postfix queue
            |
            v
       LMTP delivery
            |
            v
       Dovecot LMTP socket
            |
            v
       /home/<tenant>/mail/<address>/Maildir
            |
        (IMAP read by client / SnappyMail webmail)
```

Auth flow:
- Mailbox CRUD in panel → row in `mailboxes` table.
- Postfix uses `proxy:pgsql:` lookup for virtual_mailbox_domains,
  virtual_mailbox_maps, virtual_alias_maps.
- Dovecot uses SQL auth driver pointed at `mailboxes` table.
- No `/etc/passwd` mail users — all virtual.

DKIM:
- Rspamd handles DKIM signing on outbound.
- One key pair per domain, generated at zone creation.
- Public key exposed as DNS TXT via PowerDNS.

DMARC/SPF:
- Templates auto-generated when zone created.
- Operator can override per-zone.

TLS:
- Per-domain certs via Domus's lego integration.
- Postfix smtpd_tls_chain_files keyed per domain via SNI.

## 9. DNS stack architecture

PowerDNS Authoritative with PostgreSQL backend (separate schema from panel
DB or same, TBD — leaning same for transactional consistency).

```
                 (recursive query from internet)
                          |
                          v
                  Master node :53
                  PowerDNS (auth)
                  ↑ AXFR/IXFR
                  |
             (zone changes)
                  |
              Panel API
                  ↑
              (admin / tenant edit)
```

Slave nodes can be configured as secondaries for redundancy.

DNSSEC:
- Enable per-zone toggle in UI.
- PowerDNS handles key rotation.
- DS record presented to operator for upstream registrar config.

Zone editor UI:
- Record types: A, AAAA, CNAME, MX, TXT, SRV, CAA, NS.
- DNSSEC records hidden (managed by PowerDNS).
- Validation: prevent CNAME at apex, etc.
- Templates: "standard mail setup" applies MX + SPF + DKIM placeholder.

## 10. Multi-node coordination

Reuse 1Panel's mTLS infrastructure (PKI + CA + per-node client certs +
node health cron) from `libpanel/mtls`.

Differences from 1Panel:
- Many more nodes typical (hosting provider may have 5-50 nodes).
- Accounts are placed on specific node; this is THE key new concept.

Account placement model:

```sql
CREATE TABLE nodes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    addr TEXT NOT NULL,
    port INT DEFAULT 9999,
    role TEXT NOT NULL,                    -- 'master' | 'web' | 'mail' | 'dns' | 'db' | 'mixed'
    status TEXT NOT NULL,
    version TEXT,
    -- Capacity for placement decisions
    capacity_disk_gb INT,
    capacity_memory_gb INT,
    capacity_account_max INT,
    -- Real-time, updated by cron
    used_disk_gb INT,
    used_memory_gb INT,
    used_account_count INT,
    last_check TIMESTAMPTZ
);
```

Placement policy at account create:
1. If `node_id` specified by operator → use it (manual).
2. Else find node with `role` containing requested service, lowest
   resource utilization, capacity available.
3. Fail clearly if no node has capacity.

Roles are tags. Single physical node can have multiple roles (small
hosting provider with 1 box runs all). Large provider separates: web
nodes (Nginx+PHP), mail nodes (Postfix+Dovecot+Rspamd), DNS nodes
(PowerDNS), DB nodes (MySQL).

Cross-node operations:
- **Login**: tenant logs into panel via master URL. Master proxies
  account-specific operations to the node hosting that account (via
  `CurrentNode: <node-name>` header — exact pattern as 1Panel today).
- **DNS**: master always serves DNS API directly; pushes zone data to
  authoritative DNS nodes via PowerDNS API.
- **Mail (resolved per decision 1)**: MX points to mail node; mail
  node owns IMAP/POP storage and delivery. Each web node runs a
  minimal Postfix configured with `relayhost = [mail-node-ip]:25`,
  `smtp_tls_security_level = encrypt`, and SASL auth. Tenant PHP
  `mail()`, queued WordPress emails, etc. all go through the local
  Postfix → smart-relayed to the mail node. Per-tenant outbound rate
  limit enforced at the mail node's smtpd via `smtpd_client_restrictions`
  + `policyd-rate-limit`. No NFS, no shared mailbox storage.
- **Backup**: each node backs up locally to S3 via restic.

Account migration between nodes:
1. Mark account suspended.
2. restic backup of /home/<tenant> to S3.
3. On target node: useradd, restore from S3, set up quota/slice/PHP-FPM.
4. DB dump from source MySQL, import on target MySQL (if DB roles differ).
5. Update sites' DNS A records via PowerDNS API.
6. Update `accounts.node_id`.
7. Activate, teardown source.

Estimated migration time: ~1 min per GB of data.

## 11. Account lifecycle states

```
        operator creates
              |
              v
         provisioning  (transient)
              |
         (atomic) ↓ ↓ success
              |          \
              |           v
              |        active ←→ suspended
              |          |
              |          v
              |     terminating  (transient)
              |          |
              |          v
              |      terminated
              |
              ↓ failure
          provisioning_failed
```

Provisioning is an idempotent saga across many subsystems:
1. Allocate unix UID.
2. Create unix user, /home/<user>.
3. Set up chroot bind mounts.
4. Create systemd slice.
5. Create PHP-FPM pool(s) per requested PHP version.
6. Set XFS project quota.
7. Apply nftables uid set membership.
8. Create primary site with empty doc root.
9. Create DNS zone for primary domain.
10. Issue SSL cert (Let's Encrypt).
11. Create Postfix/Dovecot mailboxes (none initially).
12. Create MySQL/Postgres user + first DB if requested.
13. Create FTP virtual user (matches panel login).
14. Insert into `accounts` table with `status='active'`.

If any step fails, run rollback in reverse order. Use a saga
coordinator pattern; persist state machine to DB so a crashed master
can resume.

Suspension keeps data, just disables login + nginx vhost + email +
DNS resolution to the site (return 503 stub page).

Termination has retention: 30-day soft-delete by default. Permanent
purge requires explicit operator action with confirmation.

## 12. Resource accounting

Per-tenant metrics collector runs on each node every 60s:

| Metric | Source |
|---|---|
| Disk used | `xfs_quota -x -c 'report' /home` (XFS) or `quotacheck` |
| Inode used | Same |
| Bandwidth (web) | Nginx access log parsing (rotated daily, summed) |
| Bandwidth (mail) | Postfix log parsing |
| CPU seconds | cgroups `cpu.stat` per slice |
| Memory peak | cgroups `memory.peak` per slice |
| Email count | `SELECT count(*) FROM mailboxes WHERE account_id = ?` |
| DB count | Same for `databases` |

Push to master `account_usage` table via mTLS POST every 5 min.

Overage handling:
- Soft limit: send warning email to operator + tenant.
- Hard limit (disk, inode): quota system enforces. Hard limit (CPU,
  memory): systemd throttles transparently.
- Bandwidth overage: operator-configurable policy — suspend, slow, or
  alert only.

## 13. UI structure

Three top-level personas, three skins:

```
admin.host.example.com         # operator (admin) panel
reseller.host.example.com      # reseller dashboard
panel.tenant.example.com       # tenant (end-user) panel
mail.tenant.example.com        # SnappyMail webmail
```

Single Vue app, different routes/layouts gated by role.

Operator UI top-level menus:
- Dashboard (nodes, account count, alerts)
- Accounts (CRUD, suspend, migrate, terminate)
- Resellers
- Plans
- Nodes (reuse 1Panel UI from current work)
- DNS Zones
- Server (settings, OS info, services)
- Backups
- Logs

Tenant UI top-level menus (cPanel-inspired):
- Home (resource usage dashboard)
- Files (file manager, FTP accounts, disk usage analyzer)
- Domains (subdomains, addon, parked, redirects)
- Email (mailboxes, forwarders, autoresponders, mailing lists, deliverability)
- **Databases** (MySQL/MariaDB + PostgreSQL CRUD, phpMyAdmin/phpPgAdmin link, per-db user GRANT editor, remote-access allow list)
- Metrics (visitors, bandwidth, errors)
- Software (PHP version selector, Cron, SSH access toggle, Git)
- **Apps** (curated catalog: WordPress/Joomla/Drupal/Ghost/etc. with 1-click install scoped to tenant UID — Section 22)
- **Security** (SSL/TLS, IP blocker, hotlink protection, **WAF toggle + log viewer + custom rules** — Section 21)
- Advanced (DNS Zone, custom error pages, raw access logs)

## 14. Migration from cPanel

`cpmove-<user>.tar.gz` is cPanel's documented account export format.
Structure (subset):

```
cpmove-username/
├── homedir.tar             # /home/username
├── mysql/                  # *.sql dumps + grants
├── mysql.sql
├── homedir/
├── .htpasswds/
├── mail/                   # Maildir+sieve
├── cp/username             # account meta
├── ssl/
├── dnszones/<domain>.db    # BIND-format zone files
└── *.tar (compressed homes)
```

Import flow:
1. Operator uploads `cpmove-*.tar.gz` via UI or CLI.
2. Validate format, extract to `/tmp/cpmove-import-<uuid>/`.
3. Create new account scaffolding (Domus structure).
4. Restore homedir to new tenant /home (rsync, preserve permissions).
5. Restore MySQL: create DBs, import dumps with renamed users.
6. Restore mailboxes: parse Maildir, create rows in `mailboxes` table.
7. Restore DNS: parse BIND zone files, insert PowerDNS rows.
8. Re-issue SSL via Let's Encrypt (cPanel certs may not transfer cleanly).
9. Update DNS A records to new server IP.
10. Report what was successfully migrated and what was skipped.

NOT supported in v1: Plesk format, DirectAdmin format. Add later.

## 15. Phased roadmap

**Calibration note (solo dev, full-time-ish)**: original estimates were
for tim 3-5. Solo multiply by ~2.5. Numbers below are solo
calendar-month estimates. v1.0 ETA: **end of year 3 to early year 4**.

### Stage 0 — libpanel extraction ✅ DONE (branch `libpanel-extraction`, 2026-05-12)

3 commits landed:
- `35c127d1f` — pki extracted (Stage 0a)
- `480705f4f` — encrypt extracted with KeyProvider pattern (Stage 0b)
- `4de2d9e0d` — ssh extracted with Logger + ProxyResolver (Stage 0c)

- [x] Created `libpanel/` Go module at repo root (`module github.com/1Panel-dev/1Panel/libpanel`).
- [x] Moved `core/utils/pki/` → `libpanel/pki/` (zero global deps; 7 tests pass standalone).
- [x] Moved `core/utils/encrypt/` AND `agent/utils/encrypt/` → `libpanel/encrypt/` (single source of truth; KeyProvider pattern lets core + agent inject their own file/CONF/DB resolution policy). 31 import paths updated. Both module test suites pass.
- [x] Moved `core/utils/ssh/` → `libpanel/ssh/` (Logger + ProxyResolver injection). 10 core imports updated. `agent/utils/ssh/` left as-is (different feature set, no deduplication target).
- [x] Added `replace github.com/1Panel-dev/1Panel/libpanel => ../libpanel` to both `core/go.mod` and `agent/go.mod` so `cd <mod> && go build` works without `go.work`.
- [x] Bumped `golang.org/x/crypto` v0.50→0.51 and `golang.org/x/net` v0.53→0.54 in core + agent to match libpanel.
- [x] Init wiring: `core/init/encrypt`, `core/init/ssh`, `agent/init/encrypt` register providers at startup before migrations run.
- [x] Verified: core + agent build clean, libpanel encrypt + pki test suites green.
- [ ] Future (defer until needed): dedupe agent/utils/ssh into libpanel/ssh; promote `RandBytes` + base64 helpers; sanity test on real VM.

**Outcome**: 1Panel now has a deduplicated shared module ready to be
imported by Domus repo when created. Three security-critical
components (encrypt, pki, ssh) live in one place with explicit
injection points for host-app context.

### v0.1 — Foundation (months 2-12, ~10-11 months solo)
Goal: dogfood-able internally; provision a tenant on one node manually.

- Domus repo scaffolding (core + agent + frontend) — 1 week
- Operator auth + accounts table + plans (Postgres) — 4 weeks
- OS-level isolation MVP: unix user, chroot bind mounts, systemd slice,
  PHP-FPM pool template, XFS project quota — 12 weeks
- Tenant auth scoped to account (RBAC matrix + RLS) — 3 weeks
- Primary site (Nginx vhost) with PHP, IPv6 dual-listen — 2 weeks
- Tenant file manager (chrooted, ZipSlip-safe extract) — 2 weeks
- Tenant MySQL/MariaDB per-tenant (Section 20.2 + 20.4 baseline) — 3 weeks
- Tenant PostgreSQL per-tenant via PgBouncer (Section 20.3 + 20.5 baseline) — 3 weeks
- IPv6 dual-stack on all daemons — 1 week
- **cPanel `cpmove-*.tar.gz` import (subset)** — 6 weeks (promoted from v0.3)
- Distro support: Ubuntu 22.04 + 24.04 only — covered above
- Internal dogfood + bug bash — 2 weeks
- **Out of scope for v0.1**: email, DNS, multi-node, billing, WAF, app catalog, Rocky/Alma

### v0.2 — Email + DNS (months 13-19, ~7 months solo)
- Postfix + Dovecot + Rspamd stack with smart-relay support — 12 weeks
- SnappyMail webmail integration — 2 weeks
- PowerDNS + zone editor UI — 6 weeks
- DKIM/SPF/DMARC auto-config — 2 weeks
- Mail log viewer + Rspamd metrics UI — 2 weeks
- Per-tenant outbound mail rate limit — 2 weeks
- Internal dogfood + bug bash — 2 weeks

### v0.3 — Multi-node + Rocky support (months 20-24, ~5 months solo)
- Multi-node coordination (reuse libpanel mTLS) — 3 weeks
- Account placement on node creation + role tagging — 2 weeks
- Account migration between nodes (restic-based) — 5 weeks
- Suspend/terminate/restore flows + retention — 3 weeks
- Backup integration (restic + S3) — 3 weeks
- Rocky/Alma 9 support: SELinux policy module, dnf packages, firewalld — 4 weeks
- Internal dogfood — 2 weeks

### v0.4 — Polish + billing (months 25-30, ~6 months solo)
- Resource accounting + overage policies — 6 weeks
- Reseller hierarchy + UI — 6 weeks
- Tenant UI feature parity items (subdomains, addons, parked,
  redirects, error pages, hotlink protection, IP blocker) — 8 weeks
- Visitor stats integration (GoAccess) — 3 weeks
- Cron job UI per tenant — 2 weeks
- SSH access toggle — 2 weeks
- WHMCS-protocol billing webhooks — 4 weeks
- Database tier hardening + per-tenant optimization deep dive — 3 weeks (Section 20)

### v0.5 — WAF + App catalog + Hardening (months 31-39, ~9 months solo)
**WAF and app catalog deferred here from v0.4 per solo-scope-trim decision.**

- WAF (Coraza + OWASP CRS) integration via SPOA — 10 weeks (Section 21)
- Curated app catalog v1 (8-10 apps incl. WordPress, Joomla, Drupal,
  Ghost, phpBB, MediaWiki, NextCloud-lite, Matomo) — 10 weeks (Section 22)
- Third-party security audit — 4 weeks
- Bug fixes from audit — 6 weeks
- Performance test: 1000 tenants per node — 4 weeks
- Documentation site — 6 weeks
- Beta program with 3-5 real hosting providers — ongoing

### v1.0 — Stable (month 40+)
- Final fixes from beta
- Migration tool maturity
- Marketing launch

**TL;DR solo timeline:**
| Stage | Calendar months | Cumulative |
|---|---|---|
| Stage 0 (libpanel) | 1 | 1 |
| v0.1 | 11 | 12 |
| v0.2 | 7 | 19 |
| v0.3 | 5 | 24 |
| v0.4 | 6 | 30 |
| v0.5 | 9 | 39 |
| v1.0 ship | 1+ | ~40 |

That's **~3.5 years solo for v1.0 production-grade**. If you can ship
v0.1 + dogfood for 6-12 months before continuing, that's a healthier
cadence and may attract early contributors who shorten subsequent
phases.

## 16. Open questions — resolved 2026-05-12

Decisions captured here are locked-in for v0.1. Changing them requires
a new design-doc version + explicit migration plan.

1. **Mail storage in multi-node** — **RESOLVED: (a) smart-relay**.
   Web nodes run a minimal Postfix configured as `relayhost` →
   designated mail node. PHP `mail()` works transparently; web tenants
   don't see mail infrastructure. NFS rejected (operational
   complexity). Per-tenant outbound rate limit enforced at the
   smart-relay smtpd stage. See Section 10.

2. **MySQL on separate node** — **RESOLVED: same-node default + remote
   override**. v1.0 ships with DB on the same node as web for
   simplicity; provider can point at remote MySQL/Postgres via plan
   config. Section 20 covers per-tenant patterns for both.

3. **Reseller UI** — **RESOLVED: subdomain pattern**. Whitelabel is
   a v1.x feature (theme config, custom domain CNAME).

4. **Pricing / billing** — **RESOLVED: webhook hooks only**. WHMCS /
   Blesta / Hostbill protocol-compatible endpoints; first-party
   billing is a separate product.

5. **WAF** — **RESOLVED: build in OSS**. Coraza + OWASP CRS 4.x
   embedded as Nginx filter via SPOA or sidecar. Detection-only mode
   default for new sites; provider can flip to enforce. Per-site
   toggle, custom rule editor, IP-block escalation. See Section 21.

6. **Container apps for tenants** — **RESOLVED: curated catalog**.
   YAML manifests in `Domus` tree; tenant-uid installer (not
   Docker). Initial catalog: WordPress, Joomla, Drupal, Ghost, phpBB,
   MediaWiki, NextCloud-lite, Matomo. See Section 22.

7. **ZipSlip protection** — **RESOLVED: mandatory**. Path sanitization
   in Go extractor + chroot defense-in-depth.

8. **IPv6 dual-stack** — **RESOLVED: from v0.1**. All daemons bind
   both families; vhost templates dual-listen; DNS auto-creates AAAA
   alongside A.

9. **Hostname uniqueness across operators** — **RESOLVED: global
   uniqueness, FCFS**. DNS layer enforces; UI surfaces clear conflict
   error to second operator who tries to claim.

10. **Disaster recovery / HA master** — **RESOLVED: v1.x**. Single
    master with daily snapshot backup at v1.0. Active-active master
    (Postgres streaming replication + Redis sentinel + leader
    election) targeted at v1.x release.

## 17. Operator decisions log (locked 2026-05-12)

| # | Decision point | Locked answer |
|---|---|---|
| 1 | Project name | **Domus** (Latin "house", root of "domain") |
| 2 | Team size | Solo developer (timeline calibrated to solo in Section 15) |
| 3 | Linux distro for v0.1 | Ubuntu 22.04 + 24.04 LTS only. Rocky/Alma pushed to v0.3 |
| 4 | cpmove import scope | v0.1 (promoted from v0.3 — need migration story from day 1) |
| 5 | Start sequence | **Stage 0: libpanel extraction** as PR against current 1Panel repo |
| 6 | WAF + app catalog | Deferred to v0.5 (de-scoped from v0.4 for solo realism) |
| 7 | Mail multi-node | smart-relay (web→mail node) |
| 8 | DB stack | MySQL/MariaDB + PostgreSQL both first-class |
| 9 | WAF tech | Coraza + OWASP CRS 4.x (Go-native) |
| 10 | App catalog | Curated YAML manifests, tenant-uid installer (no Docker) |
| 11 | IPv6 | Dual-stack from v0.1 |
| 12 | HA master | v1.x (not v1.0) |
| 13 | Repo strategy | Stage 1: libpanel inside 1Panel repo. Stage 2: separate `domus` repo when needed |
| 14 | Git origin | `github.com/galihlasahido/1Panel` (this fork); upstream = `github.com/1Panel-dev/1Panel` |

Pending operator inputs (not blockers for Stage 0):

- [ ] Hardware target for first dogfood node (Ubuntu LTS, cores/GB) —
      can decide before v0.1 month 2.
- [ ] Domain for the eventual hosted demo (e.g. `domus.example.com`) —
      can decide before v0.2.
- [ ] WHMCS protocol research depth (full or just essential hooks?) —
      decide before v0.4 month 28.

## 18. Glossary

- **Operator**: the panel admin (= hosting provider's staff). Has
  master access.
- **Reseller**: a sub-operator with quota on accounts.
- **Account**: a tenant. One Linux user, one /home/<user>, one billing
  unit.
- **Site**: a vhost owned by an account. Primary, addon, subdomain,
  or parked.
- **Plan**: resource quotas template assigned to accounts.
- **Slice**: systemd cgroup-v2 unit for resource limits.
- **Node**: a physical/virtual server in the panel's fleet. Roles:
  master, web, mail, dns, db, mixed.

## 19. Out of scope

Explicit non-features for v1.0 to manage expectations:

- Windows hosting
- Domain registrar functionality (lookup, transfer, renew)
- Billing system
- Affiliate / referral tracking
- Live chat support module
- Phone-home telemetry
- LiteSpeed / OpenLiteSpeed (Nginx only)
- nginx → Apache fallback (some PHP apps need .htaccess; v1.1)
- IPv4 NAT for tenants (one shared public IP per node is fine)
- HTTP/3 / QUIC (Nginx 1.25+ has it; just enable in vhost template)

---

## 20. Database strategy (MySQL + PostgreSQL, isolation + optimization + hardening)

Hosting providers expect **both** MySQL/MariaDB and PostgreSQL as
first-class tenant-facing databases. Provider operator picks which
engines to install at v0.1 setup (one, the other, or both). Tenants
see "Databases" UI with engine type selector.

### 20.1 Per-tenant data model (recap)

```sql
CREATE TABLE databases (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('mysql', 'postgres')),
    db_name TEXT NOT NULL,           -- e.g. tenant_a3f9_wordpress
    db_user TEXT NOT NULL,           -- e.g. tenant_a3f9_wp
    remote_access BOOLEAN DEFAULT FALSE,  -- allow remote SQL clients?
    allowed_hosts TEXT[],            -- if remote_access, IPv4/v6 CIDRs
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(kind, db_name)
);
```

Naming convention: prefix all DB names and DB users with the tenant's
unix username (`tenant_a3f9_*`). Provides defense-in-depth visibility
in SHOW DATABASES / `\l`.

### 20.2 MySQL / MariaDB — per-tenant isolation

**User & GRANT layout**:

```sql
-- Create tenant-scoped user
CREATE USER 'tenant_a3f9_wp'@'%' IDENTIFIED WITH caching_sha2_password
    BY '<random-256bit-base64>'
    PASSWORD EXPIRE NEVER
    PASSWORD HISTORY 5
    PASSWORD REUSE INTERVAL 365 DAY
    FAILED_LOGIN_ATTEMPTS 5 PASSWORD_LOCK_TIME 1
    ATTRIBUTE '{"account_id": 12345}';

-- Grant scoped to this database ONLY
GRANT ALL PRIVILEGES ON `tenant_a3f9_wordpress`.* TO 'tenant_a3f9_wp'@'%';

-- Optional: resource group (CPU cap)
ALTER USER 'tenant_a3f9_wp'@'%'
    WITH MAX_USER_CONNECTIONS 25
         MAX_CONNECTIONS_PER_HOUR 5000
         MAX_QUERIES_PER_HOUR 0   -- 0 = unlimited; flip on for abusers
         MAX_UPDATES_PER_HOUR 0;

-- Bind user to a resource group (MySQL 8.0+ User-Defined Resource Groups)
CREATE RESOURCE GROUP IF NOT EXISTS tenant_default
    TYPE = USER
    VCPU = 0-1                       -- limit to first 2 cores per query
    THREAD_PRIORITY = 5;             -- below default (0)
ALTER USER 'tenant_a3f9_wp'@'%' DEFAULT RESOURCE GROUP tenant_default;
```

**Audit at user creation**: panel writes the resource_group binding,
not the operator. Tenants never get `CREATE USER`, `GRANT`, `RELOAD`,
`PROCESS`, `SHOW DATABASES`, `SUPER`, `FILE`, or anything outside
their schema.

**No remote_access by default**: `bind-address = 127.0.0.1, ::1` on
v0.1; remote access (e.g. for dev tools) requires tenant explicitly
opens it + supplies CIDR list, which the panel translates into
`'tenant_a3f9_wp'@'<cidr>'` GRANTs.

### 20.3 PostgreSQL — per-tenant isolation

PostgreSQL's role model is richer; we use it.

**Layout**:

```sql
-- Create tenant role (LOGIN role)
CREATE ROLE tenant_a3f9_wp WITH LOGIN
    PASSWORD '<random>'
    NOINHERIT
    CONNECTION LIMIT 25
    VALID UNTIL 'infinity';

-- Create the database, owned by this role
CREATE DATABASE tenant_a3f9_wordpress
    OWNER tenant_a3f9_wp
    ENCODING 'UTF8'
    LC_COLLATE 'C.UTF-8'
    LC_CTYPE 'C.UTF-8'
    TEMPLATE template0;

-- Revoke PUBLIC schema privileges (else any role can create tables)
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CREATE, USAGE ON SCHEMA public TO tenant_a3f9_wp;

-- Per-tenant statement timeouts to bound runaway queries
ALTER ROLE tenant_a3f9_wp SET statement_timeout = '60s';
ALTER ROLE tenant_a3f9_wp SET idle_in_transaction_session_timeout = '5min';
ALTER ROLE tenant_a3f9_wp SET lock_timeout = '30s';
ALTER ROLE tenant_a3f9_wp SET work_mem = '4MB';   -- per query op
ALTER ROLE tenant_a3f9_wp SET log_min_duration_statement = '1s';
```

**pg_hba.conf** entries written by the panel agent (one per tenant
role, ordered specific-first):

```
# TYPE  DATABASE                USER             ADDRESS         METHOD
hostssl tenant_a3f9_wordpress   tenant_a3f9_wp   127.0.0.1/32    scram-sha-256
hostssl tenant_a3f9_wordpress   tenant_a3f9_wp   ::1/128         scram-sha-256
# (remote entries injected when tenant enables remote access)
host    all                     all              all             reject
```

The agent edits pg_hba.conf via a managed-section pattern (begin/end
markers) and `pg_ctl reload` after change.

### 20.4 Optimization — MySQL/MariaDB

Per-node base `my.cnf` (auto-tuned by panel installer based on RAM):

```ini
[mysqld]
# InnoDB (the workhorse)
innodb_buffer_pool_size              = 50% of system RAM
innodb_buffer_pool_instances         = 8
innodb_log_file_size                 = 1G
innodb_log_buffer_size               = 64M
innodb_flush_log_at_trx_commit       = 1            # full ACID; 2 for non-critical
innodb_flush_method                  = O_DIRECT
innodb_file_per_table                = 1            # default in 8.0; explicit for clarity
innodb_io_capacity                   = 2000         # bump for SSD, ↑ for NVMe
innodb_io_capacity_max               = 4000
innodb_read_io_threads               = 8
innodb_write_io_threads              = 8
innodb_stats_on_metadata             = 0
innodb_adaptive_hash_index           = 1
innodb_doublewrite                   = 1            # keep on, NVMe handles it

# Threads & connections
thread_pool_size                     = (cores)      # MariaDB; MySQL needs thread_pool plugin
max_connections                      = 500          # per-tenant cap is 25 → 20 tenants concurrent
max_connect_errors                   = 100000
table_open_cache                     = 4000
open_files_limit                     = 65535

# Logging
slow_query_log                       = 1
slow_query_log_file                  = /var/log/mysql/slow.log
long_query_time                      = 1.0
log_queries_not_using_indexes        = 0
log_throttle_queries_not_using_indexes = 60

# Binary log (point-in-time recovery)
log_bin                              = /var/lib/mysql/mysql-bin
binlog_format                        = ROW
expire_logs_days                     = 7
sync_binlog                          = 1            # safety > perf
binlog_row_image                     = MINIMAL

# Disable legacy
query_cache_type                     = 0            # REMOVED in 8.0; explicit for MariaDB
performance_schema                   = ON
```

Per-tenant tuning hooks (panel exposes in UI):
- `MAX_USER_CONNECTIONS` (default 25)
- statement timeout via `MAX_EXECUTION_TIME` hint or proxy enforcement
- read-only flag (for staging)

Operator-visible monitoring: panel pulls `performance_schema.events_statements_summary_by_user_by_event_name`
to surface top slow queries per tenant.

### 20.5 Optimization — PostgreSQL

Per-node base `postgresql.conf` (auto-tuned by panel installer):

```ini
# Memory (tuned for 16GB host as example; scale linearly)
shared_buffers                       = 25% of RAM        # 4GB on 16GB
effective_cache_size                 = 75% of RAM        # 12GB on 16GB
work_mem                             = 4MB               # PER OPERATION; per-role override possible
maintenance_work_mem                 = 512MB
huge_pages                           = try

# WAL & checkpoints
wal_level                            = replica           # logical for v1.x HA
max_wal_size                         = 4GB
min_wal_size                         = 1GB
checkpoint_completion_target         = 0.9
checkpoint_timeout                   = 15min
wal_compression                      = on

# Planner
random_page_cost                     = 1.1              # SSD; 4 for HDD
effective_io_concurrency             = 200              # SSD; 2 for HDD
default_statistics_target            = 100

# Connections (note: prefer PgBouncer pooling, see below)
max_connections                      = 200              # raw; PgBouncer fans out

# Autovacuum (anti-bloat)
autovacuum                           = on
autovacuum_max_workers               = 4
autovacuum_naptime                   = 30s
autovacuum_vacuum_scale_factor       = 0.05             # more aggressive than 0.2 default
autovacuum_analyze_scale_factor      = 0.025

# Logging (security + slow query)
log_min_duration_statement           = 1000             # log queries >1s
log_connections                      = on
log_disconnections                   = on
log_lock_waits                       = on
log_temp_files                       = 0                # log all temp file creation
log_checkpoints                      = on
log_autovacuum_min_duration          = 0
log_line_prefix                      = '%t [%p]: [%l-1] user=%u,db=%d,client=%h '

# Extensions auto-installed
shared_preload_libraries             = 'pg_stat_statements'
```

**PgBouncer** (mandatory in front of Postgres for hosting workloads):

```ini
# /etc/pgbouncer/pgbouncer.ini
[pgbouncer]
listen_addr = 127.0.0.1, ::1
listen_port = 6432
auth_type = scram-sha-256
auth_file = /etc/pgbouncer/userlist.txt    # regenerated by panel from `databases` table
pool_mode = transaction                     # session for legacy apps, transaction for max efficiency
max_client_conn = 5000
default_pool_size = 25                      # matches per-tenant connection limit
reserve_pool_size = 5
server_tls_sslmode = require
ignore_startup_parameters = extra_float_digits,search_path
```

The panel agent regenerates `userlist.txt` (SCRAM hashes) and reloads
pgbouncer on `databases` table changes.

Per-role overrides via panel UI:
- `ALTER ROLE ... SET work_mem` (tenant can raise to plan max)
- `ALTER ROLE ... SET statement_timeout`
- `ALTER DATABASE ... SET random_page_cost` (advanced)

### 20.6 Hardening — common to both engines

| Concern | MySQL/MariaDB | PostgreSQL |
|---|---|---|
| Bind address | `bind-address = 127.0.0.1, ::1` default; per-tenant remote opt-in adds explicit GRANTs | `listen_addresses = 'localhost'` default; remote opt-in via pg_hba + listen_addresses widening |
| TLS in transit | `require_secure_transport = ON`; server cert auto-issued by libpanel/pki | `ssl = on`; `ssl_cert_file` / `ssl_key_file` from libpanel/pki |
| Auth | `default_authentication_plugin = caching_sha2_password`; MD5/native rejected | `password_encryption = scram-sha-256`; trust/md5 banned in pg_hba |
| Password policy | `validate_password.policy = STRONG`; length≥12, mixed case, special char | Application-side validation; SCRAM rotates iteration count |
| Failed login lockout | `failed_login_attempts = 5 password_lock_time = 1` per user | App-layer Fail2ban on `log_connections` output |
| File I/O | `secure_file_priv = ''`(blank disables LOAD DATA INFILE / SELECT INTO OUTFILE) | `pg_read_server_files` revoked from PUBLIC; large objects audited |
| Privilege escalation | No `WITH GRANT OPTION` ever granted to tenants; root@localhost socket-only | NOINHERIT on tenant roles; CREATEROLE forbidden; no SUPERUSER |
| Audit log | MariaDB Audit Plugin → JSON to rsyslog | `pgaudit` extension + `log_statement = ddl, mod` |
| `mysql_secure_installation` equivalent | Run on first start: drop anonymous, drop test DB, set root, no remote root | `initdb --auth-host=scram-sha-256`; `postgres` superuser local-socket-only |
| Defaults to disable | `LOAD DATA LOCAL INFILE = 0` | `lo_*` large object functions revoked |
| Backup encryption | `mysqldump` output piped through `age` or restic | `pg_dump` likewise |

**Common policy** (both engines): a dedicated **backup user** with
read-only privileges (`SHOW VIEW, RELOAD, LOCK TABLES, EVENT, TRIGGER`
for MySQL; `pg_read_all_data, pg_read_all_settings` for Postgres) used
by the backup cron. Never the root account.

### 20.7 Per-tenant DB quota enforcement

| Resource | MySQL | PostgreSQL |
|---|---|---|
| Disk used by DB | sum(`information_schema.tables.data_length + index_length` WHERE schema = tenant's DB) | `pg_database_size(dbname)` |
| Soft limit alert | When 80% of plan's DB-quota, email tenant + operator | Same |
| Hard limit | At 100%: REVOKE `INSERT, UPDATE, CREATE` for that user; allow SELECT/DELETE so tenant can clean up | Same; revoke via `ALTER DEFAULT PRIVILEGES` rollback |
| Re-enable | Operator UI or automatic when usage drops below 90% | Same |

Quota check runs every 5 minutes per tenant by the agent.

### 20.8 Distro-specific notes for DB tier

| Concern | Ubuntu 22.04/24.04 | Rocky/Alma 9 |
|---|---|---|
| MySQL package | `mysql-server-8.0` (apt) | `mysql-community-server` from MySQL upstream repo |
| MariaDB package | `mariadb-server` (apt) | `mariadb-server` (dnf) |
| Postgres package | `postgresql-16` from PGDG | `postgresql16-server` from PGDG |
| PgBouncer | `pgbouncer` (apt) | `pgbouncer` (dnf) |
| Default cluster | `/var/lib/mysql`, `/var/lib/postgresql/16/main` | `/var/lib/mysql`, `/var/lib/pgsql/16/data` |
| SELinux | n/a (AppArmor) | `setsebool -P httpd_can_network_connect_db on`; ship `1panel-host.pp` |
| systemd service | `mysql.service`, `postgresql@16-main.service` | `mysqld.service`, `postgresql-16.service` |

Panel agent normalizes these via a thin abstraction (`dbmgr` package
in libpanel).

### 20.9 Backup integration

Tenant backup = per-DB pg_dump / mysqldump → restic snapshot scoped to
that account. Per-DB granularity matters because operators want to
restore one site's DB without touching the rest of the account.

```bash
# MySQL per-tenant DB
mysqldump --single-transaction --routines --triggers --events \
    --hex-blob --set-gtid-purged=OFF \
    -u backup_user -p<read-pw> \
    tenant_a3f9_wordpress | zstd | restic backup --stdin --stdin-filename db.sql.zst ...

# Postgres per-tenant DB
PGPASSWORD=<read-pw> pg_dump -h 127.0.0.1 -p 6432 -U backup_user \
    --format=custom --compress=9 --no-owner --no-acl \
    tenant_a3f9_wordpress | restic backup --stdin --stdin-filename db.dump ...
```

Restore is a separate UI flow that:
1. Suspend tenant DB (revoke connections).
2. Restic pull dump file to staging path.
3. Drop & recreate empty DB.
4. Import.
5. Re-enable connections.

Atomic via DB transaction where possible; for MySQL falls back to
"create alongside, rename, drop old".

---

## 21. Web Application Firewall (Coraza + OWASP CRS)

### 21.1 Why Coraza, not ModSecurity v3 directly

| | ModSecurity 3 (libmodsecurity) | Coraza |
|---|---|---|
| Language | C++ | Go |
| Integration with our stack | external binding via nginx-modsecurity-module (C) | direct Go library; matches our codebase |
| Performance | mature, well-optimized | comparable; benchmarked 1.4-2x faster on some rule sets |
| Rule format | SecRules (the de-facto standard) | SecRules (compatible) |
| OWASP CRS support | yes (first-class) | yes (first-class, official CRS test suite passes) |
| Maintenance | CRS & ModSec maintained by separate teams | Both under SpiderLabs/CRS umbrella; Coraza now CRS's recommended engine |
| Embedding in Nginx | as native module (recompile) | via SPOA (haproxy SPOE protocol) or as Caddy module |

Decision: **Coraza for v0.4**. Specifically use **coraza-spoa** to
let unmodified Nginx (with `ngx_http_modsecurity_module` OR pure SPOE
client patch) speak to a Coraza daemon over Unix socket. Trade-off:
extra hop per request (~50µs locally), avoid recompiling Nginx.

Alternative: ship our own Nginx build with `coraza-nginx` filter
module. Provider-friendlier for new installs; harder for operators
already running custom Nginx. v0.4 ships SPOA, v1.1 may add native
build option.

### 21.2 Architecture

```
                   tenant request
                         |
                         v
                +-----------------+
                |     Nginx       |
                | (vhost per      |
                |  tenant site)   |
                +-------+---------+
                        | SPOE protocol (Unix socket)
                        v
                +-----------------+         +-----------------+
                |  coraza-spoa    |←-loads--|  OWASP CRS 4.x  |
                | (Go daemon,     |         |  /etc/coraza/    |
                |  systemd unit)  |         |    crs/         |
                +--------+--------+         +-----------------+
                         |
                         | decision: allow / deny / log
                         |
                Nginx serves OR returns 403
                         |
                Log decision → /var/log/coraza/audit.json
                         |
                Panel pulls latest entries for UI
```

### 21.3 Per-site config

`sites` table gets:

```sql
ALTER TABLE sites ADD COLUMN waf_mode TEXT DEFAULT 'detect';
   -- 'off' | 'detect' (log only) | 'prevent' (block + log)
ALTER TABLE sites ADD COLUMN waf_paranoia_level INT DEFAULT 1;
   -- 1 (least false positive) ... 4 (paranoid)
ALTER TABLE sites ADD COLUMN waf_anomaly_threshold_inbound INT DEFAULT 5;
ALTER TABLE sites ADD COLUMN waf_anomaly_threshold_outbound INT DEFAULT 4;
```

Tenant UI:
- Toggle: Off / Detect / Prevent.
- Slider: Paranoia level 1-4 (with explanations + warning that 3-4
  may break legitimate traffic on dynamic sites).
- Threshold tuning (advanced).
- Custom rule editor (textarea with syntax highlight; saved per site
  as `/etc/coraza/custom/<site>.conf` and reloaded).
- "Allow this request" button on log entry → adds a rule exception.

### 21.4 Rule sets shipped

- **OWASP CRS 4.x** (pinned version, vendored in repo or pulled at
  install).
- Categories: SQL injection, XSS, LFI, RFI, command injection, PHP
  injection, generic attacks, Trojan signatures, generic detection.
- Default paranoia level **1** for v0.4 (low false positive); operators
  can globally raise.

### 21.5 Logging & analytics

Coraza writes JSON audit logs:

```json
{
  "timestamp": "2026-05-12T10:23:45Z",
  "site": "blog.example.com",
  "tenant_id": 1234,
  "client_ip": "1.2.3.4",
  "request_uri": "/wp-login.php?union+select",
  "matched_rules": [{"id": 942100, "msg": "SQL Injection Attack", "severity": 4}],
  "anomaly_score": 7,
  "decision": "deny",
  "user_agent": "Mozilla/5.0 ..."
}
```

Panel ingests via fsnotify on the audit file → ships entries to
operator-side TimescaleDB (or just per-day rotated JSON for v0.4).

### 21.6 Rate limit + IP block escalation

A second-tier policy (not in CRS) ships out of the box:
- After **5 denied requests in 1 minute** from one IP: block IP for
  10 min at nftables level (faster than CRS to short-circuit further
  traffic).
- Repeat offender (3 blocks in 24h): block for 24h.
- Permanent ban: operator-only action.

### 21.7 Tenant escape mitigations

WAF only sees HTTP. Tenant uploading malicious PHP doesn't trigger
WAF; that's the OS isolation layer's job. WAF + OS isolation are
defense-in-depth.

---

## 22. Curated app catalog

### 22.1 Purpose

Tenants get cPanel/Softaculous-style one-click app installs WITHOUT
giving them Docker, root, or system service access. Apps run as the
tenant's UNIX UID inside the same chroot as their files.

### 22.2 Why not Docker for tenant apps

- Tenants don't have Docker access (would break OS isolation model).
- Container apps consume RAM + disk per instance (untuned WP container
  uses 200-500MB). Hosting providers oversubscribe; bare PHP-FPM with
  shared opcache is 10-20x more efficient.
- Operational complexity (image registry, vulnerability scanning).

### 22.3 Manifest format

Each app = directory under `Domus/apps/<app-id>/`:

```
apps/
└── wordpress/
    ├── manifest.yaml
    ├── icon.svg
    ├── README.md
    ├── install.sh             # runs as tenant UID after extract
    ├── upgrade.sh
    ├── uninstall.sh
    └── source/
        └── ...                # OR: download URL pinned in manifest
```

`manifest.yaml`:

```yaml
id: wordpress
name: WordPress
version: '6.5.3'
category: blog
description: The world's most popular CMS.
homepage: https://wordpress.org
license: GPLv2-or-later
icon: icon.svg

requirements:
  php:
    min: '7.4'
    recommended: '8.2'
    extensions: [mysqli, gd, curl, zip, xml, mbstring, intl]
  database:
    kind: mysql       # or postgres (some apps support both)
    min_version: '5.7'
    size_initial_mb: 10
  disk_mb: 50          # initial disk usage estimate
  memory_mb: 128       # PHP-FPM memory the app will sit in

source:
  type: tarball
  url: https://wordpress.org/wordpress-6.5.3.tar.gz
  sha256: <checksum>

config:
  - key: site_title
    label: Site Title
    type: string
    required: true
  - key: admin_username
    label: Admin Username
    type: string
    required: true
  - key: admin_email
    label: Admin Email
    type: email
    required: true
  - key: admin_password
    label: Admin Password
    type: password
    auto_generate: true
    min_length: 16

install:
  steps:
    - extract_source_to: '${DOC_ROOT}'
    - run: install.sh           # tenant-uid; receives env: DOC_ROOT, DB_*, SITE_*
    - chmod_recursive:
        path: '${DOC_ROOT}'
        mode: '0644'
        dirs: '0755'
    - chown_recursive:
        path: '${DOC_ROOT}'
        owner: tenant
        group: tenant

post_install:
  open_url: '/wp-admin/'
  show_credentials: true
```

### 22.4 Install flow

When tenant clicks "Install WordPress":

1. Panel validates account quota (disk, DB count).
2. Panel allocates a new MySQL DB + user (Section 20.2).
3. Panel creates a temp dir under tenant's home.
4. Panel downloads tarball, verifies SHA-256, extracts.
5. Panel runs `install.sh` as tenant UID via
   `systemd-run --uid=<tenant-uid> --slice=tenant-<id>.slice
   --working-directory=/home/<tenant>/...`
   - install.sh receives env vars: DOC_ROOT, DB_HOST, DB_NAME, DB_USER,
     DB_PASS, SITE_TITLE, ADMIN_USER, ADMIN_EMAIL, ADMIN_PASS.
6. install.sh exits 0 → registered in `installed_apps` table.
7. Panel surfaces credentials in UI once (with copy button); never
   stored plaintext.

### 22.5 Initial catalog (v0.4)

Targeting 8-10 apps for v0.4 ship:

| App | Engine | DB | Why |
|---|---|---|---|
| WordPress | PHP | MySQL | The #1 install of any hosting product |
| Joomla | PHP | MySQL | Strong demand in EU markets |
| Drupal | PHP | MySQL or PostgreSQL | Government/enterprise sites |
| Ghost | Node.js | MySQL | Modern blogging; PHP-only hosts can skip |
| phpBB | PHP | MySQL | Forum staple |
| MediaWiki | PHP | MySQL | Wiki use case |
| NextCloud (lite) | PHP | MySQL or PostgreSQL | Self-host file sharing; "lite" = no preview generators that need root |
| Matomo | PHP | MySQL | Privacy-respecting analytics |
| phpMyAdmin | PHP | MySQL | DB admin (operator-installed once, tenant-shared) |
| phpPgAdmin | PHP | PostgreSQL | DB admin |

v1.0 expansion target: 25+ apps including Magento, PrestaShop,
OpenCart, Roundcube (alternative to SnappyMail), OctoberCMS, etc.

### 22.6 Update lifecycle

Apps register their version in `installed_apps`. Panel cron checks
catalog for newer versions monthly. On update available:
- Tenant sees "Update available" badge.
- "Update" button runs `upgrade.sh` (manifest-supplied) in a backup-first
  flow:
  1. restic snapshot of the app's doc root.
  2. SQL dump of the app's DB.
  3. Run `upgrade.sh`.
  4. On non-zero exit, restic restore + DB restore.

Tenant can pin to a major version to avoid breaking changes.

### 22.7 Security boundary

Apps run as tenant UID. They have NO MORE PRIVILEGES than the tenant
itself. If WordPress core has an RCE, attacker gets tenant-uid access
which is already chrooted, quota'd, and slice-capped. The catalog is
NOT a trust extension; it's a convenience layer.

### 22.8 Out-of-catalog installs

Tenant can always upload arbitrary PHP code. The catalog is the
"easy path", not a gate. Custom uploads run under exact same
isolation.

