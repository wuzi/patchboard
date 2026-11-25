# PatchBoard v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build PatchBoard v1: a self-hosted community feedback board (posts, votes, comments, roadmap, updates, moderation) as one Go binary with an embedded Angular front-end and SQLite storage.

**Architecture:** A Go `net/http` server exposes a JSON API under `/api`, serves uploaded images under `/uploads`, and serves the embedded Angular build for every other path (injecting link-preview meta tags). Domain packages (`auth`, `board`, `posts`, `comments`, `uploads`, `updates`) own their SQL (sqlc for CRUD; hand-written SQL for list, search and other dynamic queries) and their HTTP handlers. `internal/server` wires them together. The Angular app is a zoneless, standalone-component SPA with signal stores and a typed API client.

**Tech Stack:** Go 1.27, modernc.org/sqlite v1.63.0 (FTS5), sqlc v1.31.1, goldmark v1.8.7, bluemonday v1.0.27, golang.org/x/{crypto v0.59.0, image v0.48.0, term v0.48.0, text v0.44.0}; Angular 22.3.0 + @angular/cdk 22.3.0, Vitest (Angular default), @fontsource/poppins 5.3.0; Playwright 1.65.1; Docker.

**Spec:** `docs/spec.md`

## Global Constraints

- Work on branch `feat/v1`, never on `main`. Commit subjects are lowercase and short (keep proper nouns, acronyms and identifiers as written); **no co-author trailer**.
- Go module path `patchboard`; `go.mod` says `go 1.27`. Builds use `CGO_ENABLED=0`. The only Go dependencies are those listed in Tech Stack (sqlc as a `go tool` dependency). No web framework, ORM or test library: assertions use `internal/testutil` helpers.
- Front-end: Angular 22.3.0, standalone components, zoneless, signals. The only extra npm dependencies are `@angular/cdk` and `@fontsource/poppins` (plus `@playwright/test` in `e2e/`). No UI component library. Modals use the native `<dialog>` element with `showModal()`.
- Timestamps are stored as `INTEGER` unix seconds (UTC) and serialised in JSON as RFC 3339 UTC strings. All "now" values come from `clock.Clock`, never `time.Now()` directly.
- JSON fields are camelCase. Every error response has the shape `{"error":{"code":"<code>","message":"<english text>","fields":{"<field>":"<reason>"}}}` (`fields` only on 422).
- Every non-GET/HEAD/OPTIONS request to `/api/*` must carry the header `X-PB-CSRF: 1`, or it gets 403 `csrf`.
- Cookies: `pb_guest` (Max-Age 34560000 = 400 days) and `pb_admin` (Max-Age 2592000 = 30 days); `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` exactly when `PB_BASE_URL` starts with `https://`.
- Raw IPs are never stored or logged. The IP hash is `hex(HMAC-SHA256(secret, ip))[:32]`, with IPv6 masked to /64 and IPv4-mapped IPv6 unmapped first.
- The values in the spec's "Limits and defaults" table are binding; they are repeated in the Shared Contracts below.
- Every user-visible UI string comes from the i18n dictionaries `web/src/app/core/i18n/pt.ts` and `en.ts`. `en.ts` is typed `Record<keyof typeof pt, string>`, so a missing key fails the build.

## Review Focus

1. **Two players vote on the same post at the same moment.** Both votes must count, with no "database is locked" error. Test: Task B8 `TestVote_Concurrent`.
2. **Accented or emoji titles** ("Ação rápida 🎮", "Remoção da \"Boots of Haste\"") must produce readable ASCII slugs, and search must ignore accents. Tests: Task B7 `TestSlugify`, Task B10 `TestSearch_AccentInsensitive`.
3. **Hidden posts leaking.** Pending, deleted and merged posts must not appear in the list, roadmap, search, similar-post hints, Updates or link previews. Tests: B9 `TestList_ExcludesHidden`, B10 `TestSimilar_ExcludesHidden`, B13 `TestUpdates_HideDeletedPosts`, B16 `TestMeta_HiddenPostUsesBoardDefaults`.
4. **A hostile or huge image** (for example a 20000×20000 PNG that is only a few KB on disk) must be rejected before it is decoded, without exhausting memory. Test: B12 `TestProcess_RejectsTooManyPixels`.
5. **An admin who has never had a guest cookie votes or likes.** It must work and quietly give them a guest identity. A merged post reached through an old link or a merge chain must land on the final post. Tests: B8 `TestVote_AdminWithoutGuestCookie`, B14 `TestMerge_Chain`.

---

## File Structure

```
cmd/patchboard/
  main.go            run(args, stdin, stdout, stderr, getenv) int — dispatch: serve | admin create | admin reset-password
  serve.go           serve(): config → store → server → http.Server, graceful shutdown, hourly orphan cleanup
  admin.go           admin subcommands
  main_test.go
internal/config/     Config, Load(getenv) — env vars and the secret file
internal/clock/      Clock interface, Real, Fake
internal/store/      Open, Migrate, InTx; migrations/*.sql; queries/*.sql; gen/ (sqlc output, committed)
internal/api/        types.go — JSON response/request types shared by handlers and tests (the contract below)
internal/httpx/      JSON helpers, Error, HandlerFunc, CSRF, RequestInfo (client IP + IP hash), Limiter + rules
internal/markdown/   Render, Excerpt, PreviewHandler
internal/auth/       guests, Actor middleware, bans, admin accounts, sessions, invites, team, passwords
internal/board/      settings, statuses, tags (public read + admin CRUD)
internal/posts/      create/get/edit/delete, slug, votes, list/roadmap, search/similar, status change, moderation, merge, vote audit
internal/comments/   comments, likes, restore
internal/uploads/    image processing, storage, serving, attachment, cleanup
internal/updates/    Updates CRUD and public list
internal/web/        embedded SPA (dist/), meta-tag injection
internal/server/     server.New(Deps) — route table and wiring; meta.go (MetaSource)
internal/testutil/   App/Client test harness
web/                 Angular workspace (ng new), builds into internal/web/dist
e2e/                 Playwright project
sqlc.yaml  Makefile  Dockerfile  .dockerignore  README.md
```

## Shared Contracts

### Database schema (`internal/store/migrations/0001_init.sql`, owned by Task B1)

```sql
CREATE TABLE board_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  title_pt TEXT NOT NULL, title_en TEXT NOT NULL,
  description_pt TEXT NOT NULL DEFAULT '', description_en TEXT NOT NULL DEFAULT '',
  team_name TEXT NOT NULL,
  accent_color TEXT NOT NULL,
  logo_upload_id TEXT REFERENCES uploads(id) ON DELETE SET NULL,
  require_approval INTEGER NOT NULL DEFAULT 0,
  uploads_enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE statuses (
  id INTEGER PRIMARY KEY,
  name_pt TEXT NOT NULL, name_en TEXT NOT NULL,
  color TEXT NOT NULL, position INTEGER NOT NULL,
  show_on_roadmap INTEGER NOT NULL DEFAULT 1,
  is_default INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX statuses_one_default ON statuses(is_default) WHERE is_default = 1;
CREATE TABLE tags (id INTEGER PRIMARY KEY, name_pt TEXT NOT NULL, name_en TEXT NOT NULL, position INTEGER NOT NULL);
CREATE TABLE guests (
  id TEXT PRIMARY KEY,                 -- public id: 16 chars lowercase base32
  token_hash BLOB NOT NULL UNIQUE,     -- sha256(cookie token)
  name TEXT,
  created_at INTEGER NOT NULL,
  created_ip_hash TEXT NOT NULL,
  last_ip_hash TEXT NOT NULL
);
CREATE TABLE admins (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  display_name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  disabled_at INTEGER
);
CREATE TABLE admin_sessions (
  token_hash BLOB PRIMARY KEY, admin_id INTEGER NOT NULL REFERENCES admins(id),
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL
);
CREATE TABLE admin_invites (
  token_hash BLOB PRIMARY KEY, created_by INTEGER NOT NULL REFERENCES admins(id),
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL,
  used_at INTEGER, used_by INTEGER REFERENCES admins(id)
);
CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL, title TEXT NOT NULL,
  body_md TEXT NOT NULL DEFAULT '', body_html TEXT NOT NULL DEFAULT '', excerpt TEXT NOT NULL DEFAULT '',
  status_id INTEGER NOT NULL REFERENCES statuses(id),
  tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL,
  author_guest_id TEXT REFERENCES guests(id),
  author_admin_id INTEGER REFERENCES admins(id),
  ip_hash TEXT NOT NULL,
  vote_count INTEGER NOT NULL DEFAULT 0,
  comment_count INTEGER NOT NULL DEFAULT 0,
  pending INTEGER NOT NULL DEFAULT 0,
  pinned_at INTEGER,
  comments_locked INTEGER NOT NULL DEFAULT 0,
  merged_into_id INTEGER REFERENCES posts(id),
  deleted_at INTEGER, deleted_by_admin_id INTEGER REFERENCES admins(id),
  response_md TEXT, response_html TEXT, response_admin_id INTEGER REFERENCES admins(id), response_at INTEGER,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  CHECK ((author_guest_id IS NULL) <> (author_admin_id IS NULL))
);
CREATE INDEX posts_visible ON posts(status_id, vote_count) WHERE deleted_at IS NULL AND merged_into_id IS NULL AND pending = 0;
CREATE TABLE votes (
  post_id INTEGER NOT NULL REFERENCES posts(id), guest_id TEXT NOT NULL REFERENCES guests(id),
  ip_hash TEXT NOT NULL, created_at INTEGER NOT NULL,
  PRIMARY KEY (post_id, guest_id)
);
CREATE INDEX votes_recent ON votes(post_id, created_at);
CREATE TABLE comments (
  id INTEGER PRIMARY KEY,
  post_id INTEGER NOT NULL REFERENCES posts(id),
  parent_id INTEGER REFERENCES comments(id),
  author_guest_id TEXT REFERENCES guests(id), author_admin_id INTEGER REFERENCES admins(id),
  body_md TEXT NOT NULL, body_html TEXT NOT NULL,
  ip_hash TEXT NOT NULL,
  like_count INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  deleted_at INTEGER, deleted_by_admin_id INTEGER REFERENCES admins(id),
  CHECK ((author_guest_id IS NULL) <> (author_admin_id IS NULL))
);
CREATE INDEX comments_post ON comments(post_id, created_at);
CREATE TABLE comment_likes (
  comment_id INTEGER NOT NULL REFERENCES comments(id), guest_id TEXT NOT NULL REFERENCES guests(id),
  created_at INTEGER NOT NULL, PRIMARY KEY (comment_id, guest_id)
);
CREATE TABLE uploads (
  id TEXT PRIMARY KEY,                 -- 20 chars lowercase base32
  ext TEXT NOT NULL CHECK (ext IN ('jpg','png')),
  width INTEGER NOT NULL, height INTEGER NOT NULL, bytes INTEGER NOT NULL,
  post_id INTEGER REFERENCES posts(id), position INTEGER NOT NULL DEFAULT 0,
  guest_id TEXT REFERENCES guests(id), admin_id INTEGER REFERENCES admins(id),
  ip_hash TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE updates (
  id INTEGER PRIMARY KEY, title TEXT NOT NULL, body_md TEXT NOT NULL, body_html TEXT NOT NULL,
  author_admin_id INTEGER NOT NULL REFERENCES admins(id),
  published_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE update_posts (
  update_id INTEGER NOT NULL REFERENCES updates(id) ON DELETE CASCADE,
  post_id INTEGER NOT NULL REFERENCES posts(id),
  status_id INTEGER NOT NULL REFERENCES statuses(id),   -- snapshot at link time
  position INTEGER NOT NULL,
  PRIMARY KEY (update_id, post_id)
);
CREATE TABLE bans (
  id INTEGER PRIMARY KEY, guest_id TEXT REFERENCES guests(id), ip_hash TEXT,
  reason TEXT NOT NULL DEFAULT '', created_by INTEGER NOT NULL REFERENCES admins(id), created_at INTEGER NOT NULL,
  CHECK (guest_id IS NOT NULL OR ip_hash IS NOT NULL)
);
CREATE INDEX bans_guest ON bans(guest_id);
CREATE INDEX bans_ip ON bans(ip_hash);
CREATE VIRTUAL TABLE posts_fts USING fts5(title, body_md, content='posts', content_rowid='id',
  tokenize='unicode61 remove_diacritics 2');
CREATE TRIGGER posts_fts_ai AFTER INSERT ON posts BEGIN
  INSERT INTO posts_fts(rowid, title, body_md) VALUES (new.id, new.title, new.body_md); END;
CREATE TRIGGER posts_fts_ad AFTER DELETE ON posts BEGIN
  INSERT INTO posts_fts(posts_fts, rowid, title, body_md) VALUES ('delete', old.id, old.title, old.body_md); END;
CREATE TRIGGER posts_fts_au AFTER UPDATE OF title, body_md ON posts BEGIN
  INSERT INTO posts_fts(posts_fts, rowid, title, body_md) VALUES ('delete', old.id, old.title, old.body_md);
  INSERT INTO posts_fts(rowid, title, body_md) VALUES (new.id, new.title, new.body_md); END;
```

`0002_seed.sql` inserts `board_settings(1, 'PatchBoard', 'PatchBoard', '', '', 'Equipe', '#c026d3', NULL, 0, 1)`, the six statuses from the spec table (ids 1–6, positions 0–5, all `show_on_roadmap = 1`, id 1 default), and the tags `(1,'Feature','Feature',0)`, `(2,'Melhoria','Improvement',1)`, `(3,'Bug','Bug',2)`.

A post is **publicly visible** when `deleted_at IS NULL AND merged_into_id IS NULL AND pending = 0`. Every public read (list, roadmap, search, similar, Updates links, meta tags) uses this predicate. Anything that writes to a post or comment must update `vote_count` / `comment_count` / `like_count` by recounting inside the same transaction (`UPDATE posts SET vote_count = (SELECT count(*) FROM votes WHERE post_id = ?)`).

### JSON types (`internal/api/types.go` ⇄ `web/src/app/core/api.types.ts`, owned by B1 / F1)

Go structs use the same names, with `json:"camelCase"` tags; optional values are pointers. Times are strings in TS and `time.Time` in Go.

