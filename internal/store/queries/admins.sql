-- name: InsertAdmin :one
INSERT INTO admins (username, display_name, password_hash, created_at) VALUES (?, ?, ?, ?) RETURNING id;

-- name: GetAdminByUsername :one
SELECT * FROM admins WHERE username = ?;

-- name: GetAdmin :one
SELECT * FROM admins WHERE id = ?;

-- name: ListAdmins :many
SELECT * FROM admins ORDER BY id;

-- name: SetAdminPassword :exec
UPDATE admins SET password_hash = ? WHERE id = ?;

-- name: SetAdminDisplayName :exec
UPDATE admins SET display_name = ? WHERE id = ?;

-- name: DisableAdmin :exec
UPDATE admins SET disabled_at = ? WHERE id = ?;

-- name: CreateSession :exec
INSERT INTO admin_sessions (token_hash, admin_id, created_at, expires_at) VALUES (?, ?, ?, ?);

-- name: GetSessionAdmin :one
SELECT a.id, a.username, a.display_name
FROM admin_sessions s JOIN admins a ON a.id = s.admin_id
WHERE s.token_hash = ? AND s.expires_at > ? AND a.disabled_at IS NULL;

-- name: DeleteSession :exec
DELETE FROM admin_sessions WHERE token_hash = ?;

-- name: DeleteAdminSessions :exec
DELETE FROM admin_sessions WHERE admin_id = ?;

-- name: CreateInvite :exec
INSERT INTO admin_invites (token_hash, created_by, created_at, expires_at) VALUES (?, ?, ?, ?);

-- name: GetInvite :one
SELECT * FROM admin_invites WHERE token_hash = ?;

-- name: UseInvite :exec
UPDATE admin_invites SET used_at = ?, used_by = ? WHERE token_hash = ?;
