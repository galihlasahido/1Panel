# PLAN.md — Multi-Node (Multi-Server) Control for 1Panel

Tujuan: Satu instance 1Panel (master) bisa mengelola banyak server (nodes/slaves)
melalui agent yang terpasang di masing-masing server, mirip WHM → cPanel servers
atau Kubernetes control plane → kubelets.

## Status

Branch kerja: `release-2.1.12`
Mulai: 2026-05-11
Target v1 solid: ~4–5 minggu

## Keputusan desain (terkunci)

| # | Topik | Keputusan |
|---|---|---|
| 1 | Enrollment method | **SSH-push** sebagai default. User input IP + SSH credential di UI master, master install agent + inject cert. Token-pairing & manual cert-exchange ditunda. |
| 2 | Network topology | **Master initiates → Slave** via mTLS HTTPS. Pola standar (Kubernetes API server → kubelet, Ansible). Reverse-tunnel untuk slave di belakang NAT = future work, di luar scope v1. |
| 3 | Identitas node "local" | **Tetap pakai Unix socket** (`/etc/1panel/agent.sock`). Master bukan entry di tabel `nodes`; slot `currentNode=="local"` di-bypass langsung ke `proxy.LocalAgentProxy`. |
| 4 | Multi-master / HA | **Tunda.** v1 single-master. |
| 5 | Version compatibility | **Wajib version check di health endpoint.** Master refuse proxy kalau slave version major-mismatch. Major-mismatch = error keras, minor-mismatch = warning kuning di UI. |

## Arsitektur yang dituju

```
                    Browser (frontend SPA)
                            |
                            | header: CurrentNode: <name>|local
                            v
                +----------------------+
                |     1panel-core      |  port 9999 (TLS opsional)
                |  - auth, session     |
                |  - CSRF, entrance    |
                |  - proxy router      |
                +----------+-----------+
                           |
        +------------------+-------------------+
        |                                      |
   CurrentNode==local                CurrentNode==<name>
        |                                      |
        v                                      v
  unix socket                          HTTPS + mTLS
  /etc/1panel/agent.sock               (cert dari master CA)
        |                                      |
        v                                      v
  1panel-agent (master)               1panel-agent (slave)
  IsMaster = true                     IsMaster = false
                                      validates Proxy-Id header
```

## Plumbing yang sudah ada di OSS (jangan dibikin ulang)

Saat menulis kode, manfaatkan ini dulu:

- `agent/server/server.go:35-109` — agent sudah punya dua mode listen (Unix socket vs TLS+mTLS) berdasarkan `global.IsMaster`. Tinggal pastikan `IsMaster` di-set false di slave.
- `agent/middleware/certificate.go:16-39` — mTLS validation + `Proxy-Id` header check terhadap `/etc/1panel/.nodeProxyID`. Tinggal pastikan file ini ditulis saat enrollment.
- `agent/init/migration/migrations/init.go:96-115` — tabel `setting` agent sudah punya slot `ServerCrt`, `ServerKey`, `RootCrt`, `NodeScope`, `NodePort` (encrypted via `utils/encrypt`).
- `core/init/router/proxy.go:20-65` — `Proxy()` middleware sudah baca header `CurrentNode`, sudah panggil `xpack.Proxy(c, currentNode)` untuk non-local. Tinggal isi stub-nya.
- `core/utils/xpack/community.go:15` — stub `func Proxy(c *gin.Context, currentNode string) {}` yang harus diisi.
- `agent/utils/xpack/community.go:25-32` — `LoadNodeInfo()` yang di OSS hardcoded `Scope: "master"`, `IsMaster = true`. Harus dibikin baca dari env/file installer di mode slave.
- `frontend/src/api/index.ts:36-40` — axios sudah otomatis kirim header `CurrentNode` dari `globalStore.currentNode`. UI tinggal set store ini saat user pilih node.

---

## Fase 1 — PKI bootstrap (CA di master) ✅ DONE

**Goal:** Master bisa generate & sign cert untuk tiap slave.

- [x] PKI helpers di `core/utils/pki/pki.go`: `GenerateCA`, `IssueServerCert`, `IssueClientCert` (ECDSA P-384 CA, P-256 leaf, 5y validity, RFC 1035 commonName validation). Tested.
- [x] Migration `InitMasterCA` di `core/init/migration/migrations/node_ca.go` — generate root CA on first boot, simpan encrypted ke setting `MasterCACrt` / `MasterCAKey`, validitas 10 tahun.
- [x] Service `core/app/service/node_pki.go`:
  - `IssueAgentCert(commonName, addr)` — bikin slave server cert.
  - `IssueMasterClientCert(commonName)` — bikin master client cert (untuk dial slave).
  - `GetCABundle()` — return root CA PEM.
  - `RotateCA()` — out of scope v1, dokumentasikan di service comments.
