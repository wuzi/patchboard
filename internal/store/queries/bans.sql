-- name: CountBansFor :one
SELECT count(*) FROM bans
WHERE (sqlc.narg('guest_id') IS NOT NULL AND guest_id = sqlc.narg('guest_id'))
   OR (sqlc.narg('ip_hash') IS NOT NULL AND ip_hash = sqlc.narg('ip_hash'));

-- name: GetGuest :one
SELECT * FROM guests WHERE id = ?;

-- name: InsertBan :one
INSERT INTO bans (guest_id, ip_hash, reason, created_by, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id;

-- name: ListBans :many
SELECT b.id, b.guest_id, g.name AS guest_name, b.ip_hash, b.reason, b.created_at
FROM bans b LEFT JOIN guests g ON g.id = b.guest_id ORDER BY b.created_at DESC, b.id DESC;

-- name: GetBan :one
SELECT * FROM bans WHERE id = ?;

-- name: DeleteBan :exec
DELETE FROM bans WHERE id = ?;
