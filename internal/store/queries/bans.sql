-- name: CountBansFor :one
SELECT count(*) FROM bans
WHERE (sqlc.narg('guest_id') IS NOT NULL AND guest_id = sqlc.narg('guest_id'))
   OR (sqlc.narg('ip_hash') IS NOT NULL AND ip_hash = sqlc.narg('ip_hash'));
