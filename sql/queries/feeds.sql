-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, NOW(), NOW(), $2, $3, $4)
RETURNING *;

-- name: GetFeeds :many
SELECT feeds.*, users.name AS user_name FROM feeds
INNER JOIN users ON feeds.user_id = users.id;

-- name: GetFeed :one
SELECT * FROM feeds WHERE url = $1;
