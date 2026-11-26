**Archived in favor of [kiska](https://github.com/wuzi/kiska)**

# PatchBoard

A self-hosted feedback board for game communities. Players post ideas and bug reports, vote, comment and follow the roadmap. The team answers, moves posts through statuses and publishes updates.

- **Views:** List (Trending / Top / New), Roadmap (one column per status), Updates (announcements linked to posts).
- **Players need no account:** a guest identity is created on their first vote, post or comment. They can set a name and restore their identity on another device with a recovery link.
- **Team tools:** admin accounts with invites, status changes with team responses, pin, lock, merge duplicates, approval queue, bans, vote audit, trash and restore.
- **Portuguese and English** interface, automatic dark mode, link previews when posts are shared (Discord, etc.).
- **One binary:** Go + SQLite, with the Angular front-end embedded. No external services.

The full specification is in [docs/spec.md](docs/spec.md).

## Running with Docker

```bash
docker build -t patchboard .
docker volume create pbdata
docker run -d --name patchboard -p 8080:8080 \
  -e PB_BASE_URL=https://feedback.example.com \
  -e PB_TRUSTED_PROXIES=172.17.0.1 \
  -v pbdata:/data patchboard
```

Create the first admin (it asks for the password twice):

```bash
docker exec -it patchboard /patchboard admin create --username wuzi --name "Equipe Venore Survivors"
```

Then sign in at `https://feedback.example.com/admin`. Invite teammates from **Admin → Team**. Invite links last 7 days and work once.

A forgotten password can be reset from the server:

```bash
docker exec -it patchboard /patchboard admin reset-password --username wuzi
```

## Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `PB_BASE_URL` | yes | – | Public URL, e.g. `https://feedback.example.com`. Used for recovery links, invites and link previews. With `https://`, cookies are marked `Secure`. |
| `PB_ADDR` | no | `:8080` | Listen address. |
| `PB_DATA_DIR` | no | `./data` (`/data` in Docker) | Folder holding the database, uploaded images and the secret. |
| `PB_TRUSTED_PROXIES` | no | – | Comma-separated IPs/CIDRs of your reverse proxy. **Set this when running behind a proxy.** Without it, every player appears to come from the proxy's IP, so they all share one rate limit. |
| `PB_SECRET` | no | generated | Key for hashing IP addresses (≥ 32 characters). If unset, a random one is created in `PB_DATA_DIR/secret` and reused. |

PatchBoard serves plain HTTP. Put it behind your own reverse proxy (Caddy, nginx, Traefik…) for HTTPS. `GET /healthz` returns `ok` when the database is reachable.

## Backups

Everything lives in `PB_DATA_DIR`:

- `patchboard.db` (plus `-wal`/`-shm` while running): the database
- `uploads/`: images
- `secret`: the IP-hash key (keep it, or existing bans by IP stop matching)

**Do not `cp` the live database file.** A copy taken while the server writes can be corrupt. Use SQLite's backup instead, for example from the host:

```bash
sqlite3 /var/lib/docker/volumes/pbdata/_data/patchboard.db ".backup '/backups/patchboard-$(date +%F).db'"
rsync -a /var/lib/docker/volumes/pbdata/_data/uploads/ /backups/uploads/
cp /var/lib/docker/volumes/pbdata/_data/secret /backups/
```

## Development

Requirements: Go 1.27, Node 26 (or 24), npm.

```bash
make run                 # API on :8080 (PB_BASE_URL points at the Angular dev server)
cd web && npx ng serve   # front-end on :4200, proxies /api, /uploads and /r to :8080
```

| Command | What it does |
|---|---|
| `make test` | Go tests (real SQLite) and Angular unit tests |
| `make e2e` | Builds the front-end, starts a throwaway server and runs the Playwright tests |
| `make build` | Builds `bin/patchboard` with the front-end embedded |
| `make sqlc` | Regenerates `internal/store/gen` after editing `internal/store/queries/*.sql` |
| `make docker` | Builds the Docker image |

Layout: `cmd/patchboard` (CLI), `internal/*` (one package per area: `auth`, `board`, `posts`, `comments`, `uploads`, `updates`, `web`…), `web/` (Angular), `e2e/` (Playwright).

## Known limitations

- Guests are identified by a cookie, so a player using private windows can vote more than once. Rate limits and the admin vote audit (shared-IP and brand-new-identity flags) make this visible but can't prevent it.
- Banning a guest's IP can also block other players on the same mobile-carrier IP (CGNAT).
- No notifications (email or in-app) in v1.
