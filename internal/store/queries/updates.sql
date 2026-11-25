-- name: InsertUpdate :one
INSERT INTO updates (title, body_md, body_html, author_admin_id, published_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?) RETURNING id;

-- name: GetUpdate :one
SELECT u.id, u.title, u.body_md, u.body_html, u.author_admin_id, u.published_at,
  CAST(COALESCE(a.display_name, '') AS TEXT) AS admin_name
FROM updates u LEFT JOIN admins a ON a.id = u.author_admin_id WHERE u.id = ?;

-- name: ListUpdates :many
SELECT u.id, u.title, u.body_md, u.body_html, u.author_admin_id, u.published_at,
  CAST(COALESCE(a.display_name, '') AS TEXT) AS admin_name
FROM updates u LEFT JOIN admins a ON a.id = u.author_admin_id
ORDER BY u.published_at DESC, u.id DESC LIMIT ? OFFSET ?;

-- name: EditUpdate :exec
UPDATE updates SET title = ?, body_md = ?, body_html = ?, updated_at = ? WHERE id = ?;

-- name: DeleteUpdate :exec
DELETE FROM updates WHERE id = ?;

-- name: InsertUpdatePost :exec
INSERT INTO update_posts (update_id, post_id, status_id, position) VALUES (?, ?, ?, ?);

-- name: ListUpdateLinks :many
SELECT post_id, status_id FROM update_posts WHERE update_id = ? ORDER BY position;

-- name: DeleteUpdateLinks :exec
DELETE FROM update_posts WHERE update_id = ?;

-- name: ListPublicUpdatePosts :many
SELECT p.id, p.slug, p.title, up.status_id FROM update_posts up JOIN posts p ON p.id = up.post_id
WHERE up.update_id = ? AND p.deleted_at IS NULL AND p.merged_into_id IS NULL AND p.pending = 0
ORDER BY up.position;

-- name: GetPostForLink :one
SELECT status_id, deleted_at FROM posts WHERE id = ?;
