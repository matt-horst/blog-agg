-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, name)
VALUES ($1, NOW(), NOW(), $2) 
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE name = $1;
