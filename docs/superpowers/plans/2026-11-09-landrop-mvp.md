# LanDrop v0.1 MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary that serves a Preact web app so LAN peers can exchange text and files with the host through a browser. It covers every P0 requirement and the P1 requirements from the spec.

**Architecture:** One Go process. In-memory stores (items, sessions, share links, SSE hub) live behind a stdlib `net/http` router. Files are streamed to a temp dir that is keyed by item ID. The Preact + Vite + TS frontend builds into `internal/webui/dist` and is embedded with `//go:embed`. Real-time updates go over SSE with a replay ring buffer. Writes go over plain POST/DELETE.

**Tech Stack:** Go 1.26 stdlib, `github.com/grandcat/zeroconf`, `github.com/skip2/go-qrcode`, `golang.org/x/sys`. Preact 10, `@preact/signals`, Vite, TypeScript, highlight.js (core), Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-09-landrop-mvp-design.md` (built on `LandropPRD.md`)

## Global Constraints

- Go module `github.com/lupguo/landrop`, `go 1.26`. Build everything with `CGO_ENABLED=0`.
- Go third-party deps are limited to: `github.com/grandcat/zeroconf`, `github.com/skip2/go-qrcode`, `golang.org/x/sys`. Nothing else.
- Binary size ≤ 15MB with frontend included. Idle RSS ≤ 30MB. Memory must not grow with file size, so stream everything.
- Cross-compile targets: darwin/arm64, darwin/amd64, windows/amd64, linux/amd64, linux/arm64.
- Default listen `0.0.0.0:9958`. If the port is busy, try +1 up to 10 attempts. If all fail, exit with an error that mentions `--port`.
- Flags: `--port` `--name`(default `drop`) `--dir` `--no-auth` `--ttl`(default `24h`) `--keep` `--max-size`(default `4GB`) `--no-save` `--no-open` `--lang`(en|zh|ja|ko, default `en`, env `LANDROP_LANG`) `--version`.
- Access code: 4 random digits (crypto/rand, leading zeros allowed); no accounts — entering the code is the only gate. Link token: 128-bit random hex, passed as `?t=`. Session cookie `landrop_sid` with `HttpOnly; SameSite=Strict; Path=/; Max-Age=2592000`. Sessions live only in memory.
- Auth rate limit: per IP, 5 consecutive failures cause a 60s lock. Global: more than 30 failures (all IPs) within any 60s window locks all code attempts for 60s; existing sessions and token links are unaffected.
- Text limit: 1 MiB (`1<<20` bytes). File default limit: 4 GiB (`4<<30`).
- Item TTL default 24h, swept every 1 minute.
- Share link: 128-bit random ID. Defaults `max_downloads=1`, `ttl_seconds=600`. Invalid links return HTTP 410.
- SSE: events `item.created` `item.deleted` `items.cleared` `peer.joined` `peer.left` `session.revoked` `resync`. Each event has an `id:` line. Heartbeat comment every 15s. Client backoff starts at 0.5s, doubles, and caps at 3s.
- Security headers on every response: `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. HTML responses also get CSP `default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'`.
- Write endpoints (POST/DELETE, except `/api/local/*`) require that the `Origin` host:port equals the request `Host`. Otherwise return 403.
- Host role = request RemoteAddr is loopback. Only the host may kick. Every authorized device may send, download, share, delete, and clear.
- Public URLs (share links, QR, displayed address) use the server-computed recommended LAN address, never the request Host header.
- Downloads: `Content-Disposition: attachment; filename*=UTF-8''<pct-encoded>`. `?inline=1` is honored only for `image/png|jpeg|gif|webp`.
- Autosave on by default to `~/Downloads/LanDrop/`, with collision suffix ` (1)`, ` (2)`… Exit cleanup never touches it.
- UI default language is English, with no browser auto-detect. The choice is stored in localStorage key `landrop.lang`. Every UI/terminal string goes through i18n; no hardcoded copy. `en` is the base language, and missing keys fall back to it.
- UI style is "Option B" (`.superpowers/brainstorm/*/content/style-b-v2.html`). Accent `#5b5bd6`. Light/dark follows the system with a manual override. Composer autosizes from 2 lines up to 180px, then scrolls. Text longer than 4 lines collapses. The feed fills the remaining viewport height and scrolls on its own.
- Never render user content with `innerHTML` / `dangerouslySetInnerHTML`, except highlight.js output, which escapes its input.
- Copy: use `navigator.clipboard.writeText` and fall back to hidden textarea + `document.execCommand('copy')`. Show a toast on success or failure.

## Review Focus

1. **Hostile or odd filenames** (`../../x`, `a/b\c.txt`, `日志 "v2".log`, empty name). The file must be stored by ID only, and the download header must carry the exact original name, UTF-8 encoded. Covered in Task 9 (`TestUploadOddFilenames`).
2. **Client aborts mid-upload or exceeds `--max-size`.** No item may be created, no temp file may be left behind, and no event may be broadcast. Covered in Task 9 (`TestUploadAbortLeavesNothing`, `TestUploadTooLarge`).
3. **One stalled SSE client among many.** Publish must never block. The stalled subscriber is dropped and the others keep receiving. Covered in Task 5 (`TestSlowSubscriberDropped`).
4. **Two people opening a 1-use share link at the same instant.** Exactly one download may succeed and the other gets 410. Covered in Task 7 (`TestConsumeConcurrent`).
5. **Host restarted while a browser still holds an old cookie, or a reconnect gap older than the ring buffer.** The browser must land on the login page or do a full resync, not loop on errors or show stale items. Covered in Task 8 (`TestStaleCookieGets401`) and Task 5 (`TestSubscribeGapNeedsResync`).

---

## File Structure

```
cmd/landrop/main.go              entry: dispatch "send" subcommand vs serve; --version
internal/version/version.go      Version, Commit, Date (ldflags)
internal/config/config.go        Config + Parse
internal/i18n/i18n.go            T(lang,key,args...), Langs; en.go zh.go ja.go ko.go message maps
internal/diskfree/               Free(path) — free_unix.go / free_windows.go
internal/store/                  Item model, Store (text in memory, files in temp dir), IDs
internal/hub/                    SSE event bus + ring buffer + peer registry
internal/auth/                   code/token/sessions/rate limit
internal/share/                  share links
internal/qr/                     ASCII + SVG QR rendering
internal/dirbrowse/              safe path resolve + listing for --dir
internal/discovery/              interface ranking + mDNS announce
internal/termui/                 startup banner + env hints
internal/autosave/               copy to ~/Downloads/LanDrop with unique names
internal/instance/               ~/.landrop/instance.json read/write/remove
internal/server/                 router, middleware, handlers (one file per area)
internal/app/                    Run(): wiring, listen retry, sweeper, signals, browser open
internal/sendcli/                `landrop send` client
internal/webui/                  embed.go (//go:embed all:dist), dist/.gitkeep
web/                             Preact app (Vite outDir ../internal/webui/dist)
Makefile, README.md
```

---

### Task 1: Repo scaffold, version, embed, Makefile

**Files:**
- Create: `go.mod`, `cmd/landrop/main.go`, `internal/version/version.go`, `internal/webui/embed.go`, `internal/webui/embed_test.go`, `internal/webui/dist/.gitkeep`, `Makefile`
- Modify: `.gitignore` (add `internal/webui/dist/*` and `!internal/webui/dist/.gitkeep`, `web/node_modules/`, `web/test-results/`)

**Interfaces:**
- Produces: `version.String() string` → `"landrop <Version> (<Commit>, <Date>)"`, with defaults `dev`/`none`/`unknown`. `webui.Handler() http.Handler` serves embedded files and falls back to `index.html` for unknown non-`/api` paths (SPA routing). If `dist/index.html` is missing, it serves 503 with the plain text `frontend not built: run "make web"`. `webui.Index() ([]byte, bool)` returns the index bytes for handlers that serve the SPA with a custom status.

- [ ] **Step 1: Write failing test** `internal/webui/embed_test.go`
```go
func TestHandlerMissingIndex(t *testing.T) {
	h := handlerFor(fstest.MapFS{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "make web") { t.Fatal(rec.Code, rec.Body) }
}
func TestHandlerSPAFallback(t *testing.T) {
	h := handlerFor(fstest.MapFS{"index.html": {Data: []byte("<html>app")}, "assets/a.js": {Data: []byte("js")}})
	for path, want := range map[string]string{"/": "<html>app", "/s/abc": "<html>app", "/assets/a.js": "js"} {
		rec := httptest.NewRecorder(); h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || rec.Body.String() != want { t.Fatal(path, rec.Code, rec.Body) }
	}
}
```
- [ ] **Step 2:** `go mod init github.com/lupguo/landrop && go test ./internal/webui/` → FAIL (undefined `handlerFor`).
- [ ] **Step 3: Implement** `handlerFor(fsys fs.FS) http.Handler` plus the exported `Handler()`/`Index()` over `fs.Sub(embedded, "dist")`. Implement `version` with `var Version, Commit, Date` and `cmd/landrop/main.go`, which prints `version.String()` when `--version` is passed (a full flag parse comes in Task 13). Write `Makefile` targets:
  - `web`: `cd web && npm ci && npm run build`
  - `build`: `go build -trimpath -ldflags "-s -w -X github.com/lupguo/landrop/internal/version.Version=$(VERSION) -X ...Commit=$(COMMIT) -X ...Date=$(DATE)" -o bin/landrop ./cmd/landrop`, where `VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)`
  - `all: web build`
  - `test: go test ./... && cd web && npm test`
  - `dist`: loop over the 5 targets with `CGO_ENABLED=0 GOOS=.. GOARCH=..`, output `dist/landrop-<os>-<arch>[.exe]`
  - `size`: `ls -l dist/`
