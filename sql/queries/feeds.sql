-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, NOW(), NOW(), $2, $3, $4)
RETURNING *;
