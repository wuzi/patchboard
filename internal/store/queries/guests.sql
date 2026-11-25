-- name: GetGuestByTokenHash :one
SELECT * FROM guests WHERE token_hash = ?;

-- name: CreateGuest :exec
INSERT INTO guests (id, token_hash, name, created_at, created_ip_hash, last_ip_hash)
VALUES (?, ?, NULL, ?, ?, ?);

-- name: TouchGuestIP :exec
UPDATE guests SET last_ip_hash = ? WHERE id = ?;

-- name: SetGuestName :exec
UPDATE guests SET name = ? WHERE id = ?;