```ts
type I18nName = { pt: string; en: string };
type Status = { id: number; name: I18nName; color: string; position: number; showOnRoadmap: boolean; isDefault: boolean };
type Tag = { id: number; name: I18nName; position: number };
type Board = { title: I18nName; description: I18nName; teamName: string; accentColor: string; logoUrl: string | null;
               requireApproval: boolean; uploadsEnabled: boolean; statuses: Status[]; tags: Tag[] };
type Author = { kind: 'guest' | 'admin'; id: string; name: string | null };  // admin id is "admin-<n>"; null name → "Anônimo"
type Me = { guest: { id: string; name: string | null; recoveryUrl: string } | null;
            admin: { id: number; username: string; displayName: string } | null };
type PostCard = { id: number; slug: string; title: string; excerpt: string; statusId: number; tagId: number | null;
                  voteCount: number; newVotes: number; commentCount: number; voted: boolean; voters: Author[];
                  pinned: boolean; pending: boolean; createdAt: string };
type Image = { id: string; url: string; width: number; height: number };
type TeamResponse = { html: string; md: string; author: Author; at: string };
type PostDetail = PostCard & { bodyHtml: string; bodyMd: string; author: Author; images: Image[];
                  response: TeamResponse | null; commentsLocked: boolean; deleted: boolean;
                  canEdit: boolean; canDelete: boolean; comments: Comment[] };
type PostRef = { id: number; slug: string };
type PostResponse = { post: PostDetail | null; mergedInto: PostRef | null };
type Comment = { id: number; parentId: number | null; author: Author | null; bodyHtml: string | null; bodyMd: string | null;
                 likeCount: number; liked: boolean; deleted: boolean; canEdit: boolean; createdAt: string; replies: Comment[] };
type PostPage = { posts: PostCard[]; nextOffset: number | null };
type Roadmap = { columns: { statusId: number; total: number; posts: PostCard[] }[] };
type Similar = { posts: { id: number; slug: string; title: string; voteCount: number; statusId: number }[] };
type UpdateItem = { id: number; title: string; bodyHtml: string; bodyMd: string; author: Author; publishedAt: string;
                    posts: { id: number; slug: string; title: string; statusId: number }[] };
type UpdatePage = { updates: UpdateItem[]; nextOffset: number | null };
type VoteState = { voteCount: number; voted: boolean };
type LikeState = { likeCount: number; liked: boolean };
type TeamMember = { id: number; username: string; displayName: string; createdAt: string; disabled: boolean };
type Invite = { url: string; expiresAt: string };
type VoteAudit = { voters: { guest: { id: string; name: string | null; createdAt: string }; ipHash: string; votedAt: string;
                             flags: ('shared_ip' | 'new_guest')[] }[] };
type Ban = { id: number; guestId: string | null; guestName: string | null; ipHash: string | null; reason: string; createdAt: string };
type TrashPost = { id: number; slug: string; title: string; deletedAt: string; deletedBy: 'author' | 'admin' };
type TrashComment = { id: number; postId: number; postTitle: string; excerpt: string; deletedAt: string; deletedBy: 'author' | 'admin' };
type Trash = { posts: TrashPost[]; comments: TrashComment[] };
```

`bodyMd` and `TeamResponse.md` are `""` unless the viewer is the author or an admin. `ipHash` in `VoteAudit` is the first 8 characters only.

### Endpoints

"Guest write" means the handler calls `auth.Service.Writer` (an admin acts as admin; anyone else gets a guest identity, created if needed, and is checked against bans). "Guest only" means `auth.Service.GuestWriter` (always a guest identity, admins included). Paths under `/api/admin/` require an admin session (401 `unauthorized`) except where marked *public*.

| Method + path | Who | Request → Response | Task | TS method |
|---|---|---|---|---|
| GET `/healthz` | any | → `ok` (text) | B1 | – |
| POST `/api/markdown/preview` | any | `{markdown}` → `{html}` | B3 | `preview` |
| GET `/api/me` | any | → `Me` | B4 | `me` |
| PUT `/api/me/name` | guest only | `{name}` → `Me` | B4 | `setName` |
| GET `/r/{token}` | any | → 302 `/?recovered=1` or `/?recovered=0` | B4 | – |
| POST `/api/admin/login` *public* | any | `{username,password}` → `Me` | B5 | `adminLogin` |
| POST `/api/admin/logout` | admin | → 204 | B5 | `adminLogout` |
| GET `/api/admin/invites/{token}` *public* | any | → `{valid}` | B5 | `checkInvite` |
| POST `/api/admin/invites/{token}/accept` *public* | any | `{username,displayName,password}` → 201 `Me` | B5 | `acceptInvite` |
| POST `/api/admin/invites` | admin | → 201 `Invite` | B5 | `createInvite` |
| GET `/api/admin/team` | admin | → `TeamMember[]` | B5 | `team` |
| POST `/api/admin/team/{id}/disable` | admin | → 204 | B5 | `disableMember` |
| PATCH `/api/admin/me` | admin | `{displayName?, currentPassword?, newPassword?}` → `Me` | B5 | `updateAdminMe` |
| GET `/api/board` | any | → `Board` | B6 | `board` |
| PUT `/api/admin/settings` | admin | `{title,description,teamName,accentColor,logoUploadId,requireApproval,uploadsEnabled}` → `Board` | B6 | `saveSettings` |
| POST `/api/admin/statuses` | admin | `{name,color,showOnRoadmap}` → 201 `Status` | B6 | `createStatus` |
| PATCH `/api/admin/statuses/{id}` | admin | `{name?,color?,showOnRoadmap?,isDefault?}` → `Status` | B6 | `updateStatus` |
| PUT `/api/admin/statuses/order` | admin | `{ids}` → `Status[]` | B6 | `orderStatuses` |
| DELETE `/api/admin/statuses/{id}?moveTo={id}` | admin | → 204 | B6 | `deleteStatus` |
| POST / PATCH `/{id}` / DELETE `/{id}` `/api/admin/tags`, PUT `/api/admin/tags/order` | admin | `{name}` / `{ids}` → `Tag` / `Tag[]` | B6 | `createTag` `updateTag` `deleteTag` `orderTags` |
| POST `/api/posts` | guest write | `{title, body, tagId?, imageIds?}` → 201 `PostDetail` | B7 (+B12) | `createPost` |
| GET `/api/posts/{id}` | any | → `PostResponse` | B7 | `post` |
| PATCH `/api/posts/{id}` | author in window, admin | `{title?, body?, tagId?, clearTag?, imageIds?}` → `PostDetail` | B7 | `updatePost` |
| DELETE `/api/posts/{id}` | author in window, admin | → 204 | B7 | `deletePost` |
| POST / DELETE `/api/posts/{id}/vote` | guest only | → `VoteState` | B8 | `vote` `unvote` |
| GET `/api/posts?sort=&status=&tag=&q=&offset=&limit=` | any | → `PostPage` | B9 (+B10) | `listPosts` |
| GET `/api/roadmap?tag=&q=` | any | → `Roadmap` | B9 (+B10) | `roadmap` |
| GET `/api/posts/similar?title=` | any | → `Similar` | B10 | `similar` |
| POST `/api/posts/{id}/comments` | guest write | `{body, parentId?}` → 201 `Comment` | B11 | `comment` |
| PATCH / DELETE `/api/comments/{id}` | author, admin | `{body}` → `Comment` / 204 | B11 | `editComment` `deleteComment` |
| POST / DELETE `/api/comments/{id}/like` | guest only | → `LikeState` | B11 | `like` `unlike` |
| POST `/api/uploads` (multipart `file`) | guest write | → 201 `Image` | B12 | `upload` |
| GET `/uploads/{id}.{ext}` | any | → image bytes | B12 | – |
| DELETE `/api/admin/uploads/{id}` | admin | → 204 | B12 | `deleteUpload` |
| POST `/api/admin/posts/{id}/status` | admin | `{statusId, response?, update?: {title, body}}` → `PostDetail` | B13 | `changeStatus` |
| GET `/api/updates?offset=` | any | → `UpdatePage` | B13 | `updates` |
| POST / PATCH `/{id}` / DELETE `/{id}` `/api/admin/updates` | admin | `{title, body, postIds}` → `UpdateItem` | B13 | `createUpdate` `editUpdate` `deleteUpdate` |
| GET `/api/admin/pending` | admin | → `PostCard[]` | B14 | `pending` |
| POST `/api/admin/posts/{id}/approve` | admin | → `PostDetail` | B14 | `approve` |
| POST `/api/admin/posts/{id}/pin` / `/lock` | admin | `{pinned}` / `{locked}` → `PostDetail` | B14 | `pin` `lock` |
| POST `/api/admin/posts/{id}/restore`, `/api/admin/comments/{id}/restore` | admin | → 204 | B14 | `restorePost` `restoreComment` |
| POST `/api/admin/posts/{id}/merge` | admin | `{intoId}` → `PostDetail` (target) | B14 | `merge` |
| GET `/api/admin/trash` | admin | → `Trash` | B14 | `trash` |
| GET `/api/admin/posts/{id}/votes` | admin | → `VoteAudit` | B15 | `voteAudit` |
| DELETE `/api/admin/posts/{id}/votes/{guestId}` | admin | → `VoteState` | B15 | `removeVote` |
| POST / GET / DELETE `/{id}` `/api/admin/bans` | admin | `{guestId, includeIp, reason}` → `Ban` / `Ban[]` / 204 | B15 | `ban` `bans` `unban` |
| GET everything else | any | SPA `index.html` with meta tags, or a static asset | B16 | – |

### Error codes

`bad_request` (400, malformed JSON / unknown field), `unauthorized` (401), `invalid_credentials` (401), `forbidden` (403), `csrf` (403), `banned` (403), `locked` (403), `uploads_disabled` (403), `edit_window_closed` (403), `not_found` (404), `username_taken` (409), `invite_used` (410), `invite_expired` (410), `too_large` (413), `validation` (422, with `fields`), `invalid_order`, `cannot_delete_default`, `move_required`, `cannot_disable_self`, `invalid_merge`, `invalid_image`, `too_many_pixels` (all 422), `rate_limited` (429, with `Retry-After` seconds).

Field reasons: `required`, `too_short`, `too_long`, `invalid`, `too_many`.

### Rate rules (`internal/httpx/limits.go`)

| Name | Limit / window | Key |
|---|---|---|
| `guest.create` | 20 / 1h | IP hash |
| `post.create` | 5 / 1h | IP hash |
| `comment.create` | 20 / 1h | IP hash |
| `vote` | 120 / 1h | IP hash (vote and unvote both count) |
| `like` | 120 / 1h | IP hash |
| `upload` | 20 / 1h | IP hash |
| `preview` | 120 / 1m | IP hash |
| `admin.login.ip` | 5 / 15m | IP hash |
| `admin.login.user` | 10 / 15m | lower-cased username |

Admins are exempt from every rule except the two login rules.

---

## Backend

### Task B1: Foundation — config, clock, store, schema, server skeleton

**Files:**
- Create: `go.mod`, `sqlc.yaml`, `Makefile`, `.gitignore`, `cmd/patchboard/main.go`, `cmd/patchboard/serve.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/clock/clock.go`, `internal/store/store.go`, `internal/store/store_test.go`, `internal/store/migrations/0001_init.sql`, `internal/store/migrations/0002_seed.sql`, `internal/store/queries/board.sql`, `internal/store/gen/*` (generated), `internal/api/types.go`, `internal/httpx/json.go`, `internal/httpx/errors.go`, `internal/server/server.go`, `internal/server/server_test.go`

**Interfaces:**
- Produces:
  - `config.Config{Addr string; DataDir string; BaseURL *url.URL; TrustedProxies []netip.Prefix; Secret []byte; SecureCookies bool}`; `config.Load(getenv func(string) string) (config.Config, error)`
  - `clock.Clock` interface `{ Now() time.Time }`; `clock.Real{}`; `clock.NewFake(t time.Time) *clock.Fake` with `Set(time.Time)`, `Advance(time.Duration)`
  - `store.Open(path string) (*sql.DB, error)`; `store.Migrate(ctx context.Context, db *sql.DB) error`; `store.InTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx, q *gen.Queries) error) error`
  - all JSON types from Shared Contracts in `internal/api/types.go`
  - `httpx.WriteJSON(w http.ResponseWriter, status int, v any)`; `httpx.DecodeJSON(r *http.Request, dst any) error` (64 KiB cap, `DisallowUnknownFields`, failure → `bad_request`)
  - `httpx.Error{Status int; Code, Message string; Fields map[string]string; RetryAfter time.Duration}` implementing `error`; `httpx.E(status int, code, message string) *httpx.Error`; `(*Error).Field(name, reason string) *Error`; `httpx.Validation() *Error` (422 `validation`); `httpx.WriteError(w, err error)` (non-`*Error` → 500 `internal`, logged); `type httpx.HandlerFunc func(http.ResponseWriter, *http.Request) error` implementing `http.Handler`
  - `server.Deps{Cfg config.Config; DB *sql.DB; Clock clock.Clock; WebFS fs.FS}`; `server.New(d server.Deps) (http.Handler, error)`

- [ ] **Step 1: Scaffold the module and tooling**

Create the branch `git switch -c feat/v1`. Run `go mod init patchboard`, then `go get modernc.org/sqlite@v1.63.0` and `go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`. `sqlc.yaml` uses engine `sqlite`, schema `internal/store/migrations`, queries `internal/store/queries`, output `internal/store/gen` with package `gen`. Makefile targets: `sqlc` (`go tool sqlc generate`), `test` (`go test ./...`), `run` (`PB_BASE_URL=http://localhost:4200 PB_DATA_DIR=./data go run ./cmd/patchboard serve`; the Angular dev server on :4200 proxies to it). `.gitignore`: `/data`, `/bin`, `web/node_modules`, `web/.angular`, `e2e/node_modules`, `e2e/test-results`, `internal/web/dist/*` except `.gitkeep`.

- [ ] **Step 2: Write the failing tests**

```go
// internal/store/store_test.go
func TestMigrate_FreshDBSeedsBoard(t *testing.T) {
	db := openTemp(t) // store.Open(filepath.Join(t.TempDir(), "pb.db")) + Migrate
	// statuses ordered by position: names_pt == []string{"Sugestões","Em análise","Planejado","Em desenvolvimento","Concluído","Recusado"}
	// exactly one is_default = 1, and it is "Sugestões"; tags count == 3; board_settings accent_color == "#c026d3"
	// PRAGMA user_version == 2; PRAGMA journal_mode == "wal"; PRAGMA foreign_keys == 1
}
func TestMigrate_Idempotent(t *testing.T)            // Migrate a second time: no error, still 6 statuses
func TestSchema_FTSTracksPostChanges(t *testing.T) {
	// insert a post titled "Melhorar análise de dano" → MATCH '"analise"*' returns it
	// update its title to "Co-op" → the old match returns 0 rows, '"coop"*' returns 0, '"co"*' returns 1
	// delete the row → '"co"*' returns 0
}
// internal/config/config_test.go
func TestLoad_RequiresBaseURL(t *testing.T)           // PB_BASE_URL "" → error; "ftp://x" → error
func TestLoad_SecretPersistsInDataDir(t *testing.T)   // two Loads with the same PB_DATA_DIR and no PB_SECRET → equal 32-byte secrets; file mode 0600
func TestLoad_TrustedProxies(t *testing.T)            // "10.0.0.0/8, 172.16.0.1" → [10.0.0.0/8, 172.16.0.1/32]; "nope" → error
func TestLoad_SecureCookiesFromScheme(t *testing.T)   // https → true, http → false
// internal/server/server_test.go
func TestHealthz(t *testing.T)                        // GET /healthz → 200 "ok"
func TestUnknownAPIRouteIsJSON404(t *testing.T)       // GET /api/nope → 404, error.code == "not_found"
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./...` — Expected: FAIL (packages and functions undefined).

- [ ] **Step 4: Implement**