- [ ] **Step 4:** `go test ./internal/webui/ && go build ./... && go run ./cmd/landrop --version` → PASS, prints `landrop dev (none, unknown)`.
- [ ] **Step 5: Commit** `chore: scaffold module, version, embedded web handler, Makefile`

---

### Task 2: Config parsing

**Files:** Create `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
```go
type Config struct {
	Port int; Name string; Dir string; NoAuth bool; TTL time.Duration; Keep bool
	MaxSize int64; NoSave bool; NoOpen bool; Lang string; ShowVersion bool
}
func Parse(args []string, getenv func(string) string) (Config, error)
func ParseSize(s string) (int64, error) // "4GB"→4<<30, "512MB"→512<<20, "100KB", "123" bytes; case-insensitive; also accepts GiB/MiB
```

- [ ] **Step 1: Failing tests**
```go
func TestDefaults(t *testing.T) {
	c, err := Parse(nil, func(string) string { return "" })
	if err != nil || c.Port != 9958 || c.Name != "drop" || c.TTL != 24*time.Hour || c.MaxSize != 4<<30 || c.Lang != "en" || c.NoAuth || c.NoSave { t.Fatalf("%+v %v", c, err) }
}
func TestLangEnvAndFlag(t *testing.T) {
	env := func(k string) string { if k == "LANDROP_LANG" { return "ja" }; return "" }
	c, _ := Parse(nil, env); if c.Lang != "ja" { t.Fatal(c.Lang) }
	c, _ = Parse([]string{"--lang", "ko"}, env); if c.Lang != "ko" { t.Fatal(c.Lang) }
	if _, err := Parse([]string{"--lang", "fr"}, env); err == nil { t.Fatal("want error") }
}
func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"4GB": 4 << 30, "512mb": 512 << 20, "1KiB": 1024, "77": 77} {
		if got, err := ParseSize(in); err != nil || got != want { t.Fatal(in, got, err) }
	}
	if _, err := ParseSize("-1GB"); err == nil { t.Fatal("want error") }
}
func TestBadPortAndDir(t *testing.T) {
	if _, err := Parse([]string{"--port", "70000"}, os.Getenv); err == nil { t.Fatal() }
	if _, err := Parse([]string{"--dir", "/no/such/dir"}, os.Getenv); err == nil { t.Fatal() }
}
```
- [ ] **Step 2:** `go test ./internal/config/` → FAIL.
- [ ] **Step 3:** Implement with `flag.NewFlagSet("landrop", flag.ContinueOnError)`. A flag beats the env var, and the env var beats the default. `--dir` is made absolute and must be an existing directory.
- [ ] **Step 4:** `go test ./internal/config/` → PASS.
- [ ] **Step 5: Commit** `feat(config): flag parsing with defaults from spec`

---

### Task 3: Go i18n (terminal strings)

**Files:** Create `internal/i18n/i18n.go`, `en.go`, `zh.go`, `ja.go`, `ko.go`, `i18n_test.go`

**Interfaces:**
- Produces: `var Langs = []string{"en","zh","ja","ko"}`. `func T(lang, key string, args ...any) string` looks up `lang`, then `en`. A missing key returns the key itself, and `args` are applied with `fmt.Sprintf`. Keys are dotted constants defined as `const` in `i18n.go`, e.g. `KeyBannerAddrs = "banner.addrs"`. Later tasks (12, 13) add keys here. The test enforces parity.
- Initial keys (en copy):
  - `banner.title` "LanDrop is running"
  - `banner.addrs` "Open on another device:"
  - `banner.recommended` "recommended"
  - `banner.mdns` "Or via name: %s"
  - `banner.code` "Access code: %s"
  - `banner.scan` "Scan to join (no code needed):"
  - `banner.noauth` "WARNING: --no-auth is on. Anyone on this network can read and send items."
  - `hint.ap_isolation` "If other devices can't connect, the Wi-Fi may have client/AP isolation enabled."
  - `hint.firewall_windows` "If peers can't connect, allow LanDrop on Private networks, or run: %s"
  - `hint.macos_localnet` "If peers can't connect, enable System Settings → Privacy & Security → Local Network for your terminal."
  - `hint.mdns_failed` "mDNS unavailable (%s); use the IP address above."
  - `err.port_exhausted` "Ports %d–%d are all in use. Pick another with --port."
  - `info.kept` "Temp files kept at: %s"
  - `info.bye` "Stopped. Temporary files removed."
  - `send.not_running` "LanDrop is not running on this machine."
  - `send.ok` "Sent."

- [ ] **Step 1: Failing tests**
```go
func TestParity(t *testing.T) {
	for _, l := range Langs { for k := range catalogs["en"] { if _, ok := catalogs[l][k]; !ok { t.Errorf("%s missing %s", l, k) } } }
}
func TestFallback(t *testing.T) {
	if T("fr", "banner.code", "123456") != "Access code: 123456" { t.Fatal() }
	if T("en", "no.such") != "no.such" { t.Fatal() }
}
```
- [ ] **Step 2:** `go test ./internal/i18n/` → FAIL.
- [ ] **Step 3:** Implement `catalogs map[string]map[string]string` assembled from the four files, then translate zh/ja/ko.
- [ ] **Step 4:** PASS.
- [ ] **Step 5: Commit** `feat(i18n): terminal message catalogs for en/zh/ja/ko`

---

### Task 4: Item store + disk free

**Files:** Create `internal/store/item.go`, `store.go`, `id.go`, `store_test.go`; `internal/diskfree/free_unix.go` (`//go:build !windows`, `unix.Statfs`), `free_windows.go` (`windows.GetDiskFreeSpaceEx`), `free_test.go`

