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
