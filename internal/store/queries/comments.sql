-- name: ListPostComments :many
SELECT c.id, c.post_id, c.parent_id, c.author_guest_id, c.author_admin_id, c.body_md, c.body_html,
  c.like_count, c.created_at, c.deleted_at,
  g.name AS guest_name, CAST(COALESCE(a.display_name, '') AS TEXT) AS admin_name
FROM comments c
LEFT JOIN guests g ON g.id = c.author_guest_id
LEFT JOIN admins a ON a.id = c.author_admin_id
WHERE c.post_id = ? ORDER BY c.created_at, c.id;

-- name: GetComment :one
SELECT c.id, c.post_id, c.parent_id, c.author_guest_id, c.author_admin_id, c.body_md, c.body_html,
  c.like_count, c.created_at, c.deleted_at,
  g.name AS guest_name, CAST(COALESCE(a.display_name, '') AS TEXT) AS admin_name
FROM comments c
LEFT JOIN guests g ON g.id = c.author_guest_id
LEFT JOIN admins a ON a.id = c.author_admin_id
WHERE c.id = ?;

-- name: LikedByGuest :many
SELECT cl.comment_id FROM comment_likes cl JOIN comments c ON c.id = cl.comment_id
WHERE c.post_id = ? AND cl.guest_id = ?;

-- name: HasLiked :one
SELECT count(*) FROM comment_likes WHERE comment_id = ? AND guest_id = ?;

-- name: InsertComment :one
INSERT INTO comments (post_id, parent_id, author_guest_id, author_admin_id, body_md, body_html, ip_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id;

-- name: UpdateCommentBody :exec
UPDATE comments SET body_md = ?, body_html = ?, updated_at = ? WHERE id = ?;

-- name: SoftDeleteComment :exec
UPDATE comments SET deleted_at = ?, deleted_by_admin_id = ? WHERE id = ?;

-- name: RecountComments :exec
UPDATE posts SET comment_count = (SELECT count(*) FROM comments WHERE comments.post_id = posts.id AND comments.deleted_at IS NULL)
WHERE posts.id = ?;

-- name: GetPostLock :one
SELECT comments_locked FROM posts WHERE id = ?;

-- name: InsertLike :exec
INSERT INTO comment_likes (comment_id, guest_id, created_at) VALUES (?, ?, ?)
ON CONFLICT (comment_id, guest_id) DO NOTHING;

-- name: DeleteLike :exec
DELETE FROM comment_likes WHERE comment_id = ? AND guest_id = ?;

-- name: RecountLikes :one
UPDATE comments SET like_count = (SELECT count(*) FROM comment_likes WHERE comment_likes.comment_id = comments.id)
WHERE comments.id = ? RETURNING like_count;

-- name: RestoreComment :exec
UPDATE comments SET deleted_at = NULL, deleted_by_admin_id = NULL WHERE id = ?;

-- name: TrashComments :many
SELECT c.id, c.post_id, p.title AS post_title, c.body_md, c.deleted_at, c.deleted_by_admin_id
FROM comments c JOIN posts p ON p.id = c.post_id
WHERE c.deleted_at IS NOT NULL ORDER BY c.deleted_at DESC, c.id DESC LIMIT ?;
