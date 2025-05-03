-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING *;

-- name: GetPostsForUser :many
SELECT
    posts.*,
    feeds.name AS feed_name,
    users.name AS user_name
FROM posts
    INNER JOIN feedfollows ON posts.feed_id = feedfollows.feed_id
    INNER JOIN users ON users.id = feedfollows.user_id
    INNER JOIN feeds ON feeds.id = feedfollows.feed_id
WHERE
    users.name = $1
ORDER BY
    published_at DESC
LIMIT $2;