- `store.Open`: DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_txlock=immediate`.
- `store.Migrate`: embeds `migrations/*.sql`, sorts the files by name, and applies each file whose number is greater than `PRAGMA user_version` in its own transaction, setting `user_version` to that number.
- `config.Load`: `PB_ADDR` defaults to `:8080`. `PB_DATA_DIR` defaults to `./data` and is created with mode 0700. `PB_BASE_URL` is required (absolute http/https, trailing slash removed). `PB_TRUSTED_PROXIES` is a comma-separated list of IPs or CIDRs. `PB_SECRET`, if set, is used as raw bytes (at least 32); otherwise the secret is read from `<data>/secret` (hex), or 32 random bytes are generated and written there with mode 0600.
- `queries/board.sql`: just `-- name: GetSettings :one` for now; run `make sqlc`.
- `server.New`: builds an `http.ServeMux` with `GET /healthz` (calls `db.PingContext`) and a catch-all `/api/` → 404 `not_found`.
- `main.go`: `run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int`. `serve` (also the default when there are no args) loads config, opens the store and migrates, then starts an `http.Server` with `ReadHeaderTimeout: 10s` and shuts down gracefully on SIGINT/SIGTERM.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./...` — Expected: PASS. Also run `make run` and `curl localhost:8080/healthz` → `ok`.

- [ ] **Step 6: Commit**

`git add -A && git commit -m "add go foundation: config, sqlite store, schema, server skeleton"`

---

### Task B2: Request plumbing — client IP, IP hash, CSRF, limiter, test harness

**Files:**
- Create: `internal/httpx/clientip.go`, `internal/httpx/csrf.go`, `internal/httpx/limiter.go`, `internal/httpx/limits.go`, `internal/httpx/httpx_test.go`, `internal/testutil/testutil.go`, `internal/testutil/testutil_test.go`
- Modify: `internal/server/server.go` (install the middleware chain)

**Interfaces:**
- Consumes: `config.Config`, `clock.Clock`, `server.New` (B1)
- Produces:
  - `httpx.ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr`
  - `httpx.NewIPHasher(secret []byte) httpx.IPHasher`; `(IPHasher) Hash(a netip.Addr) string`
  - `httpx.Info{IP netip.Addr; IPHash string}`; `httpx.RequestInfo(trusted []netip.Prefix, h IPHasher) func(http.Handler) http.Handler`; `httpx.InfoFrom(ctx context.Context) httpx.Info`
  - `httpx.CSRF(next http.Handler) http.Handler`
  - `httpx.Rule{Name string; Limit int; Window time.Duration}` plus the `Rule*` vars from the Rate rules table (`RuleGuestCreate`, `RulePostCreate`, `RuleComment`, `RuleVote`, `RuleLike`, `RuleUpload`, `RulePreview`, `RuleLoginIP`, `RuleLoginUser`)
  - `httpx.NewLimiter(c clock.Clock) *httpx.Limiter`; `(*Limiter) Allow(rule Rule, key string) (ok bool, retryAfter time.Duration)`; `httpx.RateLimited(retry time.Duration) *httpx.Error`
  - `server.Deps` gains `Limiter *httpx.Limiter` (nil → `server.New` creates one)
  - testutil:
    - `testutil.New(t *testing.T, opts ...func(*config.Config)) *testutil.App`, where `App{URL string; DB *sql.DB; Clock *clock.Fake; Cfg config.Config}`. The fake clock starts at 2026-11-25T12:00:00Z, `TrustedProxies` is `127.0.0.0/8`, `BaseURL` is `http://pb.test`, the data dir is `t.TempDir()`, and `WebFS` is a MapFS whose `index.html` is `<html><head><!-- pb:meta --></head><body></body></html>`.
    - `(*App) Client(t) *Client` gives each client its own cookie jar and a unique IP `198.51.100.<n>`.
    - `(*Client) WithIP(ip string) *Client` and `(*Client) NoCSRF() *Client` return copies that share the cookie jar.
    - `(*Client) Do(method, path string, body any) *Resp` sends JSON, sets `X-PB-CSRF: 1` and `X-Forwarded-For`, and never follows redirects. `(*Client) Upload(path, field, filename string, data []byte) *Resp`. `(*Client) Cookie(name string) *http.Cookie`.
    - `Resp{Code int; Body []byte; Header http.Header}` with `Want(t, status) *Resp` (fails with the body on mismatch), `JSON(t, v any)`, `ErrCode(t) string`, `Field(t, name) string`.
    - `testutil.Eq[T comparable](t, got, want T)`; `testutil.Exec(t, db, query string, args ...any)`

- [ ] **Step 1: Write the failing tests**

```go
func TestClientIP(t *testing.T) { /* table:
  remote 203.0.113.5, no trusted, XFF "1.2.3.4"                     → 203.0.113.5   (header ignored)
  remote 127.0.0.1, trusted 127.0.0.0/8, XFF "203.0.113.9"          → 203.0.113.9
  remote 127.0.0.1, trusted 127.0.0.0/8+10.0.0.0/8, XFF "1.1.1.1, 203.0.113.9, 10.0.0.2" → 203.0.113.9 (rightmost untrusted)
  remote 127.0.0.1, trusted 127.0.0.0/8, XFF "garbage"              → 127.0.0.1 */ }
func TestIPHasher(t *testing.T) { /* 2001:db8:1:2::1 and 2001:db8:1:2:ffff::9 → equal; 2001:db8:1:3::1 → different;
  ::ffff:203.0.113.7 == 203.0.113.7; len == 32; a different secret → a different hash */ }
func TestLimiter(t *testing.T) { /* Rule{"x",5,time.Hour}: 5 × Allow → true; 6th → false with retryAfter in (0, 1h];
  Advance(time.Hour) → true; a different key is independent */ }
func TestCSRF(t *testing.T) { /* POST /api/x without the header → 403 csrf; with "X-PB-CSRF: 1" → reaches the handler;
  GET /api/x without the header → reaches it; POST /r/abc (not /api) → reaches it */ }
// testutil_test.go
func TestHarness(t *testing.T) { /* app.Client(t).Do("GET","/healthz",nil).Want(t,200);
  two clients have different IPs; the RequestInfo IP hash differs between them (assert through a test-only handler) */ }
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/httpx/... ./internal/testutil/...` — Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

- `Limiter` uses fixed windows: key `rule.Name|key|windowStart`, where windowStart = now truncated to `rule.Window`. Expired windows are swept on every 1024th `Allow` call.
- `server.New` wraps the mux as `RequestInfo(CSRF(mux))`.
- `testutil.New` runs the real `server.New` behind an `httptest.Server`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./...` — Expected: PASS.

- [ ] **Step 5: Commit**

`git add -A && git commit -m "add client ip, ip hashing, csrf guard, rate limiter and test harness"`

---

### Task B3: Markdown rendering and preview

**Files:**
- Create: `internal/markdown/markdown.go`, `internal/markdown/preview.go`, `internal/markdown/markdown_test.go`
- Modify: `internal/server/server.go` (route)

**Interfaces:**
- Consumes: `httpx.Limiter`, `httpx.RulePreview`, `httpx.InfoFrom`
- Produces: `markdown.Render(md string) string`; `markdown.Excerpt(md string, max int) string`; `markdown.PreviewHandler(lim *httpx.Limiter) httpx.HandlerFunc`; `markdown.MaxBody = 10000`

- [ ] **Step 1: Write the failing tests**

```go
func TestRender(t *testing.T) { /* table of input → assertions on the output:
  "**bold**"                         contains "<strong>bold</strong>"
  "<script>alert(1)</script>"        not contains "<script"
  "<b>raw</b>"                       not contains "<b>"
  "[x](javascript:alert(1))"         not contains "javascript:"
  "![a](https://e.com/a.png)"        not contains "<img"
  "see https://example.com"          contains `href="https://example.com"`, `nofollow`, `target="_blank"`
  "# Title"                          contains "<h1"
  "| a | b |\n|---|---|\n| 1 | 2 |"  contains "<table>"
  "~~old~~"                          contains "<del>"
  "- [x] done"                       not contains "<input" */ }
func TestExcerpt(t *testing.T) { /* Excerpt("Na **\"Loja\"** do Xodet\n\n- item", 140) == `Na "Loja" do Xodet item`;
  a 300-rune text → at most 141 runes, ending in "…", cut at a word boundary; Excerpt("", 140) == "" */ }
func TestPreviewHandler(t *testing.T) { /* via testutil: POST /api/markdown/preview {"markdown":"**a**"} → 200 html contains "<strong>a</strong>";
  10001 chars → 422, Field("markdown") == "too_long"; the 121st call within a minute from one IP → 429 rate_limited */ }
```

- [ ] **Step 2: Run them to verify they fail** — `go test ./internal/markdown/...` → FAIL.

- [ ] **Step 3: Implement**

- goldmark with `extension.GFM` minus `TaskList` (Table, Strikethrough, Linkify), and *without* `html.WithUnsafe()`.
- The output goes through a package-level bluemonday policy built from `bluemonday.NewPolicy()`. Allowed elements: `p br strong em del code pre blockquote ul ol li h1 h2 h3 h4 h5 h6 table thead tbody tr th td hr a`. `href` is allowed on `a`, with `AllowStandardURLs`, `RequireNoFollowOnLinks(true)` and `AddTargetBlankToFullyQualifiedLinks(true)`. `align` is allowed on `th` and `td`.
- `Excerpt` renders, strips tags (bluemonday `StrictPolicy`), unescapes HTML entities, collapses whitespace and truncates.

- [ ] **Step 4: Verify they pass** — `go test ./...` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "add sanitized markdown rendering and preview endpoint"`

---

### Task B4: Guest identity, actor middleware, bans check

**Files:**
- Create: `internal/auth/actor.go`, `internal/auth/guest.go`, `internal/auth/bans.go`, `internal/auth/handlers_me.go`, `internal/auth/guest_test.go`, `internal/store/queries/guests.sql`, `internal/store/queries/bans.sql`
- Modify: `internal/server/server.go`, `internal/api/types.go` (author helper)

**Interfaces:**
- Consumes: `store.InTx`, `httpx.InfoFrom`, `httpx.Limiter`, `config.Config`, `clock.Clock`
- Produces:
  - `auth.Guest{ID string; Name *string; CreatedAt time.Time}`; `auth.Admin{ID int64; Username, DisplayName string}`
  - `auth.Actor{Guest *Guest; Admin *Admin; Info httpx.Info}`; `auth.ActorFrom(ctx) *Actor` (never nil behind the middleware); `(*Actor) IsAdmin() bool`
  - `auth.AuthorRef{GuestID *string; AdminID *int64}`
  - `auth.NewService(db *sql.DB, c clock.Clock, lim *httpx.Limiter, cfg config.Config) *auth.Service`
  - `(*Service) Middleware(next http.Handler) http.Handler` loads the guest from `pb_guest` and the admin from a non-expired `pb_admin` session (the admin part becomes real in B5)
  - `(*Service) EnsureGuest(w http.ResponseWriter, r *http.Request) (*Guest, error)`: returns the existing guest, or (after the `guest.create` limit and the IP ban check) creates one and sets the cookie. It also updates `last_ip_hash` and re-sets the cookie on every call.
  - `(*Service) Writer(w, r) (AuthorRef, error)`: an admin acts as admin; anyone else goes through `EnsureGuest` and then `CheckNotBanned`
  - `(*Service) GuestWriter(w, r) (*Guest, error)`: always `EnsureGuest` + `CheckNotBanned`
  - `(*Service) CheckNotBanned(ctx, guestID string, ipHash string) error` (403 `banned` if any ban matches the guest id or the IP hash)
  - `(*Service) Limit(r *http.Request, rule httpx.Rule) error` (admins exempt; otherwise keyed by the IP hash)
  - `api.NewAuthor(guestID sql.NullString, guestName sql.NullString, adminID sql.NullInt64, adminName sql.NullString) api.Author`
  - routes `GET /api/me`, `PUT /api/me/name`, `GET /r/{token}`

- [ ] **Step 1: Write the failing tests**

```go
func TestMe_Anonymous(t *testing.T)               // GET /api/me → {guest:null, admin:null}; no Set-Cookie header
func TestSetName_CreatesGuest(t *testing.T) {
	// PUT /api/me/name {"name":"WuzeraProMAX"} → 200; cookie pb_guest is HttpOnly, SameSite=Lax, MaxAge 34560000, not Secure (http base URL)
	// me.guest.name == "WuzeraProMAX"; len(me.guest.id) == 16; me.guest.recoveryUrl has prefix "http://pb.test/r/"
}
func TestSetName_TrimsAndValidates(t *testing.T)  // "   " → name null; 41 runes → 422 Field("name") == "too_long"; 40 runes "ç…" → 200
func TestRecoveryLink(t *testing.T) {
	// a := client A sets a name; b := a new client GETs the path of a's recoveryUrl → 302, Location "/?recovered=1", Set-Cookie pb_guest
	// b's /api/me guest.id == a's id; GET /r/not-a-token → 302 "/?recovered=0", no cookie
}
func TestGuestCreate_RateLimited(t *testing.T)    // 20 fresh clients WithIP("203.0.113.1") set names → 200; the 21st → 429 rate_limited
func TestBannedIP_CannotCreateGuest(t *testing.T) // Exec INSERT INTO bans(ip_hash, created_by...) for 203.0.113.2's hash → a new client from that IP: PUT name → 403 banned
func TestBannedGuest_CannotWrite(t *testing.T)    // a guest exists; Exec a ban on its guest_id → PUT name → 403 banned
```

Inserting a ban row needs `created_by`, which references an admin. Insert a placeholder admin row with `testutil.Exec` first.

- [ ] **Step 2: Verify they fail** — `go test ./internal/auth/...` → FAIL.

- [ ] **Step 3: Implement**

- Tokens are 32 random bytes encoded as base64url (no padding); `token_hash` = sha256(token).
- The guest id is 10 random bytes as lowercase base32 without padding (16 chars).
- The recovery URL is `cfg.BaseURL + "/r/" + token`.
- `/r/{token}` sets the cookie only if the token's hash exists.
- `Me` for an admin session is filled in B5; here `admin` stays null.

- [ ] **Step 4: Verify they pass** — `go test ./...` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "add guest identities, recovery links and ban checks"`

---

### Task B5: Admin accounts, sessions, invites, team, CLI

**Files:**
- Create: `internal/auth/password.go`, `internal/auth/admin.go`, `internal/auth/session.go`, `internal/auth/invites.go`, `internal/auth/handlers_admin.go`, `internal/auth/password_test.go`, `internal/auth/admin_test.go`, `internal/store/queries/admins.sql`, `cmd/patchboard/admin.go`, `cmd/patchboard/main_test.go`
- Modify: `internal/auth/actor.go` (load the admin session), `internal/auth/handlers_me.go` (fill `Me.admin`), `internal/server/server.go`, `internal/testutil/testutil.go`