**Interfaces:**
- Produces:
```go
type Kind string // KindText="text", KindFile="file"
type FileMeta struct { Name string `json:"name"`; Size int64 `json:"size"`; MIME string `json:"mime"` }
type From struct { PeerID string `json:"peer_id"`; Name string `json:"name"` }
type Item struct {
	ID string `json:"id"`; Seq uint64 `json:"seq"`; Type Kind `json:"type"`
	Content string `json:"content,omitempty"`; File *FileMeta `json:"file,omitempty"`
	From From `json:"from"`; CreatedAt time.Time `json:"created_at"`; ExpiresAt time.Time `json:"expires_at"`
}
const MaxText = 1 << 20
var ErrTooLarge, ErrEmpty, ErrNotFound error
func New(dir string, ttl time.Duration, now func() time.Time) (*Store, error) // MkdirAll(dir,0700)
func (s *Store) Dir() string
func (s *Store) AddText(from From, content string) (Item, error)              // ErrEmpty if strings.TrimSpace=="" ; ErrTooLarge if len>MaxText
func (s *Store) AddFile(from From, name, mime string, r io.Reader, max int64) (Item, error)
func (s *Store) Get(id string) (Item, bool)
func (s *Store) Open(id string) (*os.File, Item, error)                        // file items only
func (s *Store) List(since uint64) []Item                                       // Seq > since, newest first
func (s *Store) Delete(id string) bool                                          // removes file too
func (s *Store) Clear() []string                                                // ids removed
func (s *Store) Sweep() []string                                                // expired ids removed
func (s *Store) Close(keep bool) error                                          // RemoveAll(dir) unless keep
func diskfree.Free(path string) (uint64, error)
```
- IDs: 26-char Crockford base32 of 48-bit ms timestamp + 80 random bits (ULID-like, no dep). File path is `filepath.Join(dir, id)`, and the user-supplied name is never used in it.
- `AddFile` writes to `<dir>/<id>.part`, counts bytes with a limit of `max+1`, and renames on success. On any error (reader error, over limit) it removes `.part` and returns the error (`ErrTooLarge` when over). Name sanitizing for metadata: take `filepath.Base` after converting `\` to `/`. If the result is empty, `.` or `/`, use `"file"`. An empty MIME is detected from the extension via `mime.TypeByExtension`, otherwise `application/octet-stream`.

- [ ] **Step 1: Failing tests** (fake clock `now := func() time.Time { return t0 }` advanced via a pointer)
```go
func TestTextSeqAndList(t *testing.T) {
	s := newTestStore(t)
	a, _ := s.AddText(From{"p1", "Mac"}, "one"); b, _ := s.AddText(From{"p1", "Mac"}, "two")
	if a.Seq != 1 || b.Seq != 2 || a.ExpiresAt.Sub(a.CreatedAt) != 24*time.Hour { t.Fatal(a, b) }
	l := s.List(0); if len(l) != 2 || l[0].ID != b.ID { t.Fatal(l) }
	if l := s.List(1); len(l) != 1 || l[0].ID != b.ID { t.Fatal(l) }
}
func TestTextLimits(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.AddText(From{}, "  \n "); !errors.Is(err, ErrEmpty) { t.Fatal(err) }
	if _, err := s.AddText(From{}, strings.Repeat("x", MaxText+1)); !errors.Is(err, ErrTooLarge) { t.Fatal(err) }
	if _, err := s.AddText(From{}, strings.Repeat("x", MaxText)); err != nil { t.Fatal(err) }
}
func TestFileRoundTripAndNameSanitize(t *testing.T) {
	s := newTestStore(t)
	it, err := s.AddFile(From{}, `..\..\evil/日志 "v2".log`, "", strings.NewReader("hello"), 10)
	if err != nil || it.File.Name != `日志 "v2".log` || it.File.Size != 5 { t.Fatal(it, err) }
	f, _, _ := s.Open(it.ID); b, _ := io.ReadAll(f); f.Close(); if string(b) != "hello" { t.Fatal(b) }
	entries, _ := os.ReadDir(s.Dir()); if len(entries) != 1 || entries[0].Name() != it.ID { t.Fatal(entries) }
}
func TestFileTooLargeLeavesNothing(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.AddFile(From{}, "a", "", strings.NewReader("123456"), 5); !errors.Is(err, ErrTooLarge) { t.Fatal(err) }
	if entries, _ := os.ReadDir(s.Dir()); len(entries) != 0 || len(s.List(0)) != 0 { t.Fatal(entries) }
}
func TestDeleteClearSweep(t *testing.T) { /* add file+text; Delete(file) removes disk file; advance clock 25h; Sweep returns remaining id; Clear on empty returns 0 ids */ }
func TestCloseRemovesDirUnlessKeep(t *testing.T) { /* Close(false) → dir gone; new store Close(true) → dir exists */ }
```
`TestDeleteClearSweep` must assert: after `Delete(fileID)` the file `filepath.Join(dir,fileID)` does not exist; after advancing 25h `Sweep()` returns exactly the remaining IDs and `List(0)` is empty.
- [ ] **Step 2:** `go get golang.org/x/sys && go test ./internal/store/ ./internal/diskfree/` → FAIL.
- [ ] **Step 3:** Implement. The store uses a `sync.RWMutex`, a `map[string]*Item`, and a slice ordered by seq. `diskfree_test`: `Free(t.TempDir()) > 0`.
- [ ] **Step 4:** `go test -race ./internal/store/ ./internal/diskfree/` → PASS.
- [ ] **Step 5: Commit** `feat(store): in-memory item index with streamed temp files and TTL sweep`

---

### Task 5: Event hub + peer registry

**Files:** Create `internal/hub/hub.go`, `hub_test.go`

**Interfaces:**
- Produces:
```go
type Event struct { ID uint64; Type string; Data any }
type Peer struct { ID string `json:"peer_id"`; Name string `json:"name"`; UA string `json:"ua"`; IP string `json:"ip"`; Host bool `json:"is_host"` }
type Sub struct { C <-chan Event; Done <-chan struct{} } // Done closed when dropped/kicked/unsubscribed
func New(bufSize int) *Hub                                    // app uses 1024
func (h *Hub) Publish(typ string, data any) Event
func (h *Hub) Subscribe(p Peer, lastID uint64) (sub *Sub, replay []Event, resync bool)
func (h *Hub) Unsubscribe(s *Sub)
func (h *Hub) Kick(peerID string)                            // sends "session.revoked" to that peer's subs, then closes them
func (h *Hub) Peers() []Peer                                 // unique by peer ID, sorted by name
```
- Rules: each sub channel has buffer 64. `Publish` does a non-blocking send, and a full channel causes the sub to be dropped (its `Done` is closed). `lastID==0` means a fresh connect: no replay, no resync. If `lastID>0` and the ring no longer holds `lastID+1`, then `resync=true`. A peer's first sub publishes `peer.joined` (data Peer) and the last unsubscribe publishes `peer.left` (data `{peer_id}`). The kick event `session.revoked` is sent only to that peer, is not stored in the ring, and does not consume a ring ID.

- [ ] **Step 1: Failing tests**
```go
func TestReplayAfterLastID(t *testing.T) {
	h := New(8); e1 := h.Publish("item.created", 1); h.Publish("item.created", 2)
	_, replay, resync := h.Subscribe(Peer{ID: "p"}, e1.ID)
	if resync || len(replay) != 1 || replay[0].Data != 2 { t.Fatal(replay, resync) }
}
func TestSubscribeGapNeedsResync(t *testing.T) {
	h := New(2); for i := 0; i < 5; i++ { h.Publish("x", i) }
	if _, _, resync := h.Subscribe(Peer{ID: "p"}, 1); !resync { t.Fatal("want resync") }
}
func TestSlowSubscriberDropped(t *testing.T) {
	h := New(1024); slow, _, _ := h.Subscribe(Peer{ID: "slow"}, 0); fast, _, _ := h.Subscribe(Peer{ID: "fast"}, 0)
	got := 0; done := make(chan struct{})
	go func() { for range fast.C { got++; if got == 200 { close(done); return } } }()
	for i := 0; i < 200; i++ { h.Publish("x", i) } // must not block
	<-done; select { case <-slow.Done: default: t.Fatal("slow not dropped") }
}
func TestPeerJoinLeaveAndKick(t *testing.T) {
	h := New(16); obs, _, _ := h.Subscribe(Peer{ID: "obs"}, 0)
	a, _, _ := h.Subscribe(Peer{ID: "a", Name: "Win"}, 0)
	if ev := <-obs.C; ev.Type != "peer.joined" { t.Fatal(ev) }
	h.Kick("a")
	if ev := <-a.C; ev.Type != "session.revoked" { t.Fatal(ev) }
	<-a.Done
	if ev := <-obs.C; ev.Type != "peer.left" { t.Fatal(ev) }
	if len(h.Peers()) != 1 { t.Fatal(h.Peers()) }
}
```
- [ ] **Step 2:** `go test ./internal/hub/` → FAIL.
- [ ] **Step 3:** Implement with a mutex, a ring slice, and `map[*sub]Peer`.
- [ ] **Step 4:** `go test -race ./internal/hub/` → PASS.
- [ ] **Step 5: Commit** `feat(hub): SSE event bus with replay ring and peer presence`

---

### Task 6: Auth manager

**Files:** Create `internal/auth/auth.go`, `auth_test.go`

**Interfaces:**
- Produces:
```go
const CookieName = "landrop_sid"; const CookieMaxAge = 30 * 24 * 3600
var ErrBadCode, ErrLocked error
type Session struct { ID string; PeerID string; CreatedAt time.Time }
func New(now func() time.Time) *Manager      // random 4-digit Code (leading zeros allowed), 32-hex Token
func (m *Manager) Code() string
func (m *Manager) Token() string
func (m *Manager) CheckCode(ip, code string) error  // constant-time compare; per-IP: 5 consecutive fails → ErrLocked 60s, success resets; global: >30 fails in 60s window → ErrLocked for everyone 60s
func (m *Manager) CheckToken(tok string) bool       // constant-time
func (m *Manager) NewSession() Session               // ID 32-hex, PeerID "p_" + 8 hex
func (m *Manager) Lookup(id string) (Session, bool)
func (m *Manager) RevokePeer(peerID string)
```

- [ ] **Step 1: Failing tests**
```go
func TestCodeFormat(t *testing.T) { m := New(time.Now); if ok, _ := regexp.MatchString(`^\d{4}$`, m.Code()); !ok || len(m.Token()) != 32 { t.Fatal(m.Code(), m.Token()) } }
func TestLockout(t *testing.T) {
	now := time.Unix(0, 0); m := New(func() time.Time { return now })
	for i := 0; i < 5; i++ { if err := m.CheckCode("1.1.1.1", "x"); !errors.Is(err, ErrBadCode) { t.Fatal(i, err) } }
	if err := m.CheckCode("1.1.1.1", m.Code()); !errors.Is(err, ErrLocked) { t.Fatal(err) }
	if err := m.CheckCode("2.2.2.2", m.Code()); err != nil { t.Fatal("other IP must not be locked") }
	now = now.Add(61 * time.Second)
	if err := m.CheckCode("1.1.1.1", m.Code()); err != nil { t.Fatal(err) }
}
func TestGlobalLockout(t *testing.T) {
	now := time.Unix(0, 0); m := New(func() time.Time { return now })
	for i := 0; i < 31; i++ { m.CheckCode(fmt.Sprintf("10.0.%d.%d", i/250, i%250), "x") } // 31 distinct IPs, 1 fail each
	if err := m.CheckCode("10.9.9.9", m.Code()); !errors.Is(err, ErrLocked) { t.Fatal("want global lock", err) }
	now = now.Add(61 * time.Second)
	if err := m.CheckCode("10.9.9.9", m.Code()); err != nil { t.Fatal(err) }
}
func TestSessionsAndRevoke(t *testing.T) {
	m := New(time.Now); s := m.NewSession()
	if _, ok := m.Lookup(s.ID); !ok { t.Fatal() }
	m.RevokePeer(s.PeerID); if _, ok := m.Lookup(s.ID); ok { t.Fatal("revoked") }
}
```
- [ ] **Step 2:** FAIL. **Step 3:** Implement (crypto/rand, `subtle.ConstantTimeCompare`, mutex). **Step 4:** `go test -race ./internal/auth/` PASS.
- [ ] **Step 5: Commit** `feat(auth): access code, link token, sessions, per-IP lockout`

---

### Task 7: Share links

**Files:** Create `internal/share/share.go`, `share_test.go`

**Interfaces:**
- Produces:
```go
var ErrGone error
type Link struct { ID string `json:"id"`; ItemID string `json:"item_id"`; Remaining int `json:"remaining"`; ExpiresAt time.Time `json:"expires_at"` }
func New(now func() time.Time) *Manager
func (m *Manager) Create(itemID string, maxDownloads int, ttl time.Duration) Link // <=0 → 1 / 600s; clamp max 1..100, ttl 60s..24h [my call]
func (m *Manager) Peek(id string) (Link, error)     // ErrGone if missing/expired/exhausted
func (m *Manager) Consume(id string) (Link, error)  // atomic decrement; ErrGone as above
func (m *Manager) DropItem(itemID string)
func (m *Manager) Sweep()                           // remove expired/exhausted
```

- [ ] **Step 1: Failing tests**
```go
func TestDefaultsAndExpiry(t *testing.T) {
	now := time.Unix(0, 0); m := New(func() time.Time { return now })
	l := m.Create("i1", 0, 0); if l.Remaining != 1 || l.ExpiresAt != now.Add(10*time.Minute) || len(l.ID) != 32 { t.Fatal(l) }
	now = now.Add(11 * time.Minute); if _, err := m.Peek(l.ID); !errors.Is(err, ErrGone) { t.Fatal(err) }
}
func TestConsumeConcurrent(t *testing.T) {
	m := New(time.Now); l := m.Create("i1", 1, time.Minute)
	var ok atomic.Int32; var wg sync.WaitGroup
	for i := 0; i < 20; i++ { wg.Add(1); go func() { defer wg.Done(); if _, err := m.Consume(l.ID); err == nil { ok.Add(1) } }() }
	wg.Wait(); if ok.Load() != 1 { t.Fatal(ok.Load()) }
}
func TestDropItem(t *testing.T) { m := New(time.Now); l := m.Create("i1", 3, time.Minute); m.DropItem("i1"); if _, err := m.Peek(l.ID); !errors.Is(err, ErrGone) { t.Fatal(err) } }
```
- [ ] **Step 2–4:** FAIL → implement → `go test -race ./internal/share/` PASS.
- [ ] **Step 5: Commit** `feat(share): one-time share links with count and expiry`

---

### Task 8: HTTP server core — middleware, auth, items, text, SSE, peers

**Files:** Create `internal/server/server.go` (Deps, New, routes), `middleware.go`, `auth_handlers.go`, `items_handlers.go`, `events.go`, `peers_handlers.go`, `json.go`, `server_test.go`, `testutil_test.go`

**Interfaces:**
- Consumes: store (Task 4), hub (Task 5), auth (Task 6), share (Task 7), webui (Task 1).
- Produces:
```go
type Deps struct {
	Store *store.Store; Hub *hub.Hub; Auth *auth.Manager; Share *share.Manager
	NoAuth bool; MaxSize int64; PublicBase string /* e.g. "http://192.168.1.23:9958" */; MDNSURL string
	Dir string /* --dir root or "" */; LocalSecret string; Hostname string
	OnFileStored func(store.Item) /* autosave hook, may be nil */; Web http.Handler; Now func() time.Time
	IsHost func(*http.Request) bool /* nil → RemoteAddr is loopback; tests override */
}
func New(d Deps) http.Handler
```
- Request identity: middleware puts `auth.Session` in the context. The device name comes from header `X-LanDrop-Name` (URL-decoded, trimmed, ≤40 runes). For `/events` it comes from query `name`. Fallbacks: the host uses `Deps.Hostname`, anyone else gets `"Unknown device"`. In NoAuth mode a missing or invalid cookie silently gets a new session plus cookie.
- Routes in this task:

| Route | Auth | Behavior |
|---|---|---|
| `GET /` with `?t=` | none | valid token → new session, Set-Cookie, 302 to `/`; invalid → serve SPA |
| `POST /api/auth` `{code}` | none | 204 + Set-Cookie / 401 `{"error":"bad_code"}` / 429 `{"error":"locked"}` |
| `GET /api/me` | session | `{"peer_id","role":"host"\|"peer","no_auth","public_url","mdns_url","has_dir","max_size"}` |
| `GET /api/items?since=N` | session | `{"items":[...],"last_event_id":<hub last id>}` |
| `POST /api/text` `{content}` | session+origin | 201 Item; 400 `empty`; 413 `too_large`; publishes `item.created` |
| `DELETE /api/items/{id}` | session+origin | 204 / 404; publishes `item.deleted {id}`; `Share.DropItem` |
| `DELETE /api/items` | session+origin | 204; publishes `items.cleared`; drops shares |
| `GET /events?name=` | session | SSE (below) |
| `GET /api/peers` | session | `{"peers":[...]}` |
| `DELETE /api/peers/{peer_id}` | host+origin | 204; `Auth.RevokePeer` + `Hub.Kick`; 403 for non-host |
| everything else | none | `Deps.Web` |

- Unauthenticated `/api/*` and `/events` return 401 `{"error":"unauthorized"}`.
- SSE writes `retry: 500\n`. If `Last-Event-ID` is set and the hub returns `resync`, it emits `event: resync` first. Otherwise it writes the replay. Each event is written as `id: <ID>\nevent: <Type>\ndata: <json>\n\n` and flushed. Every 15s it writes `: ping\n\n`. It returns when the request context ends or `sub.Done` closes.
- Security headers come from Global Constraints. CSP applies only when Content-Type is HTML.

- [ ] **Step 1: Failing tests** (`testutil_test.go` provides `newTestServer(t, mut func(*Deps)) (*httptest.Server, *Deps)` and `login(t, ts) *http.Client`, which is a client with a cookie jar logged in via the token. Requests set `Origin: ts.URL`.)
```go
func TestTokenLoginSetsCookieAndRedirects(t *testing.T) { /* GET /?t=<token> no-redirect client → 302 Location "/", Set-Cookie landrop_sid HttpOnly SameSite=Strict Max-Age=2592000 */ }
func TestCodeLoginAndLockout(t *testing.T) { /* POST /api/auth wrong x5 → 401 each, 6th with right code → 429; */ }
func TestStaleCookieGets401(t *testing.T) { /* request /api/items with Cookie landrop_sid=deadbeef → 401 {"error":"unauthorized"} */ }
func TestOriginRequired(t *testing.T) {
	c := login(t, ts)
	req, _ := http.NewRequest("POST", ts.URL+"/api/text", strings.NewReader(`{"content":"hi"}`))
	resp, _ := c.Do(req); if resp.StatusCode != 403 { t.Fatal(resp.StatusCode) }       // no Origin
	req.Header.Set("Origin", "http://evil.example"); /* → 403 */
}
func TestTextEmptyAndTooLarge(t *testing.T) { /* {"content":"   "} → 400 {"error":"empty"}; 1MiB+1 → 413 {"error":"too_large"}; no item.created published */ }
func TestTextBroadcastViaSSE(t *testing.T) {
	// open /events on client B (reads lines with bufio), POST /api/text "kubectl get pods" from client A with X-LanDrop-Name "Mac",
	// expect within 500ms: "event: item.created" and data JSON with content "kubectl get pods", from.name "Mac"
}
func TestSSEReplayWithLastEventID(t *testing.T) { /* post 2 texts, connect with Last-Event-ID=<first event id> → first event received is the 2nd item.created */ }
func TestRolesAndKick(t *testing.T) {
	// ts listens on 127.0.0.1 so every test client is "host"; override Deps.IsHost to treat requests with header "X-Test-Peer: 1" as non-host
	// peer client (IsHost→false) GET /api/me role "peer"; DELETE /api/peers/x from peer → 403; from host → 204 and peer's next /api/items → 401
}
func TestSecurityHeaders(t *testing.T) { /* GET /api/me has nosniff + no-referrer; GET / (html) has CSP exact string */ }
```
- [ ] **Step 2:** `go test ./internal/server/` → FAIL.
- [ ] **Step 3:** Implement. Origin check: `u, _ := url.Parse(origin); u.Host == r.Host`.
- [ ] **Step 4:** `go test -race ./internal/server/` → PASS.
- [ ] **Step 5: Commit** `feat(server): auth, items, text, SSE, peers with origin and security headers`

---

### Task 9: Upload & download

**Files:** Create `internal/server/files_handlers.go`, `files_test.go`; Modify `internal/server/server.go` (routes)

**Interfaces:**
- Consumes: `Store.AddFile/Open`, `diskfree.Free`, `Deps.OnFileStored`, `Deps.MaxSize`.
- Routes:
  - `POST /api/upload` (session+origin): multipart with a single part named `file`. Before reading, if `r.ContentLength > 0` and `diskfree.Free(store.Dir()) < ContentLength + 64MiB`, return 507 `{"error":"disk_full"}`. Wrap the body in `http.MaxBytesReader(MaxSize + 1MiB)` and stream the part into `AddFile(from, part.FileName(), part.Header content-type, part, MaxSize)`. Too large returns 413 `{"error":"too_large"}`, a missing part returns 400 `{"error":"no_file"}`, and success returns 201 Item. Publish `item.created`, then call `OnFileStored` in a goroutine.
  - `GET /api/items/{id}/raw[?inline=1]` (session): a text item returns `text/plain; charset=utf-8` attachment `<id>.txt`. A file item uses `http.ServeContent` with `Content-Type` = stored MIME. Disposition is `inline` only if `inline=1` and MIME ∈ {image/png, image/jpeg, image/gif, image/webp}, otherwise `attachment; filename="<ascii fallback>"; filename*=UTF-8''<url.PathEscape(name)>`. Missing items return 404.
- Produces: `func contentDisposition(kind, name string) string` (used by Task 10).

- [ ] **Step 1: Failing tests**
```go
func TestUploadDownloadRoundTrip(t *testing.T) { /* multipart "app.log" 3MiB random → 201; GET raw → same sha256, Content-Disposition attachment filename*=UTF-8''app.log; SSE got item.created with file.size */ }
func TestUploadOddFilenames(t *testing.T) {
	for _, n := range []string{`../../x`, `a/b\c.txt`, `日志 "v2".log`, ``} {
		// upload; assert item.file.name == want (x, c.txt, 日志 "v2".log, file); temp dir contains only ID-named files;
		// download header filename* decodes back to item.file.name
	}
}
func TestUploadTooLarge(t *testing.T) { /* Deps.MaxSize=1024, upload 2048 bytes → 413, store.List(0) empty, store dir empty */ }
func TestUploadAbortLeavesNothing(t *testing.T) {
	// pipe-based request body: write multipart header + 100KB, then CloseWithError; expect no item, no *.part left in store dir (poll ≤1s)
}
func TestInlineOnlyForSafeImages(t *testing.T) { /* png ?inline=1 → inline; svg (image/svg+xml) ?inline=1 → attachment; nosniff present */ }
func TestRangeDownload(t *testing.T) { /* Range: bytes=0-9 → 206, 10 bytes */ }
func TestStreamingMemory(t *testing.T) {
	if testing.Short() { t.Skip() }
	// upload 512MiB from a generator reader (no buffering), then download to io.Discard;
	// runtime.ReadMemStats HeapInuse delta < 64MiB measured after runtime.GC()
}
```
- [ ] **Step 2:** FAIL. **Step 3:** Implement. **Step 4:** `go test -race ./internal/server/` PASS, and `go test -run TestStreamingMemory ./internal/server/` PASS.
- [ ] **Step 5: Commit** `feat(server): streamed multipart upload and safe download`

---

### Task 10: QR rendering + share endpoints

**Files:** Create `internal/qr/qr.go`, `qr_test.go`, `internal/server/share_handlers.go`, `share_handlers_test.go`; Modify `server.go`

**Interfaces:**
- Produces:
```go
func qr.ASCII(text string) (string, error) // half-block chars "▀▄█ ", quiet zone 2, recovery Medium
func qr.SVG(text string) ([]byte, error)   // viewBox, black modules on white, shape-rendering crispEdges
```
- Routes:
  - `POST /api/items/{id}/share` (session+origin) `{max_downloads, ttl_seconds}` → 201 `{"id","url": PublicBase+"/s/"+id,"remaining","expires_at"}`. 404 if the item is missing.
  - `GET /api/qr?data=` (session) → `image/svg+xml`. `data` must be ≤ 1024 chars, otherwise 400.
  - `GET /api/s/{id}` (none) → 200 `{"type","name","size","mime","remaining","expires_at"}` without consuming. Return 410 `{"error":"gone"}` if the link or its item is gone.
  - `GET /s/{id}` (none) → SPA `index.html` with status 200 when Peek is OK, otherwise 410 (via `webui.Index()`).
  - `GET /s/{id}/raw` (none) → `Share.Consume`, then serve the item exactly as `/api/items/{id}/raw` would (always attachment; for text, `text/plain` inline so the landing page can fetch it). 410 when gone.

- [ ] **Step 1: Failing tests**
```go
func TestQRASCIIandSVG(t *testing.T) { /* ASCII non-empty, contains "█"; SVG starts with "<svg" and contains "viewBox" */ }
func TestShareOnceThenGone(t *testing.T) {
	// upload file; POST share {} → url starts with Deps.PublicBase+"/s/"; anonymous client:
	// GET /api/s/{id} 200 (twice, still remaining 1); GET /s/{id} 200 html; GET /s/{id}/raw 200 body ok; GET /s/{id}/raw again 410; GET /s/{id} 410
}
func TestShareDiesWithItem(t *testing.T) { /* share then DELETE item → GET /api/s/{id} 410 */ }
func TestShareUsesPublicBaseNotHostHeader(t *testing.T) { /* request share via 127.0.0.1 URL; url uses PublicBase "http://192.168.1.23:9958" */ }
```
- [ ] **Step 2:** FAIL. **Step 3:** `go get github.com/skip2/go-qrcode`, then implement. **Step 4:** `go test -race ./internal/qr/ ./internal/server/` PASS.
- [ ] **Step 5: Commit** `feat(share): share link endpoints and QR rendering`

---

### Task 11: `--dir` read-only browsing

**Files:** Create `internal/dirbrowse/dirbrowse.go`, `dirbrowse_test.go`, `internal/server/dir_handlers.go`, `dir_handlers_test.go`; Modify `server.go`

**Interfaces:**
- Produces:
```go
var ErrOutside error
type Entry struct { Name string `json:"name"`; IsDir bool `json:"is_dir"`; Size int64 `json:"size"`; ModTime time.Time `json:"mod_time"` }
func Resolve(root, rel string) (string, error) // rel uses "/" separators; Clean, join, EvalSymlinks; must equal root or have root+sep prefix (root itself EvalSymlinks'd); else ErrOutside
func List(root, rel string) ([]Entry, error)   // dirs first then name asc; skips entries whose Resolve fails (escaping symlinks)
```
- Routes (session; 404 when `Deps.Dir==""`): `GET /api/dir?path=` → `{"path","entries":[...]}`, and `GET /api/dir/raw?path=` → ServeContent as an attachment. `ErrOutside` gives 403. A directory passed to raw gives 400.

- [ ] **Step 1: Failing tests**
```go
func TestResolveRejectsTraversal(t *testing.T) {
	root := t.TempDir(); os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0600)
	outside := t.TempDir(); os.Symlink(outside, filepath.Join(root, "link"))
	for _, rel := range []string{"../", "../../etc/passwd", "link", "link/x", "/etc/passwd"} {
		if p, err := Resolve(root, rel); !errors.Is(err, ErrOutside) && !(rel == "/etc/passwd" && strings.HasPrefix(p, root)) { t.Fatal(rel, p, err) }
	}
	if _, err := Resolve(root, "a.txt"); err != nil { t.Fatal(err) }
}
func TestListHidesEscapingSymlink(t *testing.T) { /* List(root,"") has a.txt, not "link" */ }
func TestDirEndpoints(t *testing.T) { /* Deps.Dir=root: /api/dir lists; /api/dir/raw?path=../x → 403; Deps.Dir="" → 404 */ }
```
(`/etc/passwd` as rel is treated as relative to root after Clean, so it resolves inside root or reports not found. Either is fine as long as it never leaves root.)
- [ ] **Step 2–4:** FAIL → implement → `go test -race ./internal/dirbrowse/ ./internal/server/` PASS.
- [ ] **Step 5: Commit** `feat(dir): read-only directory browsing with traversal protection`

---

### Task 12: Discovery + terminal banner

**Files:** Create `internal/discovery/addrs.go`, `addrs_test.go`, `mdns.go`; `internal/termui/banner.go`, `banner_test.go`; Modify `internal/i18n/*.go` only if new keys are needed

**Interfaces:**
- Produces:
```go
type IfaceInfo struct { Name string; IP net.IP; Up bool; Loopback bool }
type Addr struct { IP net.IP; Iface string; Recommended bool }
func Rank(in []IfaceInfo) []Addr      // pure; drops loopback, down, IPv6, link-local 169.254/16, and names matching ^(docker|br-|veth|utun|vboxnet|vmnet|tun|tap|llw|awdl|bridge|zt|tailscale) (case-insens.); orders: 192.168/16, 10/8, 172.16/12, others; first gets Recommended=true; en0/eth0/wlan0/"Wi-Fi"/"Ethernet" break ties first
func ListAddrs() ([]Addr, error)     // net.Interfaces → Rank
func Announce(name string, port int, ips []net.IP) (stop func(), err error) // zeroconf.RegisterProxy(name, "_http._tcp", "local.", port, name, ipStrings, []string{"path=/"}, nil)
type BannerInfo struct { Lang string; Addrs []Addr; Port int; MDNSHost string /* "" if failed */; MDNSErr error; Code string; TokenURL string; NoAuth bool; GOOS string; Version string }
func termui.Print(w io.Writer, b BannerInfo)
```
- Banner layout (all copy via `i18n.T`): the title and version, the address list (recommended marked), the mDNS line or the `hint.mdns_failed` line, the access code (omitted if NoAuth, which prints the yellow `banner.noauth` instead with ANSI `\x1b[33m`), the QR of TokenURL under `banner.scan`, then the hints (`hint.ap_isolation` always; `hint.firewall_windows` with `netsh advfirewall firewall add rule name="LanDrop" dir=in action=allow protocol=TCP localport=<port>` when GOOS=windows; `hint.macos_localnet` when darwin).

- [ ] **Step 1: Failing tests**
```go
func TestRank(t *testing.T) {
	in := []IfaceInfo{
		{"docker0", net.ParseIP("172.17.0.1"), true, false}, {"lo0", net.ParseIP("127.0.0.1"), true, true},
		{"en7", net.ParseIP("10.0.0.5"), true, false}, {"en0", net.ParseIP("192.168.1.23"), true, false},
		{"utun3", net.ParseIP("100.64.0.2"), true, false}, {"en1", net.ParseIP("169.254.3.3"), true, false},
	}
	out := Rank(in)
	if len(out) != 2 || out[0].IP.String() != "192.168.1.23" || !out[0].Recommended || out[1].Recommended { t.Fatal(out) }
}
func TestBannerWindowsAndNoAuth(t *testing.T) {
	var b bytes.Buffer; termui.Print(&b, BannerInfo{Lang: "en", Port: 9958, NoAuth: true, GOOS: "windows", TokenURL: "http://x/?t=1", Addrs: []Addr{{IP: net.ParseIP("192.168.1.23"), Recommended: true}}})
	s := b.String()
	for _, want := range []string{"http://192.168.1.23:9958", "--no-auth", "netsh advfirewall", "localport=9958"} { if !strings.Contains(s, want) { t.Fatal(want, s) } }
	if strings.Contains(s, "Access code") { t.Fatal("code shown in no-auth") }
}
func TestBannerZh(t *testing.T) { /* Lang "zh" contains the zh translation of banner.code with the code */ }
```
- [ ] **Step 2:** FAIL. **Step 3:** `go get github.com/grandcat/zeroconf`, then implement. `Announce` has no unit test and is exercised in Task 17's manual check. **Step 4:** PASS.
- [ ] **Step 5: Commit** `feat(discovery,termui): address ranking, mDNS announce, localized banner`

---

### Task 13: App wiring, autosave, instance file, local API, `landrop send`

**Files:** Create `internal/autosave/autosave.go`, `autosave_test.go`; `internal/instance/instance.go`, `instance_test.go`; `internal/server/local_handlers.go`, `local_handlers_test.go`; `internal/app/app.go`, `listen.go`, `browser.go`, `app_test.go`; `internal/sendcli/send.go`, `send_test.go`; Modify `cmd/landrop/main.go`, `internal/server/server.go`

**Interfaces:**
- Produces:
```go
func autosave.UniquePath(dir, name string) string            // "a.log" → "a (1).log" → "a (2).log" if taken
func autosave.Save(srcPath, name, dir string) (string, error) // MkdirAll 0755, copy via io.Copy to UniquePath
type instance.Info struct { PID int `json:"pid"`; Port int `json:"port"`; Secret string `json:"secret"` }
func instance.DefaultPath() string                            // ~/.landrop/instance.json
func instance.Write(path string, i Info) error                // dir 0700, file 0600
func instance.Read(path string) (Info, error)
func instance.Remove(path string)
func app.Listen(port, tries int) (net.Listener, int, error)  // 0.0.0.0:port..port+tries-1
func app.Run(ctx context.Context, cfg config.Config, out io.Writer) error
func sendcli.Run(args []string, stdin io.Reader, out io.Writer, lang string) error
```
- Local API (route group `/api/local/`): RemoteAddr must be loopback and `X-LanDrop-Secret` must equal `Deps.LocalSecret` (constant-time), otherwise 403. Origin check is skipped. `POST /api/local/text` takes a raw body as text. `POST /api/local/upload?name=` takes a raw body streamed to `AddFile`. The From name is `Hostname + " (CLI)"`. Both publish `item.created`.
- `sendcli.Run`: `send <file>` uploads the file. `send -` reads stdin into a buffer of up to `MaxText+1` bytes. If it fits it is sent as text, otherwise it streams `buffered + rest` as the upload `stdin.txt`. A missing instance file or a connection refused prints `i18n.T(lang,"send.not_running")` and returns an error.
- `app.Run` order:
  1. `Listen(cfg.Port, 10)` (on failure return an error using `err.port_exhausted`).
  2. Create the store at `os.TempDir()/landrop-<pid>`, plus hub(1024), auth, and share.
  3. Run `ListAddrs`. `PublicBase` = `http://<recommended IP>:<port>`, falling back to `http://127.0.0.1:<port>`.
  4. `Announce(cfg.Name)`; on error `MDNSURL=""`.
  5. Generate the secret and write the instance file.
  6. Build `server.New` with `OnFileStored` = autosave into `~/Downloads/LanDrop` unless `NoSave`.
  7. Start the sweeper ticker (1m) that runs `Store.Sweep` and publishes `item.deleted` for each ID, then runs `Share.Sweep`.
  8. Print the banner.
  9. Unless `NoOpen`, open `http://127.0.0.1:<port>/?t=<token>` (`open`, `xdg-open`, `rundll32 url.dll,FileProtocolHandler`).
  10. Serve until ctx is done.
  11. Shutdown(5s), stop mDNS, remove the instance file, `Store.Close(cfg.Keep)`, and print `info.kept` or `info.bye`.
- `main.go`: if `os.Args[1]=="send"`, call `sendcli.Run`. Otherwise `config.Parse`. `--version` prints and exits. Otherwise run `app.Run` with `signal.NotifyContext(SIGINT, SIGTERM)`.

- [ ] **Step 1: Failing tests**
```go
func TestUniquePath(t *testing.T) { d := t.TempDir(); os.WriteFile(filepath.Join(d, "a.log"), nil, 0600); if got := UniquePath(d, "a.log"); filepath.Base(got) != "a (1).log" { t.Fatal(got) } }
func TestInstanceRoundTripPerms(t *testing.T) { /* Write then Read equal; file mode 0600 on non-windows */ }
func TestLocalAPI(t *testing.T) { /* POST /api/local/text without secret → 403; with secret → 201 and SSE item.created with from.name ending " (CLI)" */ }
func TestListenSkipsBusyPort(t *testing.T) { l1, _ := net.Listen("tcp", "0.0.0.0:0"); p := l1.Addr().(*net.TCPAddr).Port; l2, got, err := Listen(p, 3); if err != nil || got == p { t.Fatal(got, err) }; l2.Close(); l1.Close() }
func TestListenExhausted(t *testing.T) { /* occupy p and p+1, Listen(p,2) → error */ }
func TestRunEndToEnd(t *testing.T) {
	// cfg: Port free, NoOpen, NoSave=false with HOME=t.TempDir(); run app.Run in goroutine with cancelable ctx; poll instance file;
	// sendcli.Run([]string{"send","-"}, strings.NewReader("hello"), ...) → nil; upload via sendcli a temp file "x.log"
	// assert ~/Downloads/LanDrop/x.log exists (poll ≤2s); cancel ctx; assert temp store dir removed and instance file removed
}
```
- [ ] **Step 2:** FAIL. **Step 3:** Implement. **Step 4:** `go test -race ./...` PASS.
- [ ] **Step 5: Manual smoke:** `go run ./cmd/landrop --no-open` prints the banner with a QR, `curl` against the token URL returns 302, and Ctrl-C prints the bye line.
- [ ] **Step 6: Commit** `feat(app): serve wiring, autosave, instance file, landrop send`

---

### Task 14: Frontend foundation — Vite/Preact scaffold, i18n, API client, live store, login page

**Files:** Create `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/index.html`, `web/src/main.tsx`, `web/src/app.tsx`, `web/src/styles.css`, `web/src/i18n/{index.ts,en.ts,zh.ts,ja.ts,ko.ts}`, `web/src/i18n/i18n.test.ts`, `web/src/api.ts`, `web/src/live.ts`, `web/src/lib/{device.ts,format.ts,device.test.ts,format.test.ts}`, `web/src/components/{LoginPage.tsx,LangSwitch.tsx,Toast.tsx}`

**Interfaces:**
- Consumes: HTTP API from Tasks 8–11 (exact routes and JSON above).
- Produces:
```ts
// i18n/index.ts
export type Lang = 'en'|'zh'|'ja'|'ko'; export const LANGS: Lang[]
export const lang: Signal<Lang>                       // init from localStorage 'landrop.lang' (try/catch), default 'en'
export function setLang(l: Lang): void
export function t(key: keyof typeof en, params?: Record<string,string|number>): string  // "{name}" placeholders; fallback en
// api.ts — every call sets header X-LanDrop-Name: encodeURIComponent(deviceName.value); throws ApiError{status, code}
export const api: { me(); items(since?: number); sendText(content: string); del(id: string); clear(); share(id: string, maxDownloads: number, ttlSeconds: number); peers(); kick(peerId: string); auth(code: string); dir(path: string); shareMeta(id: string) }
export function rawUrl(id: string, inline?: boolean): string
export function upload(file: File, onProgress: (loaded: number, total: number) => void): { promise: Promise<Item>; abort(): void } // XHR
// live.ts
export const items: Signal<Item[]>; export const peers: Signal<Peer[]>; export const connected: Signal<boolean>; export const me: Signal<Me|null>
export function startLive(): void   // EventSource('/events?name=..'); on item.created prepend (dedupe by id); item.deleted remove; items.cleared empty; peer.* update;
                                    // session.revoked → me=null (shows login); resync or reconnect → api.items(0) replace; backoff 500ms×2 cap 3000 via manual reconnect after onerror (close + setTimeout)
// lib/device.ts
export function guessDeviceName(ua: string): string   // "Windows · Edge", "Mac · Safari", "iPhone · Safari", "Android · Chrome", "Linux · Firefox", fallback "Browser"
export const deviceName: Signal<string>                // localStorage 'landrop.name' or guess
// lib/format.ts
export function formatSize(bytes: number): string     // 0 → "0 B", 1536 → "1.5 KB", 13002345 → "12.4 MB", 4<<30 → "4.0 GB"
export function formatSpeed(bytesPerSec: number): string // formatSize + "/s"
```
- `vite.config.ts`: `build.outDir: '../internal/webui/dist'`, `emptyOutDir: true`, dev `server.proxy` for `/api`, `/events`, `/s` → `http://127.0.0.1:9958`.
- `package.json` scripts: `dev`, `build` (`tsc --noEmit && vite build`), `test` (`vitest run`), `e2e` (`playwright test`). Deps: `preact`, `@preact/signals`, `highlight.js`. Dev deps: `vite`, `@preact/preset-vite`, `typescript`, `vitest`, `@playwright/test`.
- Routing in `app.tsx` uses `location.pathname`. `/s/:id` renders `SharePage` (Task 16). Otherwise it calls `api.me()`: a 401 renders `LoginPage`, otherwise `MainPage` (Task 15). Until then a stub `<div>` is fine.
- `LoginPage`: a 4-digit numeric input (`inputmode="numeric"`, autofocus, auto-submit at 4 digits) and error text for `bad_code`/`locked`. `LangSwitch` sits top-right.
- CSS tokens on `:root` copy the Option B mockup variables (`--bg --card --fg --mut --line --code --acc --accbg`), with a dark set under `@media (prefers-color-scheme: dark)` and an override via `[data-theme=light|dark]` on `<html>`.

- [ ] **Step 1: Failing tests** (Vitest)
```ts
test('i18n parity', () => { for (const l of LANGS) for (const k of Object.keys(en)) expect(dicts[l][k], `${l}.${k}`).toBeTruthy() })
test('t params + fallback', () => { setLang('en'); expect(t('feed.expand', { n: 14 })).toBe('Show all (14 lines)') })
test('guessDeviceName', () => {
  expect(guessDeviceName('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 Edg/130.0')).toBe('Windows · Edge')
  expect(guessDeviceName('Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1')).toBe('iPhone · Safari')
  expect(guessDeviceName('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36')).toBe('Mac · Chrome')
})
test('formatSize', () => { expect(formatSize(0)).toBe('0 B'); expect(formatSize(1536)).toBe('1.5 KB'); expect(formatSize(13002345)).toBe('12.4 MB') })
```
The en dictionary must include at least `feed.expand` = `"Show all ({n} lines)"`. All UI keys used in Tasks 15–16 are added to all four dicts as those tasks go.
- [ ] **Step 2:** `cd web && npm install && npm test` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `npm test` PASS, then `npm run build` succeeds and writes `internal/webui/dist/index.html`. Run `make build && ./bin/landrop --no-open`, open the printed address: the login page renders, the correct code reaches the stub main view, and the language switch changes the copy.
- [ ] **Step 5: Commit** `feat(web): Preact scaffold, i18n, API client, live SSE store, login page`

---

### Task 15: Frontend main page — top bar, composer, feed, uploads, copy

**Files:** Create `web/src/components/{MainPage.tsx,TopBar.tsx,ConnBanner.tsx,Composer.tsx,Feed.tsx,ItemCard.tsx,TextBody.tsx,FileBody.tsx,UploadRow.tsx}`, `web/src/lib/{clipboard.ts,detect.ts,linkify.ts,uploads.ts,detect.test.ts,linkify.test.ts,uploads.test.ts}`, `web/src/lib/highlight.ts`

**Interfaces:**
- Consumes: Task 14 `api`, `upload`, `items`, `connected`, `me`, `t`, `formatSize`, `formatSpeed`, `deviceName`.
- Produces:
```ts
export function looksLikeCode(text: string): boolean   // ≥2 lines AND (≥30% lines start with 2+ spaces/tab OR contains any of [{};] ending lines OR /^\s*(\$|#|>|PS [A-Z]:\\)/m OR keyword /\b(func|def|class|import|const|let|var|return|select|from|kubectl|docker|git)\b/), OR single line starting with "$ " / "kubectl " / "git " / "docker "
export function linkify(text: string): Array<{ text: string; href?: string }>  // http(s) URLs only; trailing ).,;! excluded
export async function copyText(text: string): Promise<boolean>   // clipboard API if isSecureContext, else textarea+execCommand
export function highlight(code: string): string                  // hljs core + bash,go,javascript,typescript,json,yaml,python,sql; highlightAuto
export const uploads: Signal<UploadTask[]>; export function enqueue(files: File[]): void // concurrency 3
export type UploadTask = { id: string; name: string; size: number; loaded: number; speed: number; state: 'queued'|'uploading'|'done'|'error'; error?: string }
```
- Behavior:
  - **TopBar**: logo "Lan**Drop**", shows `me.mdns_url || me.public_url`, an online-count pill (opens PeersPanel from Task 16), a settings button, and a `--no-auth` warning strip if `me.no_auth`.
  - **ConnBanner**: a red strip with `t('conn.lost')` while `!connected`.
  - **Composer**: a textarea (rows=2) autosizes via `scrollHeight` capped at 180px, with `overflowY` auto beyond. ⌘/Ctrl+Enter sends. The char count shows bottom-right. Over 1 MiB (UTF-8 bytes) shows `t('composer.too_large')` with a button "send as file" that enqueues a `new File([text], 'text.txt')`. Buttons: file picker (multiple) and send. Drag-over anywhere on the page highlights the composer's dashed border, and drop enqueues the files.
  - **Global paste**: a `document` `paste` listener when `document.activeElement` is not an input/textarea. Files in `clipboardData.files` are enqueued, otherwise the text is sent.
  - **Feed**: header "N items" and "Clear all" (confirm). A scroll container with `flex:1; overflow-y:auto`. Items newest first. UploadRow entries are pinned at the top while in flight.
  - **ItemCard**: avatar (first letter of from.name, color = hash(peer_id) → one of 8 fixed hues), "from X · HH:mm". Actions: copy/download, share, delete.
  - **TextBody**: `white-space: pre-wrap`. If `looksLikeCode` it uses the mono font and `highlight()` via `dangerouslySetInnerHTML` (the only allowed use). Otherwise it renders linkify segments as text/`<a target=_blank rel="noopener noreferrer">`. If the line count is > 4 it collapses to `max-height: 4 lines` with a gradient and toggles `t('feed.expand',{n})` / `t('feed.collapse')`. Copy always copies `item.content`.
  - **FileBody**: an icon with the extension uppercased (≤4 chars), name, and size. Image MIME (png/jpeg/gif/webp) shows a thumbnail `<img src=rawUrl(id,true) loading=lazy>` with max-height 160px. Download is `<a href=rawUrl(id) download>`.
  - **UploadRow**: name, `loaded/total`, speed, a progress bar, and an error state with retry.
- Toasts: copy success/failure, send errors (mapped from ApiError.code: `too_large`, `disk_full`, `empty`).

- [ ] **Step 1: Failing tests**
```ts
test('looksLikeCode', () => {
  expect(looksLikeCode('kubectl get pods -n prod | grep Error')).toBe(true)
  expect(looksLikeCode('func a() {\n  return 1\n}')).toBe(true)
  expect(looksLikeCode('see you at 5pm')).toBe(false)
  expect(looksLikeCode('Line one of a note.\nLine two of a note.')).toBe(false)
})
test('linkify', () => {
  expect(linkify('open https://a.b/c?x=1). ok')).toEqual([{ text: 'open ' }, { text: 'https://a.b/c?x=1', href: 'https://a.b/c?x=1' }, { text: '). ok' }])
  expect(linkify('javascript:alert(1)')).toEqual([{ text: 'javascript:alert(1)' }])
})
test('upload queue concurrency 3', async () => { /* mock upload() with manual resolvers; enqueue 5 files → exactly 3 'uploading'; resolve one → 4th starts */ })
```
- [ ] **Step 2:** `npm test` FAIL. **Step 3:** Implement. **Step 4:** `npm test` PASS, `npm run build` OK.
- [ ] **Step 5: Manual check** with `make all && ./bin/landrop`, comparing against the Option B mockup in two browsers. Verify:
  - The composer grows and then scrolls past ~8 lines.
  - A 14-line log collapses and expands.
  - About 5 items are visible and the feed scrolls on its own.
  - Dragging 3 files shows progress and speed.
  - Pasting a screenshot outside the textarea uploads it.
  - Copy works on `http://<LAN IP>` (execCommand path).
  - Dark mode follows the system.
- [ ] **Step 6: Commit** `feat(web): main page with composer, feed, uploads, copy, highlight`

---

### Task 16: Frontend share dialog, share landing page, peers panel, settings, dir browser

**Files:** Create `web/src/components/{ShareDialog.tsx,SharePage.tsx,PeersPanel.tsx,Settings.tsx,DirBrowser.tsx}`; Modify `MainPage.tsx`, `TopBar.tsx`, `ItemCard.tsx`, `app.tsx`, i18n dicts

**Interfaces:**
- Consumes: `api.share/shareMeta/peers/kick/dir`, `/api/qr?data=`, `/s/{id}/raw`, `me.role`, `me.has_dir`, `peers`, `deviceName`, `lang`.
- Behavior:
  - **ShareDialog** (from ItemCard "share"): a downloads select (1, 3, 10, default 1) and an expiry select (10 min, 1 h, 24 h, default 10 min). Create calls `api.share`, then shows the URL (readonly input + copy button), `<img src="/api/qr?data=<encoded url>">`, and the expiry time.
  - **SharePage** (`/s/:id`, no login): `LangSwitch` top-right. Calls `api.shareMeta(id)`. A 410 shows `t('share.gone')`. A file shows name, size and a download button (`/s/{id}/raw`, `download` attr). Text fetches `/s/{id}/raw` on load (this consumes the link) and shows the text with a copy button.
  - **PeersPanel**: a dropdown listing `peers` (name, IP, short UA, "this device"/"host" badges). A kick button appears only if `me.role==='host'` and the peer is not self.
  - **Settings**: device name (writes `landrop.name` and updates `deviceName`), theme (system/light/dark, stored in `landrop.theme` and applied as `data-theme`), language (`setLang`), and an informational line `t('settings.autosave_note')`.
  - **DirBrowser**: a tab "Messages | Folder" shown when `me.has_dir`. It has breadcrumbs and an entries list (dirs navigate, files link to `/api/dir/raw?path=`).

- [ ] **Step 1: Failing test** (Vitest + `@testing-library/preact`, added as a dev dep)
```ts
test('SharePage shows gone state on 410', async () => { /* mock fetch /api/s/abc → 410; render <SharePage id="abc"/>; expect text t('share.gone') */ })
test('PeersPanel hides kick for peer role', () => { /* me.role='peer' → no button with t('peers.kick') */ })
```
- [ ] **Step 2–4:** FAIL → implement → `npm test` PASS, `npm run build` OK.
- [ ] **Step 5: Manual check:**
  - Share a file and open the link in a private window: the first download works and a reload shows the gone page.
  - Scan the QR with a phone.
  - Kick a second browser from the host: it lands on the login page.
  - `--dir ~/Desktop` shows the Folder tab.
- [ ] **Step 6: Commit** `feat(web): share dialog and landing page, peers panel, settings, folder browser`

---

### Task 17: E2E smoke, README, cross-build verification

**Files:** Create `web/playwright.config.ts`, `web/e2e/smoke.spec.ts`, `README.md`; Modify `Makefile` (add `e2e` target that builds, starts `bin/landrop --no-open --port 18848 --lang en` with `HOME` set to a temp dir, then runs Playwright)

**Interfaces:**
- Consumes: the whole app. The `e2e` target redirects the server's stdout to `web/test-results/server.log` and passes that path to Playwright as env `LANDROP_LOG`. The spec reads the access code (regex `Access code: (\d{4})`) and the base URL from that log.

- [ ] **Step 1: Write e2e** `web/e2e/smoke.spec.ts` (chromium only):
  1. **Login by code**: open the base URL, read the code from the log, type it, and see the composer.
  2. **Text across two contexts**: context A sends `kubectl get pods`, and context B sees it within 1s and clicks Copy. Grant `clipboard-read` on localhost, then assert `navigator.clipboard.readText()` equals the text.
  3. **Collapse**: send 14 lines, see `Show all (14 lines)`, click it, and see `Collapse`.
  4. **Upload**: `setInputFiles` with a 2MB buffer named `app.log`. The item appears in context B, and the download event's suggested filename is `app.log`.
  5. **Share once**: share, open the URL in a new context, download succeeds, reload shows the gone text.
- [ ] **Step 2:** `make e2e` → all 5 pass (fix any defects found in earlier tasks' code, with a regression unit test).
- [ ] **Step 3: README.md** in English, with a 中文 section link:
  - quick start (`make all`, `./bin/landrop`), all flags, `landrop send` usage
  - unsigned binary notes (`xattr -d com.apple.quarantine landrop`; Windows SmartScreen "More info → Run anyway")
  - the Windows firewall `netsh` rule, the macOS Local Network permission, and AP isolation
  - security notes (plain HTTP on LAN; access code; share links)
- [ ] **Step 4: Verify release constraints:** `make all dist && ls -l dist/`. Each binary must be ≤ 15MB. Run `file dist/*` to confirm the arch, and `go vet ./... && go test -race ./... && (cd web && npm test)` must be all green. Idle RSS check: start `bin/landrop --no-open`, wait 5s, `ps -o rss= -p <pid>` ≤ 30720 KB.
- [ ] **Step 5: Manual acceptance (spec §9)** with Mac as host and Windows Chrome/Edge as peer, walking PRD §7 flows 7.1–7.3:
  - phone QR join
  - copy fallback on Win Chrome/Edge, Mac Safari/Chrome, and Firefox over `http://<LAN IP>`
  - a 500MB transfer at ≥ 80MB/s on gigabit
  - `drop.local` resolving from Mac/iPhone

  Record the results in `docs/acceptance-v0.1.md`.
- [ ] **Step 6: Commit** `test: e2e smoke; docs: README and acceptance notes`

---

## Spec Coverage Map

| Spec | Task |
|---|---|
| F-01, F-06, port retry | 2, 13 |
| F-02, F-04, §6.1 hints, F-15 warning | 12, 13 |
| F-03 mDNS | 12, 13 |
| F-05 auto-open | 13 |
| F-10–F-13, §6.2 | 6, 8, 16 |
| F-20–F-26 | 8, 15 |
| F-30–F-33, F-36 | 9, 15 |
| F-40–F-43 | 7, 10, 16 |
| F-50–F-53 | 4, 8, 13, 15 |
| F-60 | 11, 16 |
| F-61 | 13 |
| F-62 autosave | 13 |
| §3.2 i18n (UI + terminal, en default, 4 langs) | 3, 14–16 |
| §5 Option B, autosize, collapse, 5-item feed | 14, 15 |
| §7 security headers/CSP | 8 |
| §9 tests, size, memory | 9, 17 |
| §3.1 README unsigned-binary notes | 17 |
