-- name: GetSettings :one
SELECT * FROM board_settings WHERE id = 1;

-- name: GetLogoExt :one
SELECT u.ext FROM board_settings s JOIN uploads u ON u.id = s.logo_upload_id WHERE s.id = 1;

-- name: UpdateSettings :exec
UPDATE board_settings SET title_pt = ?, title_en = ?, description_pt = ?, description_en = ?,
  team_name = ?, accent_color = ?, logo_upload_id = ?, require_approval = ?, uploads_enabled = ?
WHERE id = 1;

-- name: ListStatuses :many
SELECT * FROM statuses ORDER BY position, id;

-- name: GetStatus :one
SELECT * FROM statuses WHERE id = ?;

-- name: InsertStatus :one
INSERT INTO statuses (name_pt, name_en, color, position, show_on_roadmap, is_default)
VALUES (?, ?, ?, ?, ?, 0) RETURNING *;

-- name: UpdateStatus :exec
UPDATE statuses SET name_pt = ?, name_en = ?, color = ?, show_on_roadmap = ? WHERE id = ?;

-- name: SetStatusPosition :exec
UPDATE statuses SET position = ? WHERE id = ?;

-- name: ClearDefaultStatus :exec
UPDATE statuses SET is_default = 0 WHERE is_default = 1;

-- name: SetDefaultStatus :exec
UPDATE statuses SET is_default = 1 WHERE id = ?;

-- name: CountStatusUsage :one
SELECT (SELECT count(*) FROM posts WHERE posts.status_id = sqlc.arg('id'))
     + (SELECT count(*) FROM update_posts WHERE update_posts.status_id = sqlc.arg('id'));

-- name: MovePostsStatus :exec
UPDATE posts SET status_id = sqlc.arg('to_id') WHERE status_id = sqlc.arg('from_id');

-- name: MoveUpdateLinksStatus :exec
UPDATE update_posts SET status_id = sqlc.arg('to_id') WHERE status_id = sqlc.arg('from_id');

-- name: DeleteStatus :exec
DELETE FROM statuses WHERE id = ?;

-- name: ListTags :many
SELECT * FROM tags ORDER BY position, id;

-- name: GetTag :one
SELECT * FROM tags WHERE id = ?;

-- name: InsertTag :one
INSERT INTO tags (name_pt, name_en, position) VALUES (?, ?, ?) RETURNING *;

-- name: UpdateTag :exec
UPDATE tags SET name_pt = ?, name_en = ? WHERE id = ?;

-- name: SetTagPosition :exec
UPDATE tags SET position = ? WHERE id = ?;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;