**Interfaces:**
- Consumes: the B4 `auth.Service`, `httpx.RuleLoginIP`, `httpx.RuleLoginUser`
- Produces:
  - `auth.HashPassword(pw string) (string, error)`; `auth.VerifyPassword(encoded, pw string) (bool, error)`. PHC format `$argon2id$v=19$m=19456,t=2,p=1$<salt b64>$<hash b64>`, 16-byte salt, 32-byte key.
  - `auth.CreateAdmin(ctx, db *sql.DB, now time.Time, username, displayName, password string) (int64, error)`. Validation: username matches `^[a-z0-9_.-]{3,32}$` after lower-casing; display name 1–40 runes; password ≥ 10 runes. A taken username → 409 `username_taken`.
  - `auth.ResetPassword(ctx, db, username, password string) error` (also deletes that admin's sessions)
  - `(*Service) RequireAdmin(next http.Handler) http.Handler` (401 `unauthorized`)
  - routes: every B5 row in the Endpoints table
  - testutil: `(*App) AdminClient(t, username string) *Client` creates the admin with password `"correct-horse-1"` and display name = the username title-cased, then logs in

- [ ] **Step 1: Write the failing tests**

```go
func TestPassword_RoundTrip(t *testing.T)        // has prefix "$argon2id$v=19$m=19456,t=2,p=1$"; correct password → true; wrong → false; two hashes of the same password differ
func TestLogin(t *testing.T)                     // CreateAdmin "wuzi"; POST login {"username":"WUZI","password":...} → 200; me.admin.username == "wuzi"; cookie pb_admin MaxAge 2592000
func TestLogin_WrongPasswordAndLimit(t *testing.T) // wrong password → 401 invalid_credentials; the 6th attempt from one IP within 15 min → 429
func TestLogin_DisabledAdmin(t *testing.T)       // Exec disabled_at → correct password → 401 invalid_credentials
func TestAdminRoutes_RequireSession(t *testing.T) // GET /api/admin/team: no cookie → 401 unauthorized; guest cookie only → 401
func TestSession_Expires(t *testing.T)           // log in; Clock.Advance(30*24h + time.Second) → GET /api/admin/team → 401
func TestLogout(t *testing.T)                    // POST logout → 204; then /api/admin/team → 401
func TestInvite_Flow(t *testing.T) {
	// admin POST /api/admin/invites → 201, url has prefix "http://pb.test/admin/invite/", expiresAt = now + 7 days
	// GET /api/admin/invites/<token> (public) → {valid:true}
	// a new client accepts {"username":"Mod.One","displayName":"Mod","password":"long-enough-pw"} → 201; me.admin.username == "mod.one"
	// accepting again → 410 invite_used; another invite after Clock.Advance(7*24h+1s) → 410 invite_expired
}
func TestInvite_UsernameTaken(t *testing.T)      // accept with "WUZI" while "wuzi" exists → 409 username_taken
func TestDisable(t *testing.T)                   // disabling self → 422 cannot_disable_self; disabling another → 204, they can no longer log in and their sessions are deleted
                                                 // (because self-disable is refused, at least one active admin always remains; no separate last-admin rule is needed)
func TestUpdateAdminMe(t *testing.T)             // change displayName → me.admin.displayName; newPassword with a wrong currentPassword → 422 Field("currentPassword")=="invalid"
// cmd/patchboard/main_test.go
func TestCLI_AdminCreateAndReset(t *testing.T) {
	// getenv provides PB_BASE_URL and PB_DATA_DIR=t.TempDir()
	// run(["admin","create","--username","wuzi","--name","Wuzi"], stdin "supersecret1\n") == 0
	// run(["admin","reset-password","--username","wuzi"], stdin "anothersecret\n") == 0; VerifyPassword with the new password → true
	// run(["admin","create","--username","x"], ...) without --name → exit code 2 and usage on stderr
}
```

- [ ] **Step 2: Verify they fail** — `go test ./internal/auth/... ./cmd/...` → FAIL.

- [ ] **Step 3: Implement**

- On a terminal, the CLI reads the password with `term.ReadPassword` (asked twice; they must match). Otherwise it reads one line from stdin.
- The CLI opens the store and runs `store.Migrate` itself, so it works before the server has ever started.

- [ ] **Step 4: Verify they pass** — `go test ./...` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "add admin accounts, sessions, invites and cli"`

---

### Task B6: Board settings, statuses and tags

**Files:**
- Create: `internal/board/board.go`, `internal/board/handlers.go`, `internal/board/board_test.go`
- Modify: `internal/store/queries/board.sql`, `internal/server/server.go`, `internal/testutil/testutil.go` (`InsertPost`)

**Interfaces:**
- Consumes: `auth.Service.RequireAdmin`, `store.InTx`
- Produces:
  - `board.NewService(db *sql.DB) *board.Service`; `(*Service) Get(ctx) (api.Board, error)`; `(*Service) DefaultStatusID(ctx) (int64, error)`; `(*Service) StatusExists(ctx, id int64) (bool, error)`; `(*Service) TagExists(ctx, id int64) (bool, error)`; `(*Service) Settings(ctx) (gen.BoardSetting, error)`
  - routes: every B6 row in the Endpoints table
  - testutil: `testutil.InsertPost(t, db, title string, statusID int64) int64` (raw SQL, guest-less admin author created on demand), for tests that run before B7

- [ ] **Step 1: Write the failing tests**

```go
func TestBoard_Public(t *testing.T)          // GET /api/board: 6 statuses ordered by position; statuses[0].name == {pt:"Sugestões", en:"Suggestions"}, isDefault true; 3 tags; accentColor "#c026d3"; logoUrl null
func TestSettings_Update(t *testing.T)       // admin PUT valid settings (title {pt:"Venore Survivors • Comunidade", en:"Venore Survivors • Community"}) → GET reflects them
                                             // accentColor "red" → 422 Field("accentColor")=="invalid"; title.pt "" → Field("title.pt")=="required"; 81 runes → "too_long"; guest → 401
func TestStatuses_CreateOrderDefault(t *testing.T) {
	// POST {name:{pt:"Em teste",en:"Testing"}, color:"#0EA5E9", showOnRoadmap:true} → 201, position 6
	// PUT order with the ids reversed → GET shows that order with positions 0..6; an order missing an id → 422 invalid_order
	// PATCH status 3 {isDefault:true} → status 3 is default and status 1 is not
}
func TestStatuses_Delete(t *testing.T) {
	// DELETE the default status → 422 cannot_delete_default
	// InsertPost(…, statusID 2); DELETE /statuses/2 → 422 move_required; DELETE /statuses/2?moveTo=3 → 204, the post is now in status 3
	// a status with no posts: DELETE without moveTo → 204
}
func TestTags_CRUD(t *testing.T)             // create, rename, reorder; delete a tag used by a post → 204 and that post's tag_id IS NULL; name.en of 31 runes → 422
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.** A status delete with `moveTo` updates `posts.status_id` and `update_posts.status_id` before deleting, in one transaction. Colours must match `^#[0-9A-Fa-f]{6}$`. `logoUrl` = `/uploads/<id>.<ext>` when set.
- [ ] **Step 4: Verify they pass** — `go test ./...` → PASS.
- [ ] **Step 5: Commit** — `git commit -m "add board settings, statuses and tags"`

---

### Task B7: Posts — create, read, edit, delete, approval

**Files:**
- Create: `internal/posts/posts.go`, `internal/posts/slug.go`, `internal/posts/visibility.go`, `internal/posts/handlers.go`, `internal/posts/posts_test.go`, `internal/posts/slug_test.go`, `internal/store/queries/posts.sql`
- Modify: `internal/server/server.go`, `internal/testutil/testutil.go` (`NewPost`)

**Interfaces:**
- Consumes: `auth.Service.Writer`, `auth.ActorFrom`, `(*auth.Service).Limit`, `board.Service`, `markdown.Render`, `markdown.Excerpt`
- Produces:
  - `posts.Slugify(title string) string`
  - `posts.Deps{DB *sql.DB; Clock clock.Clock; Auth *auth.Service; Board *board.Service; Comments CommentLoader; Images ImageStore; CreateUpdate UpdateCreator}`, where:
    - `type CommentLoader func(ctx context.Context, postID int64, viewer *auth.Actor) ([]api.Comment, error)`
    - `type ImageStore interface { ForPost(ctx, postID int64) ([]api.Image, error); Attach(ctx context.Context, tx *sql.Tx, postID int64, ids []string, owner auth.AuthorRef) error }`
    - `type UpdateCreator func(ctx context.Context, tx *sql.Tx, adminID int64, title, body string, postIDs []int64) (int64, error)`
    - In this task the server passes a `CommentLoader` that returns `[]`, an `ImageStore` whose `ForPost` returns `[]` and whose `Attach` rejects non-empty ids with 422 `imageIds:invalid`, and a nil `UpdateCreator`.
  - `posts.NewService(d posts.Deps) *posts.Service`
  - `(*Service) Detail(ctx, id int64, viewer *auth.Actor) (api.PostResponse, error)`, used by later posts tasks
  - Unexported helpers in `visibility.go`, `visible(p postRow, viewer *auth.Actor) bool` and `canEdit(p postRow, viewer *auth.Actor, defaultStatusID int64) bool`, are the only place these rules live; later posts tasks call them
  - routes: POST/GET/PATCH/DELETE for posts (see Endpoints)
  - testutil: `(*Client) NewPost(t, title string) api.PostDetail`

Rules:
- Viewing: a deleted post is visible only to admins. A pending post is visible only to its author and admins (anyone else gets 404). A merged post returns `{post:null, mergedInto:{id,slug}}`, following the chain.
- Edit/delete: admins always (unless the post is merged). The author only while `status_id` = the default status, the post isn't deleted and isn't merged; otherwise 403 `edit_window_closed`. Anyone else gets 403 `forbidden`.
- Create: `pending = 1` when `require_approval` is on and the author isn't an admin.

- [ ] **Step 1: Write the failing tests**

```go
func TestSlugify(t *testing.T) { /* "Aumentar tamanho do chat" → "aumentar-tamanho-do-chat"
  "Ação rápida 🎮!!" → "acao-rapida"; `Remoção da "Boots of Haste"` → "remocao-da-boots-of-haste"
  "🎮🎮" → "post"; a 120-char title → len ≤ 60 and no trailing "-" */ }
func TestCreatePost_Guest(t *testing.T) {
	// POST {"title":"Aumentar tamanho do chat","body":"**pls**"} → 201; statusId 1; slug "aumentar-tamanho-do-chat"
	// bodyHtml contains "<strong>pls</strong>"; excerpt "pls"; author.kind "guest"; canEdit true; voteCount 0; pb_guest cookie set
}
func TestCreatePost_Validation(t *testing.T) // title "ab" → Field("title")=="too_short"; 121 runes → "too_long"; body 10001 → Field("body")=="too_long"; tagId 999 → Field("tagId")=="invalid"
func TestCreatePost_Admin(t *testing.T)      // AdminClient "wuzi" → author {kind:"admin", id:"admin-1", name:"Wuzi"}; no pb_guest cookie set
func TestCreatePost_RateLimit(t *testing.T)  // 5 → 201; the 6th → 429 rate_limited; an admin is not limited (6 → 201)
func TestCreatePost_Approval(t *testing.T) {
	// admin sets requireApproval true; a guest creates → 201 pending true
	// other guest GET → 404; author GET → 200 pending true; admin GET → 200
}
func TestEditPost_Window(t *testing.T) {
	// author PATCH {"title":"Aumentar o chat"} → 200, slug "aumentar-o-chat"
	// Exec status_id = 3 → author PATCH → 403 edit_window_closed; admin PATCH {"clearTag":true} → 200 tagId null
	// another guest PATCH → 403 forbidden
}
func TestDeletePost(t *testing.T)            // another guest DELETE → 403; author DELETE → 204; GET → 404; admin GET → 200 with deleted true
func TestGetPost_BadID(t *testing.T)         // GET /api/posts/abc → 404; /api/posts/999 → 404
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.** `Slugify`: NFD (`golang.org/x/text/unicode/norm`), drop `unicode.Mn`, lowercase, map runs of non-`[a-z0-9]` to `-`, trim `-`, cut at 60 on a `-` boundary when possible, empty → `"post"`. The single-post read and `Detail` use sqlc queries; `Detail` fills `comments` through `Deps.Comments` and `images` through `Deps.Images`.
- [ ] **Step 4: Verify they pass** — `go test ./...` → PASS.
- [ ] **Step 5: Commit** — `git commit -m "add posts: create, read, edit, delete and approval queue"`

---

### Task B8: Votes

**Files:**
- Create: `internal/posts/votes.go`, `internal/posts/votes_test.go`, `internal/store/queries/votes.sql`
- Modify: `internal/posts/handlers.go`, `internal/testutil/testutil.go` (`Vote`)

**Interfaces:**
- Consumes: `auth.Service.GuestWriter`, `(*auth.Service).Limit` with `httpx.RuleVote`
- Produces: `(*posts.Service) Vote(ctx, postID int64, guestID, ipHash string, up bool) (api.VoteState, error)`; routes POST/DELETE `/api/posts/{id}/vote`; testutil `(*Client) Vote(t, postID int64) api.VoteState`

- [ ] **Step 1: Write the failing tests**

```go
func TestVote_Toggle(t *testing.T)        // POST twice → {voteCount:1, voted:true}; DELETE → {0,false}; DELETE again → {0,false}
func TestVote_HiddenPosts(t *testing.T)   // vote on a pending, deleted or merged post (set via Exec) → 404
func TestVote_Banned(t *testing.T)        // ban the guest → 403 banned
func TestVote_RateLimit(t *testing.T)     // 120 alternating POST/DELETE → OK; the 121st → 429
func TestVote_AdminWithoutGuestCookie(t *testing.T) // AdminClient (no pb_guest) POST vote → 200 voted true; client now has a pb_guest cookie
func TestVote_Concurrent(t *testing.T)    // 20 clients vote on one post in parallel goroutines → all 200; GET post voteCount == 20
func TestVote_RecordsIPHash(t *testing.T) // the votes row has ip_hash == the client IP's hash (compare through IPHasher with app.Cfg.Secret)
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.** `INSERT … ON CONFLICT DO NOTHING` / `DELETE`, then recount `vote_count`, all inside `store.InTx`.
- [ ] **Step 4: Verify they pass** — `go test -race ./internal/posts/...` → PASS.
- [ ] **Step 5: Commit** — `git commit -m "add voting"`

---

### Task B9: List and roadmap

**Files:**
- Create: `internal/posts/list.go`, `internal/posts/list_test.go`
- Modify: `internal/posts/handlers.go`

**Interfaces:**
- Consumes: the B8 votes table, `board.Service`
- Produces:
  - `posts.ListQuery{Sort string; StatusID, TagID *int64; Q string; Offset, Limit int}`; `(*Service) List(ctx, viewer *auth.Actor, q ListQuery) (api.PostPage, error)`
  - `(*Service) Roadmap(ctx, viewer *auth.Actor, tagID *int64, q string) (api.Roadmap, error)`
  - `(*Service) cards(ctx, viewer, ids []int64) (map[int64]api.PostCard, error)` (internal: adds `voted`, `voters`, `newVotes`)
  - routes GET `/api/posts`, GET `/api/roadmap`

Hand-written SQL (not sqlc):
- `newVotes` = count of votes with `created_at >= now - 7*24h`.
- Orders:
  - trending: `pinned_at IS NULL, newVotes DESC, vote_count DESC, id DESC`
  - top: `pinned_at IS NULL, vote_count DESC, id DESC`
  - new: `pinned_at IS NULL, created_at DESC, id DESC`
- Pinned posts come first unless `q` is set.
- `voters`: the 3 most recent per post, via `ROW_NUMBER() OVER (PARTITION BY post_id ORDER BY created_at DESC, guest_id)`.
- Invalid `sort` → trending; non-numeric `status`/`tag` → 422. `limit` defaults to 30, values above 50 become 50.
- Roadmap: one column per status with `show_on_roadmap = 1`, ordered by position, each with up to 100 cards by `vote_count DESC, id DESC` plus `total`.

- [ ] **Step 1: Write the failing tests**

```go
func TestList_Sorts(t *testing.T) {
	// A, B, C created in that order. A gets 3 votes, then Clock.Advance(8 days); B gets 2 votes, C gets 1.
	// trending → [B, C, A]; top → [A, B, C]; new → [C, B, A]; B.newVotes == 2, A.newVotes == 0
}
func TestList_PinnedFirst(t *testing.T)    // Exec pinned_at on A → trending [A, B, C], A.pinned true; with q set, pinned gets no priority
func TestList_Filters(t *testing.T)        // status=2 returns only status-2 posts; tag=1 likewise; status=x → 422
func TestList_ExcludesHidden(t *testing.T) // pending, deleted and merged posts are not in the list or any roadmap column; a pending post is not listed even for its author
func TestList_VotedAndVoters(t *testing.T) // the viewer voted on B → B.voted true, C.voted false; 4 voters → voters length 3, most recent first, with names
func TestList_Pagination(t *testing.T)     // 35 posts: default page has 30 and nextOffset 30; offset=30 → 5 posts, nextOffset null
                                           // 60 posts with limit=100 → 50 posts (capped) and nextOffset 50
func TestRoadmap(t *testing.T)             // 6 columns in position order; PATCH status 6 showOnRoadmap=false → 5 columns; column posts by voteCount desc; total == count
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement** as described above.
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add post list sorting, filters and roadmap"`

---

### Task B10: Search and similar posts

**Files:**
- Create: `internal/posts/search.go`, `internal/posts/search_test.go`
- Modify: `internal/posts/list.go` (apply `q` to List and Roadmap), `internal/posts/handlers.go`

**Interfaces:**
- Produces: `posts.FTSQuery(input string, any bool) string` (`""` when there are no usable tokens); `(*Service) Similar(ctx, title string) (api.Similar, error)`; route `GET /api/posts/similar` (the literal pattern takes precedence over `/api/posts/{id}` in `ServeMux`)

`FTSQuery` splits on runes that are neither `unicode.IsLetter` nor `unicode.IsNumber` and keeps at most 8 tokens. Each token becomes `"tok"*`. Tokens are joined with spaces (AND) when `any` is false (search, minimum 1 rune per token), and with ` OR ` when `any` is true (similar, minimum 3 runes per token). Filtering is `id IN (SELECT rowid FROM posts_fts WHERE posts_fts MATCH ?)`. Similar results are ordered by `bm25(posts_fts)`, limited to 5 and publicly visible only.

- [ ] **Step 1: Write the failing tests**

```go
func TestFTSQuery(t *testing.T) // ("Análise de dano", false) → `"Análise"* "de"* "dano"*`; (`"chat" OR (`, false) → `"chat"* "OR"*`; ("ab cd", true) → ""; ("tamanho do chat", true) → `"tamanho"* OR "chat"*`
func TestSearch_AccentInsensitive(t *testing.T) // post "Melhorar análise de dano": q=analise, q=ANÁL and q=dano each find it; q="chat xyz" finds nothing; q=`") OR (` → 200 with an empty list
func TestSearch_AppliesToRoadmap(t *testing.T)  // /api/roadmap?q=chat → only matching cards; total reflects the filter
func TestSimilar(t *testing.T)                  // posts "Aumentar tamanho do chat" and "Co-op"; title "chat maior por favor" → first is "Aumentar tamanho do chat"; "ab" → empty; at most 5 results
func TestSimilar_ExcludesHidden(t *testing.T)   // pending, deleted and merged posts with matching titles are never returned
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add accent-insensitive search and similar-post hints"`

---

### Task B11: Comments and likes

**Files:**
- Create: `internal/comments/comments.go`, `internal/comments/likes.go`, `internal/comments/handlers.go`, `internal/comments/comments_test.go`, `internal/store/queries/comments.sql`
- Modify: `internal/server/server.go` (wire `posts.Deps.Comments = commentsSvc.ForPost`)

**Interfaces:**
- Consumes: `auth.Service.Writer` / `GuestWriter` / `Limit`
- Produces:
  - `(*posts.Service) VisibleTo(ctx, postID int64, viewer *auth.Actor) (bool, error)`, added to `internal/posts/visibility.go`; it wraps the B7 `visible` rule
  - `comments.NewService(db *sql.DB, c clock.Clock, a *auth.Service, p *posts.Service) *comments.Service`
  - `(*Service) ForPost(ctx, postID int64, viewer *auth.Actor) ([]api.Comment, error)`
  - `(*Service) Restore(ctx, id int64) error`; `(*Service) Trash(ctx, limit int) ([]api.TrashComment, error)` (used by B14)
  - routes: every B11 row in the Endpoints table

Rules:
- A reply whose `parentId` is itself a reply is attached to that reply's parent.
- `parentId` must belong to the same post (422 `parentId:invalid`).
- A locked post takes comments only from admins (403 `locked`).
- `comment_count` counts non-deleted comments.
- A deleted top-level comment that has non-deleted replies is returned as `{deleted:true, author:null, bodyHtml:null, bodyMd:null}`; other deleted comments are omitted.
- Authors (guest or admin) and any admin can edit and delete.

- [ ] **Step 1: Write the failing tests**

```go
func TestComment_Thread(t *testing.T)      // c1; r1 → c1; r2 → r1 (stored with parentId c1); GET post → comments [c1{replies:[r1, r2]}]; commentCount 3
func TestComment_Locked(t *testing.T)      // Exec comments_locked=1 → guest POST → 403 locked; admin POST → 201, author.kind "admin"
func TestComment_Validation(t *testing.T)  // body "  " → Field("body")=="too_short"; 5001 → "too_long"; a parentId from another post → Field("parentId")=="invalid"
func TestComment_EditDelete(t *testing.T) {
	// author PATCH → 200, bodyHtml updated; another guest PATCH → 403
	// author DELETE c1 (which has replies) → c1 shows deleted:true with its replies; DELETE r1 → r1 omitted; commentCount 2 → 1 accordingly
}
func TestComment_Likes(t *testing.T)       // like twice → {1, true}; DELETE → {0, false}; liking a deleted comment → 404; GET shows liked for the viewer
func TestComment_HiddenPost(t *testing.T)  // commenting on a pending post as another guest → 404; on a deleted post → 404
func TestComment_RateLimit(t *testing.T)   // the 21st comment within 1h → 429
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add comments, replies and likes"`

---

### Task B12: Image uploads

**Files:**
- Create: `internal/uploads/process.go`, `internal/uploads/uploads.go`, `internal/uploads/handlers.go`, `internal/uploads/process_test.go`, `internal/uploads/uploads_test.go`, `internal/store/queries/uploads.sql`
- Modify: `internal/posts/posts.go` (`imageIds` on create/edit go through `Deps.Images.Attach`), `internal/server/server.go`

**Interfaces:**
- Consumes: `auth.Service.Writer`, `board.Service.Settings` (`uploads_enabled`), `config.Config.DataDir`
- Produces:
  - `uploads.Process(r io.Reader) (data []byte, ext string, w, h int, err error)`; `uploads.ErrNotImage`, `uploads.ErrTooManyPixels`
  - `uploads.NewService(db *sql.DB, c clock.Clock, a *auth.Service, b *board.Service, dataDir string) *uploads.Service`, which satisfies `posts.ImageStore`
  - `(*Service) CleanupOrphans(ctx) (int, error)`
  - routes POST `/api/uploads`, GET `/uploads/{name}`, DELETE `/api/admin/uploads/{id}`

Processing:
- Reject the body if it is over 5 MiB (`http.MaxBytesReader` at 5 MiB + 64 KiB for the multipart overhead; the file part itself must be ≤ 5,242,880 bytes) → 413 `too_large`.
- Run `image.DecodeConfig` first; width×height > 40,000,000 → `too_many_pixels`, before any full decode.
- Supported inputs: JPEG, PNG, GIF (first frame), WebP (`golang.org/x/image/webp`). Anything else → 422 `invalid_image`.
- Scale down to ≤ 2560 px on the longest side (`draw.CatmullRom`).
- PNG/GIF input → PNG output; JPEG/WebP input → JPEG at quality 85.
- Files go to `<data>/uploads/<id>.<ext>`.
- Attach: ids must exist, be owned by the same `AuthorRef`, and be either unattached or already on this post; at most 4 (`imageIds:too_many`), otherwise `imageIds:invalid`. `position` follows the order of the ids. Uploads that are no longer in the list are detached (`post_id = NULL`).
- Cleanup deletes the row and the file for uploads where `post_id IS NULL`, `created_at < now - 24h`, and the upload isn't the board logo.
- Serving checks the name against `^[a-z2-7]{20}\.(jpg|png)$` and sets `Content-Type`, `Cache-Control: public, max-age=31536000, immutable` and `X-Content-Type-Options: nosniff`.

- [ ] **Step 1: Write the failing tests**

```go
func TestProcess_StripsMetadata(t *testing.T)     // a JPEG with an APP1 "Exif\x00\x00" segment spliced in after SOI → output does not contain "Exif"; ext "jpg"
func TestProcess_Resizes(t *testing.T)            // a 3000×1000 PNG → 2560×853, ext "png"
func TestProcess_GIFBecomesPNG(t *testing.T)      // GIF → ext "png"
func TestProcess_RejectsNonImage(t *testing.T)    // []byte("hello") → ErrNotImage
func TestProcess_RejectsTooManyPixels(t *testing.T) // a 1×1 PNG with IHDR patched to 20000×20000 and the CRC recomputed (helper hugePNG) → ErrTooManyPixels
func TestUpload_Endpoint(t *testing.T) {
	// POST /api/uploads with a 200×100 JPEG → 201 {id (20 chars), url "/uploads/<id>.jpg", width 200, height 100}
	// a 6 MiB body → 413 too_large; text → 422 invalid_image
	// admin sets uploadsEnabled false → guest 403 uploads_disabled, admin 201
	// the 21st upload in 1h from one IP → 429
}
func TestUpload_AttachToPost(t *testing.T) {
	// create a post with imageIds [u1,u2] → images [u1,u2] in that order; 5 ids → Field("imageIds")=="too_many"
	// another guest's upload id → Field("imageIds")=="invalid"; PATCH imageIds [u2] → images [u2] and u1 is detached
}
func TestUpload_Serve(t *testing.T)               // GET url → 200, image/jpeg, the cache header, nosniff; GET /uploads/..%2fsecret → 404; GET /uploads/aaaa.jpg → 404
func TestCleanupOrphans(t *testing.T)             // an unattached upload, Clock.Advance(25h) → CleanupOrphans == 1, file gone; attached uploads and the logo stay
func TestAdminDeleteUpload(t *testing.T)          // DELETE → 204; file gone; the post's images are empty
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.** `go get golang.org/x/image@v0.48.0`.
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add image uploads with re-encoding and cleanup"`

---

### Task B13: Status changes, team responses, Updates

**Files:**
- Create: `internal/posts/status.go`, `internal/posts/status_test.go`, `internal/updates/updates.go`, `internal/updates/handlers.go`, `internal/updates/updates_test.go`, `internal/store/queries/updates.sql`
- Modify: `internal/server/server.go` (wire `posts.Deps.CreateUpdate = updatesSvc.CreateTx`)

**Interfaces:**
- Consumes: `auth.Service.RequireAdmin`, `markdown.Render`, `posts.Deps.CreateUpdate`
- Produces:
  - `updates.NewService(db *sql.DB, c clock.Clock) *updates.Service`
  - `(*Service) CreateTx(ctx, tx *sql.Tx, adminID int64, title, body string, postIDs []int64) (int64, error)` (matches `posts.UpdateCreator`)
  - `(*posts.Service) ChangeStatus(ctx, admin *auth.Admin, postID int64, req api.ChangeStatusRequest) (api.PostDetail, error)`, with `api.ChangeStatusRequest{StatusID int64; Response *string; Update *struct{Title, Body string}}` (add it to `internal/api/types.go`)
  - routes: every B13 row in the Endpoints table

Rules:
- `response` nil → unchanged; `""` → cleared; otherwise set (≤ 5000), with `response_admin_id` and `response_at` = now.
- Changing to the same status is allowed.
- Update: title 1–120, body 1–10000, `postIds` ≤ 10 and every id must be a post that isn't deleted (`postIds:invalid` / `too_many`).
- Each link snapshots the post's current `status_id`. PATCH with `postIds` replaces the links, keeping the existing snapshots for ids that stay.
- Public list: newest `published_at` first, 20 per page. Linked posts that aren't publicly visible are left out.

- [ ] **Step 1: Write the failing tests**

```go
func TestChangeStatus_WithResponseAndUpdate(t *testing.T) {
	// admin POST status {statusId:3, response:"Vamos fazer!", update:{title:"Chat maior planejado", body:"Obrigado pelos votos."}}
	// → statusId 3, response.html contains "Vamos fazer!", response.author.kind "admin"
	// GET /api/updates → 1 item, title "Chat maior planejado", posts[0] {id, statusId:3}
}
func TestChangeStatus_ClearResponse(t *testing.T)   // response "" → response null; omitting response keeps the previous one
func TestChangeStatus_Errors(t *testing.T)          // statusId 99 → Field("statusId")=="invalid"; guest → 401; a deleted post → 404
func TestUpdate_StatusSnapshot(t *testing.T)        // link a post in status 3, then change the post to status 5 without an update → the update still shows statusId 3
func TestUpdates_Validation(t *testing.T)           // title "" → required; 11 postIds → Field("postIds")=="too_many"; postIds [999] → "invalid"
func TestUpdates_HideDeletedPosts(t *testing.T)     // linked post deleted → the update's posts no longer include it; pending/merged likewise
func TestUpdates_EditDeletePaginate(t *testing.T)   // PATCH the title → reflected; DELETE → gone; 25 updates → page 1 has 20 and nextOffset 20
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.** `ChangeStatus` runs in one `store.InTx`, including `CreateUpdate` when `update` is present.
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add status changes, team responses and updates"`

---

### Task B14: Moderation — approval, pin, lock, restore, trash, merge

**Files:**
- Create: `internal/posts/moderation.go`, `internal/posts/merge.go`, `internal/posts/moderation_test.go`, `internal/posts/merge_test.go`, `internal/server/trash.go`
- Modify: `internal/comments/comments.go` (restore + trash list), `internal/server/server.go`

**Interfaces:**
- Produces:
  - `(*posts.Service)` methods `Pending(ctx)`, `Approve(ctx, id)`, `SetPinned(ctx, id, bool)`, `SetLocked(ctx, id, bool)`, `Restore(ctx, id)`, `Merge(ctx, admin *auth.Admin, id, intoID int64) (api.PostDetail, error)`, `TrashPosts(ctx, limit int) ([]api.TrashPost, error)`
  - routes: every B14 row in the Endpoints table (`/api/admin/trash` combines posts and comments, 100 each, most recently deleted first; `deletedBy` is `"admin"` when `deleted_by_admin_id` is set)

Merge, in one transaction:
- 422 `invalid_merge` if `id == intoID`, either post is deleted, the target is merged, or the source is already merged.
- Votes: `INSERT OR IGNORE INTO votes SELECT intoID, guest_id, ip_hash, created_at FROM votes WHERE post_id = id`, then delete the source's votes.
- `UPDATE comments SET post_id = intoID WHERE post_id = id`.
- Set the source's `merged_into_id = intoID`, and `UPDATE posts SET merged_into_id = intoID WHERE merged_into_id = id`.
- Recount `vote_count` and `comment_count` on both posts.

- [ ] **Step 1: Write the failing tests**

```go
func TestApproval(t *testing.T)     // requireApproval on; guest post → GET /api/admin/pending lists it (oldest first); approve → listed publicly, pending false
func TestPinAndLock(t *testing.T)   // pin → pinned true, first in the list; unpin; lock → guest comment 403 locked; unlock → 201
func TestRestore(t *testing.T)      // an author-deleted post appears in trash with deletedBy "author"; restore → GET 200 for guests; restoring a comment brings it back into the thread
func TestMerge(t *testing.T) {
	// A has votes from g1,g2 and comments c1; B has votes from g2,g3
	// merge A→B → B.voteCount 3 (g2 deduplicated), B's comments include c1; GET A → {post:null, mergedInto:{id:B, slug}}
	// A is gone from the list and from search
}
func TestMerge_Chain(t *testing.T)  // merge A→B, then B→C → GET A mergedInto C
func TestMerge_Invalid(t *testing.T) // A→A, into a merged post, or from a deleted post → 422 invalid_merge
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add moderation: approval, pin, lock, restore, trash and merge"`

---

### Task B15: Bans and vote audit

**Files:**
- Create: `internal/auth/bans_admin.go`, `internal/auth/bans_admin_test.go`, `internal/posts/audit.go`, `internal/posts/audit_test.go`
- Modify: `internal/store/queries/bans.sql`, `internal/store/queries/votes.sql`, `internal/server/server.go`

**Interfaces:**
- Produces: routes POST/GET/DELETE `/api/admin/bans`, GET `/api/admin/posts/{id}/votes`, DELETE `/api/admin/posts/{id}/votes/{guestId}`

Rules:
- A ban with `includeIp` stores the guest's `last_ip_hash` too.
- Vote audit flags:
  - `shared_ip`: at least 2 other voters on this post share the `ip_hash`.
  - `new_guest`: the guest was created < 10 minutes before the vote.
- Removing a vote recounts `vote_count` and returns `VoteState` (`voted` reflects the removed guest = false).

- [ ] **Step 1: Write the failing tests**

```go
func TestBan_GuestOnly(t *testing.T) // ban g1 with includeIp false → g1 vote 403 banned; g2 from the same IP → 200
func TestBan_WithIP(t *testing.T)    // includeIp true → a new client from g1's IP: setting a name → 403 banned; DELETE the ban → 200
func TestBans_List(t *testing.T)     // GET → [{guestId, guestName, ipHash (8 chars or null), reason}]
func TestVoteAudit(t *testing.T) {
	// 3 guests from IP X vote → each has the flag "shared_ip"; a guest from another IP, created 2 h before voting (Clock.Advance) → no flags
	// a guest that votes 5 min after being created → "new_guest"; ipHash has length 8
}
func TestRemoveVote(t *testing.T)    // DELETE …/votes/{g1} → voteCount decremented; that guest's GET shows voted false
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify they pass** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add bans and vote audit"`

---

### Task B16: Embedded SPA and link-preview meta tags

**Files:**
- Create: `internal/web/web.go`, `internal/web/web_test.go`, `internal/web/dist/.gitkeep`, `internal/server/meta.go`, `internal/server/meta_test.go`
- Modify: `internal/server/server.go` (catch-all `/`), `cmd/patchboard/serve.go` (`WebFS` from `web.Dist()`, hourly `uploads.CleanupOrphans` ticker plus one run at startup)

**Interfaces:**
- Produces:
  - `web.Dist() fs.FS` (`//go:embed all:dist`, sub-FS `dist`)
  - `web.Meta{Title, Description, URL, Image string}`; `web.MetaSource` interface `{ Meta(r *http.Request) web.Meta }`
  - `web.Handler(fsys fs.FS, src web.MetaSource) http.Handler`

Behaviour:
- An existing file is served as-is. Names matching `-[A-Z0-9]{8}\.(js|css|woff2?)$` get `Cache-Control: public, max-age=31536000, immutable`.
- Every other path gets `index.html` with `<!-- pb:meta -->` replaced, `Cache-Control: no-cache` and `Content-Type: text/html; charset=utf-8`.
- Injected tags: `<title>`, `meta name=description`, `og:site_name`, `og:type=website`, `og:title`, `og:description`, `og:url`, `og:image` (only when there is one) and `twitter:card` (`summary_large_image` with an image, `summary` without). All values are HTML-escaped.
- `server.metaSource`:
  - Path `/p/{id}-…` with a publicly visible post → the post title, its excerpt, `BaseURL/p/{id}-{slug}`, and the absolute URL of its first image (falling back to the logo).
  - Anything else, including hidden posts → the board title and description in `pt`, or in `en` when `Accept-Language` prefers `en`. Absolute logo URL.

- [ ] **Step 1: Write the failing tests**

```go
func TestWeb_StaticAndFallback(t *testing.T) // MapFS {index.html, main-ABCD1234.js}: the js → immutable cache; /roadmap → index with injected <title>, no-cache; /favicon.ico missing → index (SPA)
func TestWeb_EscapesMeta(t *testing.T)       // Meta{Title:`<script>"x"`} → body contains `&lt;script&gt;&#34;x&#34;` and not `<script>"x"`
func TestMeta_Post(t *testing.T)             // via testutil: a post with an image → GET /p/{id}-anything: og:title == the title, og:url == "http://pb.test/p/{id}-{slug}", og:image absolute, twitter:card summary_large_image
func TestMeta_HiddenPostUsesBoardDefaults(t *testing.T) // pending, deleted and merged posts → og:title == the board title, not the post title
func TestMeta_Language(t *testing.T)         // Accept-Language "en-US,en;q=0.9" → the board title.en; none → title.pt
func TestAPIStillJSON404(t *testing.T)       // GET /api/nope → JSON 404, not index.html
```

- [ ] **Step 2: Verify they fail** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify they pass** — `go test -race ./...` → PASS.
- [ ] **Step 5: Commit** — `git commit -m "serve embedded spa with link-preview meta tags"`

---

## Front-end

All front-end tasks run inside `web/` unless noted. Unit tests: `npx ng test --watch=false`. Build: `npm run build` (outputs to `../internal/web/dist`). End-to-end tests: `cd e2e && npx playwright test`.

### Task F1: Angular workspace and core

**Files:**
- Create: `web/` via `npx -y @angular/cli@22.3.0 new patchboard-web --directory web --style scss --routing --ssr=false --zoneless --skip-git --package-manager npm --prefix pb --ai-config none`
- Modify: `web/angular.json` (`outputPath: {"base": "../internal/web/dist", "browser": ""}`), `web/package.json` (`"build": "ng build && touch ../internal/web/dist/.gitkeep"`; the builder clears the output folder, so the script puts the tracked `.gitkeep` back), `web/src/index.html` (no `<title>`; `<!-- pb:meta -->` in `<head>`)
- Create: `web/proxy.conf.json` (`/api`, `/uploads`, `/r` → `http://localhost:8080`; `ng serve` uses it), `web/src/styles.scss`, `web/src/app/app.config.ts`, `web/src/app/app.routes.ts`, `web/src/app/app.ts`, `web/src/app/core/api.types.ts`, `web/src/app/core/api.ts`, `web/src/app/core/csrf.interceptor.ts`, `web/src/app/core/board.store.ts`, `web/src/app/core/me.store.ts`, `web/src/app/core/toast.ts`, `web/src/app/core/board-reuse.strategy.ts`, `web/src/app/core/route-utils.ts`, `web/src/app/core/i18n/{pt.ts,en.ts,i18n.ts,t.pipe.ts}`, `web/src/app/shared/avatar.ts`, `web/src/app/shared/status-chip.ts`, and specs `core/i18n/i18n.spec.ts`, `core/route-utils.spec.ts`, `shared/avatar.spec.ts`

**Interfaces:**
- Consumes: the JSON types and endpoints (Shared Contracts)
- Produces:
  - `Api` (injectable), one method per "TS method" in the Endpoints table, each returning `Observable<T>`, e.g. `listPosts(p: {sort?: 'trending'|'top'|'new'; status?: number; tag?: number; q?: string; offset?: number}): Observable<PostPage>`
  - `csrfInterceptor` adds `X-PB-CSRF: 1` to non-GET requests and maps HTTP errors to `ApiError` (`{status, code, fields}`)
  - `BoardStore` `{ board: Signal<Board | null>; load(): Promise<void>; status(id): Status | undefined; tag(id): Tag | undefined }`, loaded through `provideAppInitializer`; it applies `--pb-accent` on `document.documentElement`
  - `MeStore` `{ me: Signal<Me>; refresh(): Promise<void>; isAdmin: Signal<boolean> }`
  - `I18n` `{ lang: Signal<'pt'|'en'>; setLang(l): void; t(key: TKey, params?: Record<string,string|number>): string; plural(base: string, n: number): string; name(n: I18nName): string; date(iso: string, style?: 'short'|'long'): string }`. `detectLang(navigatorLang: string): 'pt'|'en'`. The choice is stored in `localStorage['pb_lang']` (try/catch), and `<html lang>` is set to `pt-BR` or `en`. `TPipe` (`t`, impure).
  - `Toast` `{ show(msg: string, kind?: 'info'|'error'): void; error(e: ApiError): void }` (`error.<code>` i18n keys)
  - `parseIdSlug(s: string): {id: number; slug: string} | null`; `postUrl(p: {id: number; slug: string}): string` → `/p/<id>-<slug>`
  - `BoardReuseStrategy` (`shouldReuseRoute` is true when both snapshots' `component === BoardShell`, otherwise the default)
  - `Avatar` component `pb-avatar` (inputs `seed: string`, `size = 24`): a deterministic 5×5 mirrored identicon SVG on a coloured circle. The colour comes from an 8-colour palette chosen by an FNV-1a hash of the seed. `avatarSvg(seed: string): string` is exported for tests.
  - `StatusChip` component `pb-status-chip` (input `statusId`): dot in the status colour + `i18n.name(status.name)`
  - Routes:
    - `''`, `'roadmap'`, `'updates'` and `'p/:idSlug'` → `BoardShell`, with `data.view` set to `list`, `roadmap`, `updates` and `post` respectively
    - `'admin/login'`, `'admin/invite/:token'`, `'admin'` → lazy admin routes (F7)
    - `'**'` → redirect to `''`
  - Placeholders for `BoardShell` and the admin routes are created here and filled in later tasks.
- Theme tokens on `:root`: `--pb-bg`, `--pb-surface`, `--pb-column`, `--pb-text`, `--pb-muted`, `--pb-border`, `--pb-accent`, `--pb-radius: 14px`. Dark values go under `@media (prefers-color-scheme: dark)`. Font: Poppins 400/500/600 from `@fontsource/poppins`.

Required copy (pt / en), used by later tasks:
`nav.list` Lista / List · `nav.roadmap` Roadmap / Roadmap · `nav.updates` Atualizações / Updates · `post.add` Adicionar post / Add a post · `common.empty` Nada aqui ainda / Nothing here yet · `post.newVotes_one` {n} voto novo esta semana / {n} new vote this week · `post.newVotes_other` {n} votos novos esta semana / {n} new votes this week · `comment.namePlaceholder` Seu nome (opcional) / Your name (optional) · `comment.submit` Comentar / Comment · `common.cancel` Cancelar / Cancel · `comment.reply` Responder / Reply · `comment.title` Comentários / Comments · `post.addedBy` Adicionado por {name} em {date} / Added by {name} on {date} · `author.anonymous` Anônimo / Anonymous · `author.team` Equipe / Team · `comment.placeholder` Adicione um comentário / Add a comment.

- [ ] **Step 1: Scaffold.** Run the `ng new` command above from the repo root, then `cd web && npm i @angular/cdk@22.3.0 @fontsource/poppins@5.3.0`. If `ng new` rejects the Node version, use Node 24 and note it in the README.

- [ ] **Step 2: Write the failing unit tests**

```ts
// i18n.spec.ts
it('translates in both languages', () => { i18n.setLang('pt'); expect(i18n.t('common.empty')).toBe('Nada aqui ainda');
  i18n.setLang('en'); expect(i18n.t('common.empty')).toBe('Nothing here yet'); });
it('interpolates and pluralises', () => { i18n.setLang('en'); expect(i18n.plural('post.newVotes', 4)).toBe('4 new votes this week');
  expect(i18n.plural('post.newVotes', 1)).toBe('1 new vote this week'); });
it('detects language', () => { expect(detectLang('pt-BR')).toBe('pt'); expect(detectLang('pt-PT')).toBe('pt');
  expect(detectLang('en-US')).toBe('en'); expect(detectLang('fr')).toBe('en'); });
it('formats dates per language', () => { i18n.setLang('pt'); expect(i18n.date('2026-11-25T12:00:00Z', 'long')).toBe('25 de nov. de 2026');
  i18n.setLang('en'); expect(i18n.date('2026-11-25T12:00:00Z', 'long')).toBe('Nov 25, 2026'); });
// route-utils.spec.ts
it('parses id-slug', () => { expect(parseIdSlug('12-aumentar-chat')).toEqual({id: 12, slug: 'aumentar-chat'});
  expect(parseIdSlug('12')).toEqual({id: 12, slug: ''}); expect(parseIdSlug('abc')).toBeNull(); });
// avatar.spec.ts
it('is deterministic', () => { expect(avatarSvg('k3j4')).toBe(avatarSvg('k3j4')); expect(avatarSvg('k3j4')).not.toBe(avatarSvg('zz99')); });
```

The `date('…','long')` expectations follow `Intl.DateTimeFormat` with `{day:'numeric', month:'short', year:'numeric'}`. If the runtime's ICU gives a different pt-BR string, assert the runtime value and record it in the test.

- [ ] **Step 3: Run them to verify they fail** — `npx ng test --watch=false` → FAIL.
- [ ] **Step 4: Implement** the core files listed above.
- [ ] **Step 5: Verify** — `npx ng test --watch=false` → PASS. `npm run build` → `../internal/web/dist/index.html` exists and contains `<!-- pb:meta -->`. `cd .. && go build ./... && go test ./internal/web/...` → PASS.
- [ ] **Step 6: Commit** — `git commit -m "scaffold angular app with api client, i18n, theme and avatar"` (do not commit `internal/web/dist` output)

---

### Task F2: Board shell, List view, voting, Playwright setup

**Files:**
- Create: `web/src/app/features/board/board-shell.{ts,html,scss}`, `web/src/app/features/board/list-view.{ts,html,scss}`, `web/src/app/shared/post-card.{ts,html,scss}`, `web/src/app/shared/vote-button.{ts,scss}`, `web/src/app/core/nav-state.ts`, `e2e/package.json`, `e2e/playwright.config.ts`, `e2e/global-setup.ts`, `e2e/helpers.ts`, `e2e/vote.spec.ts`
- Modify: `Makefile` (targets `web`, `build`, `e2e`)

**Interfaces:**
- Consumes: `Api`, `BoardStore`, `MeStore`, `I18n`, `Avatar`, `StatusChip`, `parseIdSlug`, `postUrl`, `BoardReuseStrategy`
- Produces:
  - `NavState` `{ lastView: Signal<'list'|'roadmap'|'updates'>; lastUrl: Signal<string> }`, updated by the shell on every non-post navigation
  - `BoardShell`:
    - header: logo, `i18n.name(board.title)`, description
    - tabs List/Roadmap/Updates as `routerLink`s
    - toolbar: a search toggle bound to the `q` query param (debounce 300 ms); a filter popover for status and tag bound to the `status`/`tag` query params; a `+` button that calls `openNewPost()` (a signal `newPostOpen`, used in F4); a profile-menu slot (F6)
    - renders `@switch(view)`: `list` → `pb-list-view`; `roadmap`/`updates` → placeholders until F5/F6; `post` → `NavState.lastView()`'s view plus `pb-post-modal` (F3)
  - `ListView`: sort tabs (Trending/Top/New → `sort` query param); loads `api.listPosts` from the query params; a "load more" button uses `nextOffset`; empty state `common.empty`
  - `PostCard` (`pb-post-card`, inputs `post: PostCard`, `showStatus = false`):
    - title, excerpt (2 lines clamped), tag chip, status chip when `showStatus`
    - `newVotes ≥ 2` → trend icon + `plural('post.newVotes', n)`
    - footer: vote button, comment count (when > 0), avatar stack of `voters`
    - `pinned` → a pin icon
    - Structure: `<article data-testid="post-card">`. The title is the only link (to `postUrl(post)`, `queryParamsHandling: 'preserve'`); a CSS `::after` overlay stretches it over the whole card. The vote button sits above the overlay (`position: relative; z-index: 1`). No button is nested inside a link.
  - `VoteButton` (`pb-vote-button`, inputs `postId`, `count`, `voted`, `size: 'sm'|'lg'`; output `changed: VoteState`): optimistic toggle, roll back + `toast.error` on failure, `MeStore.refresh()` after the first successful vote; filled dark style when `voted`; `aria-pressed`
  - e2e helpers (`e2e/helpers.ts`):
    - `createPost(request: APIRequestContext, title: string, body = ''): Promise<PostDetail>`
    - `adminApi(playwright: Playwright): Promise<APIRequestContext>` (a new request context, logged in as `e2e`)
    - `adminLogin(page: Page): Promise<void>`
  - `playwright.config.ts`:
    - `use: { baseURL: 'http://localhost:8099', locale: 'pt-BR', extraHTTPHeaders: { 'X-PB-CSRF': '1' } }`
    - It sets `process.env.PB_E2E_DATA ??= fs.mkdtempSync(os.tmpdir() + '/pb-e2e-')` at load time, so the main process and its workers share one data dir.
    - `webServer`: `command: 'make web && go run ./cmd/patchboard serve'`, `cwd: '..'`, `url: 'http://localhost:8099/healthz'`, `timeout: 240_000`, and env `PB_ADDR=:8099`, `PB_BASE_URL=http://localhost:8099`, `PB_DATA_DIR=$PB_E2E_DATA`.
    - `global-setup.ts` creates the admin `e2e` / `e2e-password-1` by running `go run ./cmd/patchboard admin create --username e2e --name "E2E Admin"` from `..`, with the password on stdin and the same env. The CLI migrates the database itself and SQLite tolerates the running server, so the order of `webServer` and `globalSetup` doesn't matter.

- [ ] **Step 1: Write the failing e2e test**

```ts
// e2e/vote.spec.ts
test('a player votes and unvotes from the list', async ({ page, request }) => {
  const post = await createPost(request, 'Aumentar tamanho do chat');
  await page.goto('/');
  const card = page.getByTestId('post-card').filter({ hasText: 'Aumentar tamanho do chat' });
  const vote = card.getByRole('button', { name: /vot/i });
  await expect(vote).toHaveText('0'); await vote.click();
  await expect(vote).toHaveText('1'); await expect(vote).toHaveAttribute('aria-pressed', 'true');
  await page.reload(); await expect(vote).toHaveText('1');
  await vote.click(); await expect(vote).toHaveText('0');
});
test('list sorts and searches', async ({ page, request }) => { /* two posts; ?q=chat shows only the chat post; Top/New tabs update the URL sort param */ });
```

The vote button's accessible name comes from i18n keys `post.vote` (Votar / Upvote) and `post.unvote` (Remover voto / Remove vote). Its visible text is just the count; the arrow icon is an `aria-hidden` SVG.

- [ ] **Step 2: Run it to verify it fails** — `cd e2e && npm i && npx playwright install chromium && npx playwright test vote.spec.ts` → FAIL.
- [ ] **Step 3: Implement** the components above, matching the reference look: soft grey page, white rounded cards with a subtle border, a pill vote button showing `^ n`, Poppins.
- [ ] **Step 4: Verify** — the e2e test → PASS; `npx ng test --watch=false` → PASS.
- [ ] **Step 5: Commit** — `git commit -m "add board shell, list view and voting with e2e setup"`

---

### Task F3: Post modal, comments, Markdown editor

**Files:**
- Create: `web/src/app/features/post/post-modal.{ts,html,scss}`, `web/src/app/features/post/comment-thread.{ts,html,scss}`, `web/src/app/features/post/comment-form.{ts,html,scss}`, `web/src/app/shared/markdown-editor/markdown-editor.{ts,html,scss}`, `web/src/app/shared/markdown-editor/format.ts`, `web/src/app/shared/markdown-editor/format.spec.ts`, `e2e/comment.spec.ts`
- Modify: `board-shell` (render the modal for `view === 'post'`)

**Interfaces:**
- Produces:
  - `applyFormat(text: string, start: number, end: number, kind: 'bold'|'italic'|'link'|'bullet'|'number'|'heading'|'code'): {text: string; start: number; end: number}`
  - `MarkdownEditor` (`pb-markdown-editor`: `value = model<string>('')`, inputs `placeholder`, `maxLength`, `rows = 4`): textarea + toolbar (B, I, link, bullets, numbers, heading, code) + Write/Preview tabs; Preview calls `api.preview` (shows `common.loading`, then sanitised HTML via `[innerHTML]`); a character counter shows when > 80 % of `maxLength`
  - `PostModal` (`pb-post-modal`, input `idSlug: string`):
    - opens a native `<dialog>` with `showModal()` and loads `api.post(id)`
    - `mergedInto` → `router.navigate([postUrl(mergedInto)], {replaceUrl: true})`; a slug mismatch → canonical URL with `replaceUrl`; 404 → toast `error.not_found` and close
    - header: big vote button (`size 'lg'`), title, `post.addedBy` with avatar, name + team badge for admins, and the date
    - chips: status chip, tag chip, comment count
    - body (`bodyHtml`), image grid (click opens the full image in a new tab)
    - team response block (highlighted, author + team badge)
    - share button (copies `location.origin + postUrl`, toast `post.linkCopied`)
    - edit/delete when `canEdit`/`canDelete`: edit opens inline title + `pb-markdown-editor`; delete asks via `confirm(i18n.t('post.confirmDelete'))`
    - Escape, the close button and a backdrop click navigate to `NavState.lastUrl()`
    - sets the document title to the post title
  - `CommentThread` (inputs `postId`, `comments: Comment[]`, `locked: boolean`; output `changed`): comments oldest first with replies indented; each has avatar, name (`author.anonymous` when null), team badge, short date, `bodyHtml`, a like button (heart + count), Reply (opens a nested `pb-comment-form` with the top-level `parentId`) and edit/delete when `canEdit`; deleted placeholders show `comment.deleted`
  - `CommentForm` (inputs `postId`, `parentId?`; outputs `posted: Comment`, `cancelled`):
    - `pb-markdown-editor` plus a footer with an avatar, a name input (`comment.namePlaceholder`, prefilled from `me.guest.name`, hidden for admins), Cancel and Comment (disabled while the body is empty or saving)
    - on submit, if the name changed → `api.setName` first, then `api.comment`, then `MeStore.refresh()`
    - a locked post shows `comment.locked` instead of the form (admins still see the form)

- [ ] **Step 1: Write the failing tests**

```ts
// format.spec.ts
it('bolds a selection', () => expect(applyFormat('hello', 0, 5, 'bold')).toEqual({text: '**hello**', start: 2, end: 7}));
it('bolds at the cursor', () => expect(applyFormat('ab', 1, 1, 'bold')).toEqual({text: 'a****b', start: 3, end: 3}));
it('bullets every selected line', () => expect(applyFormat('a\nb', 0, 3, 'bullet').text).toBe('- a\n- b'));
it('numbers lines', () => expect(applyFormat('a\nb', 0, 3, 'number').text).toBe('1. a\n2. b'));
it('links with the url selected', () => expect(applyFormat('site', 0, 4, 'link')).toEqual({text: '[site](https://)', start: 7, end: 15}));
```

```ts
// e2e/comment.spec.ts
test('comment, reply, like, close', async ({ page, request }) => {
  const post = await createPost(request, 'Aumentar tamanho do chat');
  await page.goto(`/p/${post.id}-wrong-slug`);
  await expect(page).toHaveURL(new RegExp(`/p/${post.id}-aumentar-tamanho-do-chat$`));
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('heading', { name: 'Aumentar tamanho do chat' })).toBeVisible();
  await dialog.getByPlaceholder('Adicione um comentário').fill('**pls**, meu olhos agradecem');
  await dialog.getByPlaceholder('Seu nome (opcional)').fill('WuzeraProMAX');
  await dialog.getByRole('button', { name: 'Comentar' }).click();
  await expect(dialog.locator('strong', { hasText: 'pls' })).toBeVisible();
  await expect(dialog.getByText('WuzeraProMAX')).toBeVisible();
  await dialog.getByRole('button', { name: 'Responder' }).click();
  /* fill the reply, submit → reply visible inside the first comment's replies; like → count 1 */
  await page.keyboard.press('Escape'); await expect(page).toHaveURL(/\/$/);
});
```

The e2e browser context uses `locale: 'pt-BR'` (set in `playwright.config.ts`), so Portuguese copy applies.

- [ ] **Step 2: Verify they fail** — unit tests and `npx playwright test comment.spec.ts` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify they pass** — PASS (and `vote.spec.ts` still passes).
- [ ] **Step 5: Commit** — `git commit -m "add post modal, comments and markdown editor"`

---

### Task F4: New post dialog, similar hints, image picker

**Files:**
- Create: `web/src/app/features/post/new-post-dialog.{ts,html,scss}`, `web/src/app/shared/image-picker.{ts,html,scss}`, `e2e/post.spec.ts`, `e2e/fixtures/shot.png` (a small PNG, e.g. 320×200)
- Modify: `board-shell` (renders the dialog when `newPostOpen()`), `list-view` (an `post.add` button above the list)

**Interfaces:**
- Produces:
  - `NewPostDialog` (`pb-new-post-dialog`; output `closed`):
    - title input (3–120 counter); below it, while ≥ 3 characters, `api.similar` (debounce 300 ms) shows up to 5 suggestions (`post.similarTitle`: "Parecidos — vote em vez de duplicar" / "Similar — vote instead of duplicating"), each linking to its post
    - `pb-markdown-editor` (max 10000)
    - tag select (optional, `i18n.name`)
    - `pb-image-picker` (hidden when `!board.uploadsEnabled && !isAdmin`)
    - name input for guests
    - submit → `api.createPost`; on success close and navigate to `postUrl`; when `pending`, show `toast(post.pendingNotice)`
    - 422 → field errors from `error.fields` via `validation.<reason>` keys
  - `ImagePicker` (`pb-image-picker`, `images = model<Image[]>([])`): file input (accept `image/*`, multiple); uploads each file at once via `api.upload` with a progress spinner; thumbnails with remove buttons; max 4 (the add button is hidden at 4); per-file errors use `error.<code>`

- [ ] **Step 1: Write the failing e2e test**

```ts
test('create a post with a tag and an image, seeing similar posts first', async ({ page, request }) => {
  await createPost(request, 'Aumentar tamanho do chat');
  await page.goto('/');
  await page.getByRole('button', { name: 'Adicionar post' }).first().click();
  const d = page.getByRole('dialog');
  await d.getByLabel('Título').fill('tamanho do chat');
  await expect(d.getByRole('link', { name: /Aumentar tamanho do chat/ })).toBeVisible();
  await d.getByLabel('Título').fill('Filtro de Level no market');
  await d.getByLabel('Tag').selectOption({ label: 'Melhoria' });
  await d.locator('input[type=file]').setInputFiles('fixtures/shot.png');
  await expect(d.getByRole('img', { name: /anexo|attachment/i })).toBeVisible();
  await d.getByRole('button', { name: 'Publicar' }).click();
  await expect(page).toHaveURL(/\/p\/\d+-filtro-de-level-no-market$/);
  await expect(page.getByRole('dialog').getByRole('img')).toBeVisible();
});
```

New copy: `post.titleLabel` Título / Title · `post.tagLabel` Tag / Tag · `post.submit` Publicar / Publish · `post.imageAlt` Anexo {n} / Attachment {n} · `post.pendingNotice` Seu post aguarda aprovação da equipe / Your post is waiting for team approval.

- [ ] **Step 2: Verify it fails** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify it passes** — all e2e tests PASS.
- [ ] **Step 5: Commit** — `git commit -m "add new post dialog with similar hints and image uploads"`

---

### Task F5: Roadmap view

**Files:**
- Create: `web/src/app/features/board/roadmap-view.{ts,html,scss}`, `e2e/roadmap.spec.ts`
- Modify: `board-shell`

**Interfaces:**
- Produces: `RoadmapView` (`pb-roadmap-view`): loads `api.roadmap({tag, q})` from the query params. It shows a horizontally scrolling row of columns (min-width 260 px, `scroll-snap` on narrow screens). Each column header has a status dot, the name (`i18n.name`, ellipsis), and `total`. Cards are `pb-post-card`. An empty column shows `common.empty`. The default-status column ends with a dashed `post.add` button that opens the new-post dialog. When `total > posts.length`, the column shows `roadmap.more` ("+{n} mais" / "+{n} more"). Column drag & drop for admins is added in F8; here the columns are wrapped in `cdkDropListGroup` with `cdkDropListDisabled = !isAdmin`, and no drop handler yet.

- [ ] **Step 1: Write the failing e2e test**

```ts
test('roadmap groups posts by status and follows the language', async ({ page, request, playwright }) => {
  const admin = await adminApi(playwright);
  const p = await createPost(request, 'Transferência de tiers');
  await admin.post(`/api/admin/posts/${p.id}/status`, { data: { statusId: 3 } });
  await page.goto('/roadmap');
  const col = page.getByRole('region', { name: /Planejado/ });
  await expect(col.getByText('Transferência de tiers')).toBeVisible();
  await expect(col.getByTestId('column-count')).toHaveText('1');
  await page.evaluate(() => localStorage.setItem('pb_lang', 'en')); await page.reload();
  await expect(page.getByRole('region', { name: /Planned/ })).toBeVisible();
});
```

Each column is a `<section role="region" [attr.aria-label]="statusName">`.

- [ ] **Step 2: Verify it fails** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify it passes** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add roadmap view"`

---

### Task F6: Updates view and profile menu

**Files:**
- Create: `web/src/app/features/board/updates-view.{ts,html,scss}`, `web/src/app/features/profile/profile-menu.{ts,html,scss}`, `e2e/profile.spec.ts`
- Modify: `board-shell` (updates view, the profile menu in the toolbar, the `?recovered=` toast on load)

**Interfaces:**
- Produces:
  - `UpdatesView`: cards with a long date, title, `bodyHtml`, and linked posts as chips (status dot + status name + post title, each a link to `postUrl`); "load more"; empty state `updates.empty`. For admins: an `updates.new` button and per-card edit/delete hooks (dialog in F8).
  - `ProfileMenu` (avatar button in the toolbar → popover):
    - Guests: a name field + save (`api.setName`); the recovery link with a copy button and the warning `profile.recoveryWarning` ("Quem tiver este link vira você. Não compartilhe." / "Anyone with this link becomes you. Don't share it."); when `me.guest` is null, `profile.noIdentity` instead.
    - Admins: `admin.panel` link to `/admin`, and logout.
    - Everyone: language switcher PT | EN (`I18n.setLang`).
  - The shell reads `?recovered=1|0` once and shows `profile.recovered` or `profile.recoverFailed`, then removes the param (`replaceUrl`).

- [ ] **Step 1: Write the failing e2e test**

```ts
test('name persists and the recovery link restores identity in another browser', async ({ page, browser }) => {
  await page.goto('/');
  await page.getByRole('button', { name: /perfil|profile/i }).click();
  await page.getByLabel('Seu nome').fill('WuzeraProMAX'); await page.getByRole('button', { name: 'Salvar' }).click();
  await page.reload(); await page.getByRole('button', { name: /perfil|profile/i }).click();
  await expect(page.getByLabel('Seu nome')).toHaveValue('WuzeraProMAX');
  const link = await page.getByTestId('recovery-link').inputValue();
  const other = await (await browser.newContext({ locale: 'pt-BR' })).newPage();
  await other.goto(link);
  await expect(other.getByText('Identidade restaurada')).toBeVisible();
  await other.getByRole('button', { name: /perfil|profile/i }).click();
  await expect(other.getByLabel('Seu nome')).toHaveValue('WuzeraProMAX');
});
test('updates list shows linked post status', async ({ page, request }) => { /* admin creates an update via the API linking a post in status 3 → /updates shows the title and a chip "Planejado" + the post title */ });
```

Copy: `profile.open` Perfil / Profile · `profile.nameLabel` Seu nome / Your name · `common.save` Salvar / Save · `profile.recovered` Identidade restaurada / Identity restored.

- [ ] **Step 2: Verify it fails** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify it passes** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add updates view and profile menu"`

---

### Task F7: Admin login, invites and settings pages

**Files:**
- Create: `web/src/app/features/admin/admin.routes.ts`, `admin-guard.ts`, `admin-login.{ts,html}`, `invite-accept.{ts,html}`, `admin-shell.{ts,html,scss}`, `board-settings.{ts,html}`, `statuses-page.{ts,html}`, `tags-page.{ts,html}`, `team-page.{ts,html}` (all under `web/src/app/features/admin/`), `e2e/admin.spec.ts`

**Interfaces:**
- Produces:
  - `adminGuard`: `MeStore.isAdmin()`, or redirect to `/admin/login`
  - `AdminLogin`: username + password → `api.adminLogin` → `/admin`; `invalid_credentials` / `rate_limited` messages
  - `InviteAccept`: checks the token (invalid → `admin.inviteInvalid`); a form for username, display name, password (≥ 10) → `/admin`
  - `AdminShell`: a side nav (Board, Statuses, Tags, Team, Moderation) and a back-to-board link
  - `BoardSettings`: titles pt/en, descriptions pt/en, team name, accent (`<input type=color>` + hex text), logo upload (`pb-image-picker` limited to 1), require-approval and uploads-enabled toggles → `api.saveSettings`, then `BoardStore.load()`
  - `StatusesPage`: a cdk drag-to-reorder list (→ `orderStatuses`); inline edit of pt/en name, colour, roadmap toggle and the default radio; add; delete (asks for a target status in a select when the API answers `move_required`)
  - `TagsPage`: the same pattern without colour/default
  - `TeamPage`: a member list with disable; an invite button showing the URL in a read-only input with copy and the expiry date; an own-profile form (display name, change password with the current password)

- [ ] **Step 1: Write the failing e2e test**

```ts
test('admin logs in, renames the board and creates an invite', async ({ page }) => {
  await page.goto('/admin/login');
  await page.getByLabel('Usuário').fill('e2e'); await page.getByLabel('Senha').fill('e2e-password-1');
  await page.getByRole('button', { name: 'Entrar' }).click();
  await expect(page).toHaveURL(/\/admin\/settings$/);
  await page.getByLabel('Título (PT)').fill('Venore Survivors • Comunidade');
  await page.getByRole('button', { name: 'Salvar' }).click();
  await page.goto('/'); await expect(page.getByRole('heading', { level: 1 })).toHaveText('Venore Survivors • Comunidade');
  await page.goto('/admin/team'); await page.getByRole('button', { name: 'Convidar' }).click();
  await expect(page.getByTestId('invite-url')).toHaveValue(/\/admin\/invite\//);
});
test('wrong password shows an error', async ({ page }) => { /* → text from error.invalid_credentials */ });
```

Copy: `admin.username` Usuário / Username · `admin.password` Senha / Password · `admin.login` Entrar / Sign in · `admin.titlePt` Título (PT) / Title (PT) · `admin.titleEn` Título (EN) / Title (EN) · `admin.invite` Convidar / Invite · `error.invalid_credentials` Usuário ou senha incorretos / Wrong username or password.

- [ ] **Step 2: Verify it fails** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify it passes** — PASS.
- [ ] **Step 5: Commit** — `git commit -m "add admin login, invites and settings pages"`

---

### Task F8: Admin moderation on the board

**Files:**
- Create: `web/src/app/features/admin/status-change-dialog.{ts,html}`, `update-editor-dialog.{ts,html}`, `vote-audit-dialog.{ts,html}`, `merge-dialog.{ts,html}`, `moderation-page.{ts,html}` (under `features/admin/`), `web/src/app/features/post/post-admin-bar.{ts,html}`, `e2e/status.spec.ts`
- Modify: `post-modal` (renders `pb-post-admin-bar` for admins, plus image delete and comment delete/restore for admins), `roadmap-view` (drop handler), `updates-view` (admin edit/delete/new), `admin.routes.ts` (moderation page)

**Interfaces:**
- Produces:
  - `StatusChangeDialog` (inputs `post: PostCard`, `toStatusId: number`; outputs `done: PostDetail`, `cancelled`):
    - a status select, a team-response `pb-markdown-editor` (prefilled with the existing `response.md`)
    - a "publish as Update" checkbox; when ticked it shows a title input prefilled with `i18n.t('status.updateTitle', {post: post.title, status: statusName})` ("{post}: {status}") and a body prefilled with the response text
    - → `api.changeStatus`
  - `PostAdminBar`: a status select (opens `StatusChangeDialog`), pin/unpin, lock/unlock, merge (`MergeDialog`: search via `api.listPosts({q})` and pick a target → `api.merge` → navigate to the target), delete/restore, and Votes (`VoteAuditDialog`: voters with flags as chips, remove vote, and ban guest with an "also block IP" checkbox, checked by default, with a note about shared mobile IPs)
  - Roadmap drop: dropping a card in another column opens `StatusChangeDialog` with `toStatusId` = that column; confirming moves the card (reload the roadmap); cancelling puts it back
  - `UpdateEditorDialog` (input `update?: UpdateItem`): title, body editor, a linked-posts picker (search → chips, max 10) → create/edit
  - `ModerationPage`: tabs Pending (approve / delete), Trash (restore posts and comments), Bans (unban)

- [ ] **Step 1: Write the failing e2e test**

```ts
test('admin changes a status with a response and publishes an update', async ({ page, request }) => {
  const p = await createPost(request, 'Aumentar tamanho do chat');
  await adminLogin(page);
  await page.goto(`/p/${p.id}-${p.slug}`);
  const d = page.getByRole('dialog');
  await d.getByLabel('Status').selectOption({ label: 'Planejado' });
  const s = page.getByRole('dialog', { name: 'Alterar status' });
  await s.getByPlaceholder('Resposta da equipe').fill('Vamos fazer!');
  await s.getByLabel('Publicar como atualização').check();
  await expect(s.getByLabel('Título da atualização')).toHaveValue('Aumentar tamanho do chat: Planejado');
  await s.getByRole('button', { name: 'Confirmar' }).click();
  await expect(d.getByText('Vamos fazer!')).toBeVisible();
  await expect(d.getByTestId('status-chip')).toHaveText(/Planejado/);
  await page.goto('/updates');
  await expect(page.getByText('Aumentar tamanho do chat: Planejado')).toBeVisible();
});
test('admin drags a roadmap card to another column', async ({ page, request }) => {
  /* post in Sugestões; drag the card onto the "Em análise" region (mouse down/move in steps/up)
     → the status dialog shows "Em análise" preselected → Confirmar → the card is inside the "Em análise" region */
});
```

Copy: `status.label` Status / Status · `status.dialogTitle` Alterar status / Change status (the `<dialog>`'s `aria-label`) · `status.responsePlaceholder` Resposta da equipe / Team response · `status.publishUpdate` Publicar como atualização / Publish as update · `status.updateTitleLabel` Título da atualização / Update title · `status.updateTitle` {post}: {status} · `common.confirm` Confirmar / Confirm.

- [ ] **Step 2: Verify it fails** — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Verify it passes** — all e2e tests PASS.
- [ ] **Step 5: Commit** — `git commit -m "add admin moderation tools on the board"`

---

### Task F9: Docker image, Makefile, README

**Files:**
- Create: `Dockerfile`, `.dockerignore`
- Modify: `Makefile`, `README.md`

**Interfaces:**
- Produces:
  - `Dockerfile` with three stages:
    1. `node:26-alpine` (or the Node major verified in F1): `npm ci && npm run build` in `web/`.
    2. `golang:1.27-alpine`: copies the built `internal/web/dist`, runs `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/patchboard ./cmd/patchboard`, and runs `mkdir -p /out/data`.
    3. `gcr.io/distroless/static-debian12:nonroot`: `COPY --from=go --chown=65532:65532 /out/data /data`; the binary goes to `/patchboard`. `ENV PB_ADDR=:8080 PB_DATA_DIR=/data`, `EXPOSE 8080`, `VOLUME /data`, `ENTRYPOINT ["/patchboard"]`, `CMD ["serve"]`.
  - Makefile targets: `web`, `sqlc`, `build` (web + go build → `bin/patchboard`), `test` (go test + ng test), `e2e`, `docker` (`docker build -t patchboard .`)
  - README sections:
    - what PatchBoard is
    - running locally (`make run` + `cd web && npx ng serve`)
    - environment variables (`PB_BASE_URL` required, `PB_ADDR`, `PB_DATA_DIR`, `PB_TRUSTED_PROXIES`, `PB_SECRET`)
    - creating the first admin (`docker run --rm -it -v pbdata:/data -e PB_BASE_URL=… patchboard admin create --username … --name …`)
    - reverse-proxy note: set `PB_TRUSTED_PROXIES` to the proxy's address, or every player shares one rate limit
    - backups: use `sqlite3 /data/patchboard.db ".backup '/backup/pb.db'"` (never `cp` of the live file) and also copy `/data/uploads` and `/data/secret`
    - limitations (guest votes, CGNAT bans)

- [ ] **Step 1: Write the check script**

```bash
# verify image end-to-end (manual check, run from repo root)
docker build -t patchboard . \
 && docker run -d --name pbtest -p 8098:8080 -e PB_BASE_URL=http://localhost:8098 -v pbtest:/data patchboard \
 && sleep 2 && curl -fsS localhost:8098/healthz \
 && curl -fsS localhost:8098/ | grep -q 'og:title' \
 && printf 'docker-pass-1\n' | docker exec -i pbtest /patchboard admin create --username ops --name Ops \
 ; docker rm -f pbtest; docker volume rm pbtest
```

- [ ] **Step 2: Run it to verify it fails** — FAIL (no Dockerfile yet).
- [ ] **Step 3: Implement** the files above.
- [ ] **Step 4: Verify** — the script prints `ok` and exits 0. Also run `make test` and `make e2e` → all PASS.
- [ ] **Step 5: Commit** — `git commit -m "add docker image, makefile targets and readme"`

---

## Spec coverage check (done while writing)

| Spec section | Tasks |
|---|---|
| Board (single, settings, three views) | B6, F1, F2, F5, F6, F7 |
| Statuses & tags | B1 (seed), B6, F7 |
| Posts (fields, similar, votes, card details, URL + previews) | B7, B8, B9, B10, B12, B16, F2, F3, F4 |
| Comments | B11, F3 |
| List / trending / search / roadmap | B9, B10, F2, F5 |
| Guests, names, avatars, recovery, edit window | B4, B7, F1, F6 |
| Protection (rate limits, IP hash, audit, bans) | B2, B4, B15, F8 |
| Team (accounts, CLI, invites, badge, disable) | B5, F7 |
| Moderation (approval, uploads toggle, edit/hide/restore, image delete, merge, ban, pin, lock, roadmap drag) | B6, B12, B14, B15, F8 |
| Status changes & Updates | B13, F6, F8 |
| Language | F1 (+ every UI task), B6 (bilingual names), B16 (meta language) |
| Look | F1, F2 |
| Markdown | B3, F3 |
| Tech / hosting / ops | B1, B2, B16, F9 |
| Testing | every task; e2e for post (F4), vote (F2), comment (F3), status change (F8) |
