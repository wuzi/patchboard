# PatchBoard v1 — Specification

Agreed 2026-11-25. PatchBoard is a self-hosted feedback board where a game's community posts ideas and bug reports, votes on them, comments, and follows the team's roadmap and announcements. The first deployment is for Venore Survivors, but nothing in the code is specific to that game.

## Board

- One board per deployment. Admins set the board title and description (Portuguese and English versions of each), the team name, an accent colour and a logo.
- Bugs and ideas share the board and are told apart by tags.
- Three views: **List**, **Roadmap**, **Updates**.

## Statuses and tags

- Stored in the database, each with a Portuguese and an English name; admins can create, rename, recolour, reorder and delete them.
- Admins choose which statuses appear on the Roadmap. Exactly one status is the default; new posts land there.
- Seeded on first run:

| Position | Portuguese | English | Colour |
|---|---|---|---|
| 0 (default) | Sugestões | Suggestions | `#9CA3AF` |
| 1 | Em análise | Under review | `#7C3AED` |
| 2 | Planejado | Planned | `#2563EB` |
| 3 | Em desenvolvimento | In progress | `#F59E0B` |
| 4 | Concluído | Done | `#16A34A` |
| 5 | Recusado | Declined | `#DC2626` |

- Seeded tags: Feature / Feature, Melhoria / Improvement, Bug / Bug.
- Deleting a status requires choosing another status to move its posts to; the default status cannot be deleted. Deleting a tag removes it from its posts.

## Posts

- Title (required), Markdown description (optional), one optional tag picked by the player (admins can change it), up to 4 images.
- While the player types a title, up to 5 similar existing posts are suggested so they can vote instead of duplicating.
- Upvotes only. A player can take back their vote. The author gets no automatic vote.
- Cards show: title, excerpt, tag, vote count, comment count, up to 3 most recent voters' avatars, and "N new votes this week" when N ≥ 2 (votes in the last 7 days).
- Each post has its own URL, `/p/<id>-<slug>`, which opens as a modal over the current view. Shared links show the post's title, excerpt and first image as a link preview (Discord, etc.).

## Comments

- One level of replies (a reply to a reply attaches to the top-level comment). Oldest first.
- Players can like comments.
- A deleted comment that has replies shows as a "deleted" placeholder; otherwise it disappears.

## List view, trending and search

- Sorts: **Trending** (default: most votes in the last 7 days, ties broken by total votes), **Top** (total votes), **New**.
- Pinned posts stay at the top of the List view.
- Search ignores accents and case (`análise` = `analise`) and matches word prefixes. Filters by status and tag.
- Roadmap: one column per roadmap status, cards sorted by votes, with a count per column. Search and the tag filter apply.

## Players (guests)

- No accounts. A guest identity is a random token in a cookie, created the first time the player does something (vote, post, comment, like, set a name).
- Optional saved name (max 40 characters), editable anytime; blank shows as "Anônimo" / "Anonymous". Names are not unique.
- Avatars are generated from the guest's public id, with no outside service.
- A **recovery link** in the profile menu restores the identity in another browser or device. Anyone the link is shared with becomes that guest.
- Players can edit or delete their post while it is still in the default status, and their comments at any time.

## Protection

- Per-IP rate limits on creating identities, posting, commenting, voting, liking and uploading. Admins are exempt (except login).
- IP addresses are never stored raw: only a keyed hash (IPv6 grouped by /64).
- Admins can review a post's votes (flagging voters who share an IP hash or whose identity is brand new), remove votes, and ban a guest (optionally also their IP hash).
- Known limitation, accepted: a player using private windows can still vote more than once; banning an IP hash can block other players behind the same mobile-carrier IP.

## Team (admins)

- Each teammate has an admin account (username + password, argon2id hashing, login attempts limited, sessions expire after 30 days). No 2FA in v1.
- The first admin is created with `patchboard admin create`; passwords can be reset with `patchboard admin reset-password`. Admins invite teammates with one-time links (valid 7 days) that they copy and send themselves.
- Teammates appear under their own display name with a team badge.
- Admins can disable teammates (not themselves, and never the last active admin).

## Moderation

- Posts appear immediately. An admin setting turns on an approval queue (pending posts are visible only to their author and admins). Another setting turns off player image uploads.
- Admins can edit, hide (soft delete) and restore any post or comment; delete any image (removed from disk immediately); merge duplicates (votes and comments move to the kept post, the duplicate redirects); ban guests; pin posts; lock comments.
- Roadmap: admins drag a card to another column to change its status.

## Status changes and Updates

- When an admin changes a post's status they can add a **team response** shown on the post (e.g. why it was declined) and tick **publish as Update** with prefilled text.
- Updates are dated announcements (title + Markdown body) that link to any number of posts; each link shows the post's status at the time it was linked. Admins can write, edit and delete Updates on their own too.
- Updates are written in a single language (as written).

## Language

- Interface in Brazilian Portuguese and English. The browser's language picks the default (`pt*` → Portuguese, anything else → English); a switcher changes and remembers it.
- Status and tag names, and the board title/description, have both language versions. Posts, comments and Updates show as written.

## Look

- The clean light look of the reference screenshots (rounded cards, soft grey columns, coloured status dots, Poppins), plus an automatic dark mode and the board's accent colour.

## Markdown

- Full Markdown (headings, lists, tables, quotes, code, links, strikethrough), with a toolbar (bold, italic, link, lists, heading, code) and a preview tab.
- No raw HTML and no images by URL (images go through attachments). Links open in a new tab with `rel="nofollow noopener"`. The server renders and sanitises; the preview uses the same renderer.

## Tech

- Go 1.27: standard `net/http` router, `modernc.org/sqlite` (pure Go, WAL mode), `sqlc` for queries, SQL migrations built into the binary, JSON REST API.
- Angular (latest stable) compiled to static files and embedded in the Go binary; Go injects link-preview meta tags.
- Single binary, packaged as a Docker image.

## Hosting and ops

- The operator runs the container on a VPS and provides the reverse proxy, HTTPS and backups.
- PatchBoard provides: a trusted-proxy setting (so rate limits see players' real IPs), a `/healthz` endpoint, and one data folder holding the database and uploaded images.
- Backups must copy the database with `sqlite3 .backup` (or `VACUUM INTO`), not `cp` of the live file, and must include the uploads folder.

## Testing

- Go tests for the API and its rules, against a real SQLite database.
- Playwright browser tests for the main flows: post, vote, comment, change status.

## Limits and defaults (chosen during planning; adjustable)

| Item | Value |
|---|---|
| Post title | 3–120 characters |
| Post / Update body | ≤ 10,000 characters |
| Comment | 1–5,000 characters |
| Team response | ≤ 5,000 characters |
| Guest name / admin display name | ≤ 40 characters |
| Status / tag name (each language) | 1–30 characters |
| Board title / description | 1–80 / ≤ 500 characters |
| Images | ≤ 4 per post, ≤ 5 MiB each, ≤ 40 megapixels; resized to ≤ 2560 px on the longest side; metadata stripped |
| Unattached uploads | deleted after 24 hours |
| Updates | ≤ 10 linked posts each |
| Rate limits per IP | 20 new identities/h, 5 posts/h, 20 comments/h, 120 votes/h, 120 likes/h, 20 uploads/h, 120 previews/min |
| Admin login | 5 attempts per IP and 10 per username per 15 min |
| Admin password | ≥ 10 characters |
| Admin invite | 7 days, single use |
| List page size | 30 (max 50) |
| Roadmap column | first 100 cards plus total count |

## Not in v1

Notifications (email and the bell icon), importing the old board, a backup command, 2FA, multiple boards.
