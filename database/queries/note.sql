-- name: CreateNote :one
INSERT INTO notes (title, content, user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetNoteByID :one
SELECT * FROM notes WHERE id = $1;

-- name: DeleteNote :exec
DELETE FROM notes WHERE id = $1;
