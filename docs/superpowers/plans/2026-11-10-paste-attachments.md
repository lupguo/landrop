# Paste Attachments & Multi-file Messages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Paste, pick, or drop files into a pending tray above the composer, then send the tray plus the text as one message card. Every image in it can be downloaded on its own, opened in an in-page image viewer, and shared as one link that is limited only by expiry.

**Architecture:** The store gains a `message` item kind and a per-peer staging area. A client uploads each attachment to staging (`POST /api/attachments`), then commits all of them atomically with `POST /api/messages`. Only that commit publishes `item.created`, so peers never see half a message. The web client keeps the draft (text plus pending files) in `lib/draft.ts`. When the user sends, the draft picks the item type: text only becomes `text`, one file with no text becomes the existing `file`, and anything else becomes `message`. All paste, pick, and drop entry points feed the draft instead of uploading at once.

**Tech Stack:** Go 1.26 stdlib; Preact 10 + `@preact/signals`, TypeScript, Vitest + @testing-library/preact, Playwright. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-11-10-paste-attachments-design.md` (an increment on `docs/superpowers/specs/2026-11-09-landrop-mvp-design.md`)

## Global Constraints

- No new Go or npm dependencies. `CGO_ENABLED=0`.
- Max attachments per message: **20** (`store.MaxAttachments`, `MAX_ATTACHMENTS` in the web code). Each attachment uses the existing per-file limit `MaxSize` (default 4 GiB). Message text uses the existing `store.MaxText` (1 MiB).
- Staged attachments that no message references expire after **30 minutes** (`store.StageTTL`). They are removed by the existing 1-minute sweep.
- Type selection when sending: text only → `text` (`POST /api/text`); exactly 1 attachment and blank text → `file` (`POST /api/upload`); everything else → `message`.
- Write endpoints keep `origin` + `session` middleware. Error codes reuse `too_large`, `disk_full`, `no_file`, `upload_failed`, `empty`. New codes are `too_many` and `bad_attachment`.
- Inline serving only for the existing `inlineMIMEs` whitelist (png/jpeg/gif/webp). Everything else is served as `attachment`.
- `message` share links: expiry only, no download count (`Remaining == -1` means unlimited). `text`/`file` share links stay exactly as they are.
- Every new UI string goes into **all four** dicts (`en`, `zh`, `ja`, `ko`). `i18n.test.ts` parity enforces this.
- Pending tray sits **above** the textarea (design option A). Image thumbnails use `URL.createObjectURL` and are revoked on remove or after a successful send. No base64.
- Pasting plain text inside the textarea keeps browser default behaviour.
- Screenshot rename: prefix from i18n key `composer.shot_prefix` (`en` "screenshot", `zh` "截图"). Format `<prefix>-YYYYMMDD-HHMMSS[-n].<ext>`. This applies only to pasted files whose name is empty or matches `/^image(\.\w+)?$/i`.

## Review Focus

1. **A clipboard holding both a file and text** (Finder copy, or "copy image" in a browser, which also carries text or HTML): only the file enters the tray, nothing is inserted into the textarea, and the draft text stays unchanged. Owned by Task 6.
2. **Retry after staging expired**: a send fails, the user retries more than 30 minutes later, and the server answers `bad_attachment`. The client drops every `stagedId` and re-uploads the files on the next send instead of looping on the error. Owned by Task 4.
3. **Several screenshots pasted within the same second** get distinct names (`-2`, `-3`, …), so downloads and autosave do not overwrite each other. Owned by Task 4.
4. **Hostile attachment names** (`../../etc/x.png`, `a\b.png`, an empty name): staging sanitizes them exactly as `AddFile` does, and `Content-Disposition` stays well formed. Owned by Task 1.
5. **A message deleted while its share page is open**: share attachment downloads return 410, and the list endpoints no longer show it. Owned by Task 3.

---

### Task 1: Store — `message` kind and staging

**Files:**
- Modify: `internal/store/item.go`
- Modify: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces:
  - `const KindMessage Kind = "message"`; `const MaxAttachments = 20`; `const StageTTL = 30 * time.Minute`
  - `var ErrTooMany`, `ErrBadAttachment` (new sentinel errors)
  - `type Attachment struct { ID, Name string; Size int64; MIME string }` with JSON tags `id,name,size,mime`
  - `Item.Attachments []Attachment \`json:"attachments,omitempty"\``
  - `func (s *Store) Stage(owner, name, mimeType string, r io.Reader, max int64) (Attachment, error)`: the bytes live at `filepath.Join(s.Dir(), att.ID)`. **Invariant:** each attachment's file path is `Dir()/<attachment ID>`, which autosave relies on.
  - `func (s *Store) AddMessage(from From, content string, ids []string) (Item, error)`
  - `func (s *Store) OpenAttachment(itemID, attID string) (*os.File, Attachment, Item, error)`

