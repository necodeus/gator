-- name: CreateFeed :one
INSERT INTO feeds (id, user_id, name, url)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetFeedsByName :many
SELECT *
FROM feeds
WHERE name = $1;

-- name: GetFeeds :many
SELECT *
FROM feeds;