- [x] Master local tetap pakai Unix socket — PKI hanya untuk slave (no-op untuk master agent).
- [x] Unit test untuk issue + verify + validation flow (7 test cases pass).

---

## Fase 2 — Tabel & model Node di core ✅ DONE

**Goal:** Persist daftar node, kreden mTLS, status.

- [x] Model `core/app/model/node.go`:
  ```go
  type Node struct {
      BaseModel
      Name        string  `gorm:"uniqueIndex;not null"`  // "prod-web-1"
      Addr        string  `gorm:"not null"`              // IP atau hostname
      Port        uint    `gorm:"default:9999"`
      Status      string  `gorm:"default:'Pending'"`     // Pending|Healthy|Unhealthy|VersionMismatch
      Version     string                                  // versi agent terdeteksi
      GroupID     uint
      AgentCrt    string  // encrypted, dipakai core sebagai client cert ke node ini
      AgentKey    string  // encrypted
      ProxyID     string  // random string, ditulis ke /etc/1panel/.nodeProxyID di slave
      LastCheck   *time.Time
      Description string
  }
  ```
- [x] Migration `AddNodeTable` di `core/init/migration/migrations/node.go` — `AutoMigrate(&model.Node{})`. Registered di `core/init/migration/migrate.go`.
- [x] Repo `core/app/repo/node.go` dengan `INodeRepo`: List, Page, Get, Create, Update, Delete, MarkChecked. Pakai existing `WithByName`, `WithByStatus`, `WithByID`, `WithByGroupID` di `core/app/repo/common.go`.
- [x] DTO `core/app/dto/node.go`: `NodeCreate`, `NodeUpdate`, `NodeSearch`, `NodeInfo`. AgentCrt/AgentKey/ProxyID di-mark `json:"-"` supaya tidak leak ke API response.

**Done when:** Bisa CRUD node via repo test, AgentCrt/AgentKey ter-encrypt at-rest. ✓ (encryption via existing `utils/encrypt` AES-GCM)

---

## Fase 3 — Isi stub `xpack.Proxy()` di core ✅ DONE

**Goal:** Request dengan `CurrentNode: <name>` benar-benar diteruskan ke agent slave via mTLS.

- [x] Implementasi `core/utils/xpack/node_proxy.go` `Proxy()` (build tag `!xpack && !xpackee`):
  - Lookup node by name.
  - Cache `*httputil.ReverseProxy` per node ID (rebuild kalau cert berubah). Pakai `sync.Map` atau LRU.
  - Build `*http.Transport` dengan `tls.Config`:
    - `Certificates`: client cert dari `node.AgentCrt`/`AgentKey` (decrypt).
    - `RootCAs`: pool dari `MasterCACrt` (slave akan present cert yang ditandatangani CA yang sama, bidirectional trust).
    - `ServerName`: `node.Name` (harus match SAN cert slave).
  - Director: set `req.URL.Scheme = "https"`, `req.URL.Host = "<addr>:<port>"`, set header `Proxy-Id: <node.ProxyID>`.
  - ErrorHandler: log Error level + return 502 dengan body `"node unreachable: <err>"`. `ErrNodeUnBind` / `ErrNodeVersionMismatch` i18n keys diisi di Fase 6.
- [x] Invalidate cache via `xpack.InvalidateNodeProxy(nodeID)`. Dipanggil otomatis dari `NodeService.Update` dan `Delete`.
- [x] WebSocket upgrade jalan default — pakai `httputil.NewSingleHostReverseProxy` Go 1.25 native, Director cuma append X-Forwarded-Proto + Proxy-Id (Proxy-Id selalu di-overwrite dari per-node secret, bukan dari user header — anti-spoofing).
- [x] Bonus: `xpack.NodeHTTPClient(node, timeout)` + `xpack.NodeBaseURL(node)` helpers untuk reusable mTLS client (dipakai cron + service).

**Done when:** Setting cert manual di 2 VM (sebelum enrollment otomatis ada), bisa pilih node di UI → file manager / terminal di slave berfungsi dari browser via master. (Test end-to-end perlu Fase 5+6 jalan dulu.)

---

## Fase 4 — Health check + version compatibility ✅ DONE

**Goal:** Tahu node hidup atau mati, dan version-nya kompatibel.

