-- name: InsertVote :exec
INSERT INTO votes (post_id, guest_id, ip_hash, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT (post_id, guest_id) DO NOTHING;

-- name: DeleteVote :exec
DELETE FROM votes WHERE post_id = ? AND guest_id = ?;

-- name: RecountVotes :one
UPDATE posts SET vote_count = (SELECT count(*) FROM votes WHERE votes.post_id = posts.id)
WHERE posts.id = ? RETURNING vote_count;

-- name: GetPostState :one
SELECT deleted_at, merged_into_id, pending FROM posts WHERE id = ?;

-- name: ListPostVotes :many
SELECT v.guest_id, g.name, g.created_at AS guest_created_at, v.ip_hash, v.created_at,
  (SELECT count(*) FROM votes o WHERE o.post_id = v.post_id AND o.ip_hash = v.ip_hash AND o.guest_id <> v.guest_id) AS same_ip
FROM votes v JOIN guests g ON g.id = v.guest_id
WHERE v.post_id = ? ORDER BY v.created_at DESC, v.guest_id;
