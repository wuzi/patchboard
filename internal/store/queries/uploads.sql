-- name: GetUpload :one
SELECT * FROM uploads WHERE id = ?;

-- name: InsertUpload :exec
INSERT INTO uploads (id, ext, width, height, bytes, guest_id, admin_id, ip_hash, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPostUploads :many
SELECT * FROM uploads WHERE post_id = ? ORDER BY position, id;

-- name: DetachPostUploads :exec
UPDATE uploads SET post_id = NULL, position = 0 WHERE post_id = ?;

-- name: AttachUpload :exec
UPDATE uploads SET post_id = ?, position = ? WHERE id = ?;

-- name: ListOrphanUploads :many
SELECT u.id, u.ext FROM uploads u
WHERE u.post_id IS NULL AND u.created_at < ?
  AND u.id IS NOT (SELECT logo_upload_id FROM board_settings WHERE id = 1);

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = ?;