- [x] Endpoint di agent: `GET /api/v2/health/check` — `agent/app/api/v2/health.go` upgraded dari `helper.Success(c)` ke return `{"version", "scope", "uptimeSecs"}` dari setting + process start time.
- [x] Cron job `core/init/cron/job/node_health.go`:
  - Schedule `@every 30s` wired di `core/init/cron/cron.go`.
  - Loop semua node, GET `/api/v2/health/check` via mTLS (`xpack.NodeHTTPClient`).
  - Update Status+Version+LastMessage+LastCheck via `NodeRepo.MarkChecked()`.
  - Version compare: split `vX.Y.Z` → major/minor. Major mismatch → `VersionMismatch`. Minor mismatch → `Healthy` dengan LastMessage drift warning.
- [x] Block proxy di `Proxy()` kalau `Status == "VersionMismatch"` — return 502 dengan i18n key `ErrNodeVersionMismatch` + detail.
- [x] Manual recheck endpoint: `POST /api/v2/core/nodes/healthcheck/:id` (`NodeService.Recheck` melakukan probe sync + persist).
- [x] Route CRUD lain: `/search`, `/:id` (get), `/update`, `/del/:id`. `/add` masih stub 500 sampai Fase 5.

**Done when:** Stop agent di slave → UI master tunjuk node merah dalam <60 detik tanpa user refresh. (Tergantung Fase 6 UI untuk visual.)

---

## Fase 5 — SSH-push enrollment ✅ DONE (backend + agent bootstrap; installer-repo work pending)

**Goal:** User klik "Add Node" → input IP/SSH cred → master otomatis install agent + setup mTLS.

- [x] Reuse `core/utils/ssh` (sudah ada di codebase).
- [x] Service `core/app/service/node.go` `EnrollViaSSH(req dto.NodeCreate)`:
  1. Validate SSH connectivity via `ssh.NewClient` (password atau key+passphrase).
  2. Reject duplicate name (uniqueIndex pre-flight).
  3. Generate `ProxyID` (32 char alphanumerik via RandStr).
  4. Generate slave server cert via `IssueAgentCert(name, addr)`.
  5. Generate master client cert via `IssueMasterClientCert(name)`.
  6. SCP cert bundle (scope/port/proxy_id/server.crt/server.key/root.crt) ke `/etc/1panel/bootstrap/` di slave (mode 0700 dir, 0600 files via umask in remote shell).
  7. Restart slave agent (`systemctl restart 1panel-agent || service 1panel-agent restart`).
  8. Persist node row dengan master's client cert encrypted via `encrypt.StringEncrypt`.
  9. Inline `Recheck()` untuk validate end-to-end + update Status.
- [BLOCKED — installer repo] Installer changes documented di `INSTALLER_TODO.md`:
  - Mode `INSTALL_MODE=agent` skip install core binary, install only `1panel-agent` + service.
  - Respect existing `/etc/1panel/bootstrap/` directory yang sudah ditulis master via SSH.
  - Buka port 9999 di firewall slave (firewalld/ufw/nftables).
- [x] `agent/utils/xpack/community.go` `LoadNodeInfo()`:
  - Read `/etc/1panel/bootstrap/scope` → bila `slave`, baca cert+key+CA+port+proxy_id dari file lain di dir.
  - Set `global.IsMaster = false`, populate `NodeInfo` (termasuk `RootCrt` baru ditambah ke struct).
  - Tulis `/etc/1panel/.nodeProxyID` (mode 0600) untuk middleware `certificate.go`.
- [x] `agent/init/migration/migrations/init.go`: persist `RootCrt` ke setting (encrypted) saat slave first boot — gap yang sebelumnya bikin agent server.go gak punya `ClientCAs` untuk mTLS validate.
- [x] Rollback partial: master DB row gak di-insert sampai SSH push sukses; bootstrap files di slave dibiarkan untuk retry.

**Done when:** Klik "Add Node" di UI → 2 menit kemudian node muncul Healthy, bisa langsung di-switch & jalan. **Caveat:** butuh installer-repo work (lihat `INSTALLER_TODO.md`) supaya `1panel-agent` binary udah ada di slave host sebelum enrollment.

---

## Fase 6 — Frontend: UI Nodes + selector ✅ DONE (list + add) — [REQUIRES MANUAL BROWSER VERIFICATION]

**Goal:** User experience seamless multi-node.

