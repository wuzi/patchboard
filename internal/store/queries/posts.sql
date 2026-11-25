-- name: InsertPost :one
INSERT INTO posts (slug, title, body_md, body_html, excerpt, status_id, tag_id,
  author_guest_id, author_admin_id, ip_hash, pending, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id;

-- name: GetPostFull :one
SELECT p.id, p.slug, p.title, p.body_md, p.body_html, p.excerpt, p.status_id, p.tag_id,
  p.author_guest_id, p.author_admin_id, p.vote_count, p.comment_count, p.pending, p.pinned_at,
  p.comments_locked, p.merged_into_id, p.deleted_at, p.deleted_by_admin_id,
  p.response_md, p.response_html, p.response_admin_id, p.response_at, p.created_at, p.updated_at,
  g.name AS guest_name,
  CAST(COALESCE(a.display_name, '') AS TEXT) AS admin_name,
  CAST(COALESCE(ra.display_name, '') AS TEXT) AS response_admin_name
FROM posts p
LEFT JOIN guests g ON g.id = p.author_guest_id
LEFT JOIN admins a ON a.id = p.author_admin_id
LEFT JOIN admins ra ON ra.id = p.response_admin_id
WHERE p.id = ?;

-- name: GetPostRef :one
SELECT id, slug, merged_into_id FROM posts WHERE id = ?;

-- name: UpdatePostContent :exec
UPDATE posts SET slug = ?, title = ?, body_md = ?, body_html = ?, excerpt = ?, tag_id = ?, updated_at = ?
WHERE id = ?;

-- name: SoftDeletePost :exec
UPDATE posts SET deleted_at = ?, deleted_by_admin_id = ? WHERE id = ?;

-- name: HasVoted :one
SELECT count(*) FROM votes WHERE post_id = ? AND guest_id = ?;

-- name: RecentVoters :many
SELECT v.guest_id, g.name FROM votes v JOIN guests g ON g.id = v.guest_id
WHERE v.post_id = ? ORDER BY v.created_at DESC, v.guest_id LIMIT 3;

-- name: CountVotesSince :one
SELECT count(*) FROM votes WHERE post_id = ? AND created_at >= ?;

-- name: SetPostStatus :exec
UPDATE posts SET status_id = ?, updated_at = ? WHERE id = ?;

-- name: SetPostResponse :exec
UPDATE posts SET response_md = ?, response_html = ?, response_admin_id = ?, response_at = ? WHERE id = ?;