- [ ] **Step 1: Write failing tests**

```go
func TestStageAndAddMessage(t *testing.T)            // Stage 2 files as owner "p1"; AddMessage(From{PeerID:"p1"}, "hi", ids) → Type==KindMessage, Content=="hi", len(Attachments)==2 in given order; List(0) has exactly 1 item
func TestAddMessageEmptyContentAllowed(t *testing.T) // content "" with 2 ids → ok
func TestAddMessageErrors(t *testing.T)              // table: 0 ids → ErrEmpty; 21 ids → ErrTooMany; content > MaxText → ErrTooLarge; unknown id / id staged by "p2" / same id twice / id already used by a prior message → ErrBadAttachment; after any error the valid staged ids are still usable
func TestStageSanitizesName(t *testing.T)            // "../../etc/x.png"→"x.png", `a\b.png`→"b.png", ""→"file"; MIME falls back from extension
func TestStageTooLarge(t *testing.T)                 // max=3, 4 bytes → ErrTooLarge, no file left in Dir()
func TestOpenAttachment(t *testing.T)                // bytes round-trip; wrong attID or non-message item → ErrNotFound
func TestMessageDeleteRemovesFiles(t *testing.T)     // Delete, Clear and Sweep (expired) each remove every attachment file from Dir()
func TestSweepDropsStaleStaging(t *testing.T)        // fake clock: staged file survives Sweep at +29m, removed at +31m; Sweep's returned ids don't include it
```

- [ ] **Step 2: Run** `go test ./internal/store/ -run 'Stage|Message|OpenAttachment'`. Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement.** Extract the `.part` write/rename/limit logic from `AddFile` into `func (s *Store) writeBlob(id string, r io.Reader, max int64) (int64, error)`, which both `AddFile` and `Stage` use. Staging is a `map[string]staged{att Attachment; owner string; at time.Time}` guarded by `mu`. `AddMessage` validates every id under the lock before it changes anything, then removes them from staging and inserts the item (factor `insert` into a locked/unlocked pair so this stays one critical section). `removeFile` deletes each attachment file for `KindMessage`. `Sweep` also purges stale staging.

- [ ] **Step 4: Run** `go test ./internal/store/`. Expected: PASS (existing tests included).

- [ ] **Step 5: Commit** `feat(store): message items with staged attachments`

---

### Task 2: Server — stage, commit, and download attachments

**Files:**
- Modify: `internal/server/files_handlers.go`, `internal/server/items_handlers.go`, `internal/server/server.go`
- Create: `internal/server/message_handlers.go`
- Test: `internal/server/message_handlers_test.go`

**Interfaces:**
- Consumes: Task 1 `Stage`, `AddMessage`, `OpenAttachment`, `ErrTooMany`, `ErrBadAttachment`.
- Produces (HTTP):
  - `POST /api/attachments` (origin+session): a multipart part named `file`, the same limits and disk checks as `/api/upload` → `201` with an `Attachment` JSON. Publishes nothing. The owner is `sessionFrom(r).PeerID`.
  - `POST /api/messages` (origin+session): `{"content": string, "attachments": [id…]}` → `201` with the `Item`. Errors: `400 empty`, `400 too_many`, `400 bad_attachment`, `413 too_large`. On success it publishes `item.created`, then calls `OnFileStored` once per attachment with `store.Item{ID: att.ID, Type: KindFile, File: &FileMeta{att.Name, att.Size, att.MIME}, From: it.From, CreatedAt: it.CreatedAt}`. The app's autosave hook stays unchanged.
  - `GET /api/items/{id}/attachments/{aid}/raw[?inline=1]` (session).
- Produces (Go, used by Task 3): `func (s *server) serveFile(w http.ResponseWriter, r *http.Request, f *os.File, name, mimeType string, mod time.Time, public, inline bool)`, extracted from `serveItem`'s file branch.

- [ ] **Step 1: Write failing tests** (use the existing `login`, `do`, `multipartBody`, `openSSE` helpers):