- [x] API client: `frontend/src/api/interface/node.ts` (types `NodeInfo`, `NodeCreate`, `NodeUpdate`, `NodeSearch`) + `frontend/src/api/modules/node.ts` (searchNodes, getNode, addNode, updateNode, deleteNode, recheckNode).
- [x] View baru `frontend/src/views/setting/node/`:
  - `index.vue` — list nodes (table: name, addr, status badge dengan warna by enum, version, last check, last message, refresh + delete buttons).
  - `operate.vue` — drawer enrollment SSH (password atau key+passphrase, name validated via RFC 1035 regex).
- [x] Tambah ke router `frontend/src/routers/modules/setting.ts` — path `/settings/nodes`, name `Nodes`.
- [x] i18n: `setting.nodes` di `en.ts`. **[I18N TODO]** locales lain (zh, id, dst.) — terjemahan belum.
- [BLOCKED — REQUIRES MANUAL BROWSER VERIFICATION] Node selector di header layout (`frontend/src/layout/`). Tidak ditulis di batch ini karena modifikasi layout butuh visual review. Workaround sementara: user buka DevTools, set `globalStore.currentNode = '<nodeName>'`. **TODO** sebelum ship.
- [x] Handling `ResultEnum.NodeUnBind` di axios sudah ada (`api/index.ts:88-91`).

**Done when:** Tester awam bisa add node via UI tanpa baca dokumentasi, switch node terasa seperti switch tab. **Status: butuh manual browser verify + selector di header.**

---

## Fase 7 — Cross-node sync ✅ DONE (infrastructure) — per-type endpoints geser ke v1.1

- [x] **Broadcast infrastructure**: `xpack.BroadcastToHealthyNodes(method, path, body, timeout) []BroadcastResult` di `core/utils/xpack/node_proxy.go`. Reuses `NodeHTTPClient` (per-call transport, tidak nyentuh proxy cache). Returns per-node Status + Err sehingga caller bisa decide partial-failure.
- [x] Wire `xpack.Sync(dataType)` ke broadcast: POST `/api/v2/sync/<dataType>` ke semua healthy nodes. 404 di-downgrade (slave receiver belum shipped).
- [ ] [REQUIRES v1.1] Implementasi per-type receiver di agent (POST `/api/v2/sync/<type>`) untuk:
  - `backup` — slave re-pull backup storage credentials from master.
  - `app_cache` — slave refresh app marketplace cache.
  - `language`, `proxy`, `script`, `alert`, `custom_app`, `edition` — semua dataType yang sudah constants di `core/constant/common.go`.
- [ ] [REQUIRES v1.1] `xpack.PushSSLToNode` di `agent/utils/xpack/community.go:87`. Saat ini stub no-op. Implementasinya akan: master agent → master core hop → broadcast SSL cert ke slaves yang di-pilih.
- [ ] Operation log fan-out: skip (filter "by node" sudah memadai untuk v1).
- [ ] Dashboard agregat: skip.

---

## Risiko & open questions yang belum diputuskan

- [ ] **Slave di belakang NAT** — out of scope v1, tapi dokumentasikan di README. Workaround user: pakai SSH reverse tunnel manual atau VPN (Tailscale/WireGuard).
- [ ] **Cert rotation** — v1 skip. Dokumentasikan: re-enroll node kalau cert expire. Validitas cert: kasih panjang (5 tahun) untuk hindari issue ini di v1.
- [ ] **Port lain selain 9999** di slave — sudah supported (`Node.Port`), tapi installer harus bisa override. Pastikan flag installer terima.
- [ ] **Firewall di slave** — installer harus buka port 9999 di firewalld/ufw, atau enrollment harus instruksi user buka manual. Decision needed.
- [ ] **OSS license & feature gating** — fitur ini akan masuk OSS atau tetap di-gate `xpack`? Default plan-nya: **masuk OSS** (mengisi stub, bukan menambah Pro feature). Konfirmasi sebelum merge.
- [ ] **Naming collision dengan Pro** — Pro 1Panel sudah punya multi-node. Kalau user upgrade dari OSS-multinode ke Pro, perlu jalur migrasi data `nodes` table → format Pro. Anggap forward-compatible: gunakan skema yang reasonable mirip Pro kalau bisa di-reverse-engineer dari API.

## Audit findings & status (snapshot 2026-05-11)

Audit dijalankan pada code paths security-critical (PKI baru, mTLS,
proxy, encrypt, session, CSRF, API auth). Ringkasan status setelah
patch:

