-- name: CreateFeedFollow :one
WITH inserted_feedfollow AS (
    INSERT INTO feedfollows (
        id,
        created_at,
        updated_at,
        user_id,
        feed_id
    )
    VALUES (
        $1,
        $2,
        $3,
        $4,
        $5
    )
    RETURNING *
) SELECT
    inserted_feedfollow.*,
    feeds.name AS feed_name,
    users.name AS user_name
FROM inserted_feedfollow
    INNER JOIN users ON users.id = inserted_feedfollow.user_id
    INNER JOIN feeds ON feeds.id = inserted_feedfollow.feed_id;

-- name: GetFeedFollowsForUser :many
SELECT
    feedfollows.*,
    feeds.name AS feed_name,
    users.name AS user_name
FROM feedfollows
    INNER JOIN users ON users.id = feedfollows.user_id
    INNER JOIN feeds ON feeds.id = feedfollows.feed_id
WHERE
    users.name = $1;

-- name: DeleteFeedFollow :exec
DELETE
FROM feedfollows
USING
    users, feeds
WHERE
    users.id = feedfollows.user_id
AND feeds.id = feedfollows.feed_id
AND users.name = $1
AND feeds.url = $2;

-- name: DeleteFeedFollows :exec
DELETE FROM feedfollows;