```go
func TestStageThenMessage(t *testing.T)            // stage 2 → no SSE item.created yet; POST /api/messages → exactly one item.created carrying type "message" + 2 attachments; GET /api/items lists 1 item
func TestMessageForeignStagingRejected(t *testing.T) // client A stages, client B commits A's id → 400 bad_attachment
func TestMessageValidation(t *testing.T)           // 0 ids → 400 empty; 21 ids → 400 too_many; content 1MiB+1 → 413 too_large
func TestStageTooLargeAndOrigin(t *testing.T)      // MaxSize=4, 5-byte file → 413 too_large; missing Origin → 403 bad_origin
func TestAttachmentRaw(t *testing.T)               // png ?inline=1 → Content-Disposition "inline"; .html ?inline=1 → attachment; name `报告 "a".pdf` → filename* encoded; unknown aid → 404
func TestMessageAutosaveHook(t *testing.T)         // Deps.OnFileStored records calls: 2 calls, each Item.ID == attachment ID and file exists at Store.Dir()/ID
```

- [ ] **Step 2: Run** `go test ./internal/server/ -run 'Stage|Message|AttachmentRaw'`. Expected: FAIL (404 / undefined).

- [ ] **Step 3: Implement.** Extract the shared prologue of `handleUpload` (Content-Length limit, disk-free check, `MaxBytesReader`, finding the `file` part) into `func (s *server) filePart(w http.ResponseWriter, r *http.Request) (*multipart.Part, bool)` and use it in both upload handlers. Register the three routes in `server.go`.

- [ ] **Step 4: Run** `go test ./internal/server/`. Expected: PASS, including the existing upload and raw tests.

- [ ] **Step 5: Commit** `feat(server): staged attachments and message items`

---

### Task 3: Share links for messages (expiry only)

**Files:**
- Modify: `internal/share/share.go`, `internal/server/share_handlers.go`, `internal/server/server.go`
- Test: `internal/share/share_test.go`, `internal/server/share_handlers_test.go`

**Interfaces:**
- Consumes: Task 2 `serveFile`; Task 1 `OpenAttachment`.
- Produces:
  - `const Unlimited = -1`; `Manager.Create` takes `maxDownloads == Unlimited` and keeps it as `Remaining = -1`. `live()` treats `Remaining < 0` as having no count limit, and `Consume` does not decrement it.
  - `POST /api/items/{id}/share` on a `message` item ignores `max_downloads` and creates an `Unlimited` link.
  - `GET /api/s/{id}` for a message → `{"type":"message","content":…,"attachments":[…],"remaining":-1,"expires_at":…}`. For text and file the response is unchanged.
  - `GET /s/{id}/attachments/{aid}/raw[?inline=1]` (public): `Peek` (never consumes). Inline is allowed only for whitelisted MIME types; otherwise it is served as `attachment`. Gone link, gone item, or unknown aid → `410 gone`.
  - `GET /s/{id}/raw` on a message → `410 gone`.

- [ ] **Step 1: Write failing tests**

```go
func TestUnlimitedLink(t *testing.T)               // share pkg: Create(id, Unlimited, ttl); 50× Consume ok, Remaining stays -1; after ttl → ErrGone; Sweep keeps it until expiry
func TestShareMessage(t *testing.T)                // server: share a message with max_downloads=1; GET attachment raw 3× → 200 each; meta has content + 2 attachments + remaining -1
func TestShareMessageGoneAfterDelete(t *testing.T) // delete the message → attachment raw 410, meta 410
func TestShareMessageExpired(t *testing.T)         // advance Deps.Now past ttl → 410
func TestShareFileStillCounts(t *testing.T)        // regression: file link max 1 → second /s/{id}/raw is 410
```

- [ ] **Step 2: Run** `go test ./internal/share/ ./internal/server/ -run 'Unlimited|ShareMessage|ShareFileStillCounts'`. Expected: FAIL.

- [ ] **Step 3: Implement** the changes listed under Produces.

- [ ] **Step 4: Run** `go test ./...`. Expected: PASS.

- [ ] **Step 5: Commit** `feat(share): expiry-only share links for messages`

---

### Task 4: Web — API client and draft model

**Files:**
- Modify: `web/src/api.ts`
- Create: `web/src/lib/draft.ts`, `web/src/lib/draft.test.ts`
- Modify: `web/src/i18n/{en,zh,ja,ko}.ts`