| # | Finding | Status | Catatan |
|---|---|---|---|
| C1 | AES-CBC tanpa MAC | **Fixed** | Migrasi ke AES-256-GCM; legacy CBC tetap di-decrypt untuk back-compat. Tests roundtrip + tamper-rejection di `core/utils/encrypt/encrypt_test.go` & agent equivalent. |
| C2 | EncryptKey plaintext di DB | **Fixed (defense-in-depth)** | Key sekarang di-persist ke `/etc/1panel/.encrypt_key` mode 0600 (file preferred → CONF cache → DB). Backup yang capture DB tanpa `/etc/1panel/` jadi tidak cukup untuk decrypt. Atomic write via tmp + rename. Tests di `keystore_test.go` di kedua module. Argon2id-from-password tetap future work. |
| H1 | Non-CT ProxyID compare | **Fixed** | `subtle.ConstantTimeCompare` di `agent/middleware/certificate.go`. |
| H2 | Non-CT CSRF token compare | **Fixed** | `subtle.ConstantTimeCompare` di `psession.go`. |
| H3 | Unix socket world-readable | **Fixed** | `/etc/1panel` mode 0700, `agent.sock` mode 0600 di `agent/server/server.go`. |
| H4 | MD5 API token | **Fixed (back-compat)** | Tambah jalur HMAC-SHA256 (token length 64), retain MD5 (length 32) untuk client lama. Keduanya constant-time. New clients pakai SHA-256. |
| H5 | `cat` via shell untuk baca file | **Fixed** | Ganti `os.ReadFile` di `agent/middleware/certificate.go`. |
| M1 | TLS MinVersion implicit | **Fixed** | Explicit `MinVersion: tls.VersionTLS12` di agent + core (both SSL and mux branches). |
| M2 | Session cap | **Fixed** | Global `maxSessionEntries=64` + tambah per-user `maxSessionsPerUser=8` via `evictPerUserOverflowLocked()`. Akun yang spam login tidak lagi bisa displace user lain. |
| M3 | File share endpoints tanpa rate limit | **Fixed (tightened)** | Audit menemukan `FileSharePublicAccess` middleware sudah ada (20 req/s per IP). Tambah: ubah per-code limiter dari per-IP+code ke per-code GLOBAL (3 attempts per 10 sec), supaya distributed brute-force juga capped. Password handling sudah aman (SHA-256+salt, constant-time, panjang 4-256). |
| M4 | Panic log level Debug | **Fixed** | Naikkan ke `Errorf` + stack trace via `runtime/debug.Stack()`. |
| M5 | HTTP-only mode silent | **Fixed** | Startup `Warnf` jelas-jelas. |
| M6 | PKI commonName tidak divalidasi | **Fixed** | Regex RFC 1035 di `core/utils/pki/pki.go`, test untuk reject empty/leading-hyphen/space/newline/oversize. |
| L1 | RandStr entropy suboptimal | **Mitigated** | Tambah `common.RandBytes(n)` di kedua module untuk callers yang butuh entropi penuh. Existing RandStr tetap untuk back-compat. |
| L2 | CSRF skip kalau no session | **Fixed** | Defense-in-depth: selalu enforce CSRF pada unsafe method `/api/v2/` (kecuali whitelisted login + API_AUTH). |
| L3 | Shell injection via SudoHandleCmd | **No-op** | `SudoHandleCmd()` return literal `"sudo "` atau `""`, bukan user-controlled. Pola tetap risky kalau implementasi berubah — catat saja. |

**Future work** (tidak blok v1):
- C2 (lanjutan): Argon2id-from-admin-password derivation untuk full key isolation (butuh UX flow: prompt password di startup atau pakai kernel keyring). Saat ini file 0600 sudah significant DiD.
- H4: deprecate jalur MD5 token setelah migrasi client (target ~6 bulan, monitor pemakaian via log).
- TLS 1.3-only di mTLS master↔slave (sekarang TLS12+); aman karena traffic full di kontrol kita.
- Audit otomatis di CI: `gosec`, `semgrep`, `govulncheck`.

## Definition of Done (v1)

- [ ] User bisa add 2+ slave via SSH dari UI master tanpa baca dokumentasi.
- [ ] Switch node di header → semua page (file manager, terminal, app store, website, container, database) jalan tanpa edge case.
- [ ] Slave mati → UI master tampilkan merah <60 detik.
- [ ] Version mismatch → block proxy + tampilkan banner di UI.
- [ ] Remove node → cert revoke (best-effort: hapus dari master DB, kasih instruksi manual untuk uninstall di slave).
- [ ] Backup/restore master tetap berfungsi termasuk tabel `nodes`.
- [ ] Tidak ada regression di mode single-node (CurrentNode=local tetap pakai Unix socket).