**Interfaces:**
- Consumes: the HTTP endpoints from Tasks 2 and 3.
- Produces (`api.ts`):
  - `Item.type: 'text' | 'file' | 'message'`; `Item.attachments?: Attachment[]`; `export interface Attachment { id: string; name: string; size: number; mime: string }`
  - `ShareMeta` gains `content?: string; attachments?: Attachment[]`, and `type` becomes `'text' | 'file' | 'message'`
  - `upload(file, onProgress, url = '/api/upload')`: the generalized version of the existing function
  - `api.sendMessage(content: string, ids: string[]): Promise<Item>`
  - `attachmentUrl(itemId: string, attId: string, inline?: boolean): string`
  - `shareAttachmentUrl(shareId: string, attId: string, inline?: boolean): string`
- Produces (`lib/draft.ts`; `draft` moves here from `Composer.tsx`):
  - `export const draft: Signal<string>`, `export const pending: Signal<PendingFile[]>`, `export const sending: Signal<boolean>`, `export const canSend: ReadonlySignal<boolean>`
  - `export type PendingFile = { key: string; file: File; url?: string; isImage: boolean; state: 'idle' | 'uploading' | 'done' | 'error'; loaded: number; error?: string; stagedId?: string }`
  - `export const MAX_ATTACHMENTS = 20`
  - `export function pasteName(file: File, now: Date, n: number, prefix: string): string`
  - `export function addFiles(files: File[], opts?: { pasted?: boolean }): void`
  - `export function removeFile(key: string): void`
  - `export function sendDraft(): Promise<boolean>`
  - `export function cancelSend(): void`
- i18n keys: `composer.shot_prefix`, `composer.remove` ("Remove {name}"), `composer.too_many` ("Up to 20 attachments per message"), `composer.file_too_large`, `composer.busy` ("Sending, please wait…"), `composer.cancel`, `composer.retry`.

- [ ] **Step 1: Write failing tests** in `draft.test.ts` (mock `../api` the same way `uploads.test.ts` does; stub `URL.createObjectURL` and `revokeObjectURL`):

```ts
test('pasteName formats and dedups')         // pasteName(new File([], 'image.png', {type:'image/png'}), new Date(2026,10,10,9,5,7), 0, 'screenshot') === 'screenshot-20261110-090507.png'; n=2 → '…-090507-2.png'; ext from mime for 'image/jpeg' → '.jpg'
test('addFiles renames only pasted generic names') // pasted 'image.png' renamed; pasted 'diagram.png' kept; picked 'image.png' kept; two pasted in same second → distinct names
test('addFiles caps at 20 and toasts')        // add 25 → pending.length 20, toast 'composer.too_many'
test('oversize file marked error and blocks send') // me.max_size=10, 11-byte file → state 'error', error 'too_large'; canSend false until removeFile
test('addFiles while sending is rejected')    // sending=true → pending unchanged, toast 'composer.busy'
test('removeFile revokes object url')
test('canSend')                               // blank text + 0 files false; text only true; 1 file true
test('send picks type')                       // text only → api.sendText; 1 file + blank → upload(url '/api/upload'); 1 file + text → upload('/api/attachments') then sendMessage('t',[id]); 2 files → 2 stage uploads then sendMessage
test('success clears draft and revokes urls')
test('stage failure keeps draft, no sendMessage')    // one of two uploads rejects → sendDraft false, draft text intact, failed file state 'error', the other keeps stagedId; next sendDraft only re-uploads the failed one
test('bad_attachment drops staged ids')       // sendMessage rejects ApiError(400,'bad_attachment') → every pending stagedId cleared; next sendDraft re-uploads all
test('cancelSend aborts uploads and keeps draft')
```

- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/draft.test.ts`. Expected: FAIL (module not found).

- [ ] **Step 3: Implement** `api.ts` changes and `lib/draft.ts`. Staging uploads run at most 3 at a time. `isImage` uses `isImageMime`. The oversize check reads `me.value?.max_size`, as in `uploads.ts`. Add the i18n keys to all four dicts.

- [ ] **Step 4: Run** `cd web && npm test`. Expected: PASS, including i18n parity.

- [ ] **Step 5: Commit** `feat(web): draft model with staged multi-file send`

---

### Task 5: Web — image viewer overlay

**Files:**
- Create: `web/src/components/ImageViewer.tsx`, `web/src/components/ImageViewer.test.tsx`
- Modify: `web/src/styles.css`, `web/src/i18n/{en,zh,ja,ko}.ts`

**Interfaces:**
- Produces: `export function ImageViewer(props: { images: { src: string; download: string; name: string }[]; start: number; onClose: () => void })`. `src` is the inline URL and `download` is the attachment URL.
- i18n keys: `viewer.prev`, `viewer.next`, `viewer.close`, `viewer.download`, `viewer.count` ("{i} / {n}").

- [ ] **Step 1: Write failing tests**

```tsx
test('opens at start, arrows and keys navigate, wraps none') // 3 images start 1 → shows "2 / 3"; ArrowRight → "3 / 3"; ArrowRight again stays "3 / 3"; ArrowLeft → "2 / 3"
test('single image hides prev/next and count')
test('Escape and backdrop click close')
test('registers as overlay')                                  // openOverlay increments while mounted, restores on unmount
test('download link points at the current image')
```

- [ ] **Step 2: Run** `cd web && npx vitest run src/components/ImageViewer.test.tsx`. Expected: FAIL.

- [ ] **Step 3: Implement.** Use `useOverlay()`. Move focus inside on open and restore it on close, the same way `Dialog` does. Use `role="dialog"` and `aria-modal`. A touch swipe with horizontal delta > 50px navigates. The image is `object-fit: contain` on a dark translucent backdrop.

- [ ] **Step 4: Run** `cd web && npm test`. Expected: PASS.

- [ ] **Step 5: Commit** `feat(web): in-page image viewer`

---

### Task 6: Web — composer tray and entry points

**Files:**
- Modify: `web/src/components/Composer.tsx`, `web/src/components/MainPage.tsx`, `web/src/components/Feed.tsx`, `web/src/styles.css`, `web/src/i18n/{en,zh,ja,ko}.ts`
- Delete: `web/src/lib/uploads.ts`, `web/src/lib/uploads.test.ts`, `web/src/components/UploadRow.tsx` (`enqueue` has no callers left; progress now shows in the tray)
- Create: `web/src/components/Composer.test.tsx`

**Interfaces:**
- Consumes: Task 4 `draft`, `pending`, `sending`, `canSend`, `addFiles`, `removeFile`, `sendDraft`, `cancelSend`.
- Behaviour:
  - Textarea `onPaste`: if `clipboardData.files.length` is > 0 → `preventDefault`, then `addFiles(files, { pasted: true })`. Otherwise do nothing, so the browser default applies.
  - The global paste handler in `MainPage` (outside editable elements, no overlay): files → `addFiles(files, { pasted: true })` + focus the textarea. Text → if `draft` is blank and `pending` is empty, keep today's `sendText`/too-large path; otherwise append the text to `draft` and focus.
  - Drop and 📎 pick → `addFiles(files)`.
  - "Send as file" → `addFiles([new File([draft], 'text.txt', {type:'text/plain'})])`, then `draft.value = ''`.
  - The tray sits above the textarea and is not rendered when empty. Each image renders as a 64×64 `object-fit: cover` thumb from `url`. Each non-image renders as a chip with name and `formatSize`. Each item has a ✕ button with `aria-label` `composer.remove` and shows a percent while uploading and a red error state with `errText` on failure. While `sending`, the ✕ buttons are disabled, Send becomes Cancel, and the textarea is read-only.
  - The Send button and ⌘↵/Ctrl+↵ are gated by `canSend`.
  - The placeholder text changes to "Paste text, screenshots or drop files…" in all 4 languages.

- [ ] **Step 1: Write failing tests** in `Composer.test.tsx`:

```tsx
test('pasting an image into the textarea adds it to the tray and inserts no text') // fire paste with files=[png] and text/plain 'x' → 1 thumb, textarea value unchanged, defaultPrevented
test('pasting plain text into the textarea is not intercepted')                    // no files → not defaultPrevented
test('✕ removes the attachment')
test('send button enabled by attachment alone')
test('tray hidden when empty')
```

and in `MainPage` (extend an existing test or add `MainPage.test.tsx`):

```tsx
test('outside paste of files goes to tray and focuses textarea')
test('outside paste of text sends when draft empty, appends when draft has attachments')
test('drop adds to tray instead of uploading')
```

- [ ] **Step 2: Run** `cd web && npx vitest run src/components/Composer.test.tsx`. Expected: FAIL.

- [ ] **Step 3: Implement** the behaviour above. Delete the upload queue files and remove the `UploadRow` rendering from `Feed`.

- [ ] **Step 4: Run** `cd web && npm test && npx tsc --noEmit`. Expected: PASS, no type errors.

- [ ] **Step 5: Commit** `feat(web): pending attachment tray for paste, pick and drop`

---

### Task 7: Web — message card, single-image viewer, sharing UI

**Files:**
- Create: `web/src/components/MessageBody.tsx`, `web/src/components/MessageBody.test.tsx`
- Modify: `web/src/components/ItemCard.tsx`, `web/src/components/FileBody.tsx`, `web/src/components/ShareDialog.tsx`, `web/src/components/SharePage.tsx`, `web/src/components/SharePage.test.tsx`, `web/src/styles.css`, `web/src/i18n/{en,zh,ja,ko}.ts`

**Interfaces:**
- Consumes: Task 4 `attachmentUrl`, `shareAttachmentUrl`, `Attachment`; Task 5 `ImageViewer`.
- Produces: `export function MessageBody(props: { content: string; attachments: Attachment[]; src: (a: Attachment, inline: boolean) => string; actions?: ComponentChildren })`, shared by `ItemCard` (with `attachmentUrl`) and `SharePage` (with `shareAttachmentUrl`).
- Layout: text first (reuse `TextBody` when the content is non-blank). Then the images in send order: one image renders large (aspect kept, `max-height: 320px`); 2 or more render in a square grid with up to 3 per row, each with a ⬇ link (`aria-label` `item.download_name` "Download {name}"). Then the non-image attachments, one row each with a download link. Clicking an image opens `ImageViewer` over that message's images.
- `ItemCard` for `message`: actions are Copy text (only when the content is non-blank), Share, and Delete.
- `FileBody` for an image: click opens `ImageViewer` with one image instead of `target=_blank`.
- `ShareDialog` for `message`: hide the "Allowed downloads" field and the `share.remaining` hint.
- `SharePage` for `message`: show the expiry hint (no remaining count, no "Show text" button), then `MessageBody`.
- i18n keys: `item.download_name`, `share.message_title` (e.g. "Shared message").

- [ ] **Step 1: Write failing tests**

```tsx
// MessageBody.test.tsx
test('renders text, image grid in order, and file rows')  // 2 png + 1 pdf: 2 <img>, 1 file row named 'a.pdf', text present
test('each image has its own download link')               // hrefs === src(att,false) per image
test('clicking an image opens the viewer at that index')   // click 2nd → "2 / 2"
test('no text block when content blank')
// SharePage.test.tsx
test('message share shows content and attachments without consuming') // meta type 'message' → no 'Show text' button, attachment links use /s/{id}/attachments/
```

- [ ] **Step 2: Run** `cd web && npx vitest run src/components/MessageBody.test.tsx src/components/SharePage.test.tsx`. Expected: FAIL.

- [ ] **Step 3: Implement** the components and changes above.

- [ ] **Step 4: Run** `cd web && npm test && npx tsc --noEmit`. Expected: PASS.

- [ ] **Step 5: Commit** `feat(web): multi-attachment message cards and sharing`

---

### Task 8: End-to-end and build

**Files:**
- Modify: `web/e2e/smoke.spec.ts`

- [ ] **Step 1: Update the existing e2e tests** that relied on instant upload. In "2MB upload…" and "share link works once…", after `setInputFiles`, click `Send` (exact). Single file + no text stays a `file` item, so the rest of each test is unchanged.

- [ ] **Step 2: Add e2e tests**

```ts
test('pasted screenshots + text arrive as one message', ...)
// device A: focus Message; dispatch a paste on the textarea with a DataTransfer holding two 1×1 PNGs (build via page.evaluate + new File); type 'look'; remove the 2nd thumb via its ✕; Send.
// device B: exactly one new item card containing 'look' and exactly 1 <img>; clicking it opens a dialog showing the image; Escape closes it; its ⬇ link download has suggestedFilename starting 'screenshot-'.
test('message share link is reusable within expiry', ...)
// A sends 2 files + text; Share → no 'Allowed downloads' field; guest opens link twice, downloads an attachment twice → both succeed.
```

- [ ] **Step 3: Build and run.** Run `make web && make build`, then start `bin/landrop --no-open --port 18848 > web/test-results/server.log 2>&1 &`, then `cd web && npm run e2e`. Expected: all tests PASS. Stop the server afterwards.

- [ ] **Step 4: Run the full suite** with `make test`. Expected: PASS.

- [ ] **Step 5: Commit** `test(e2e): paste attachments and message sharing`
