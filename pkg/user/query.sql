-- name: ListUsers :many
SELECT * FROM "user"
ORDER BY id ASC;

-- name: GetUser :one
SELECT * FROM "user"
WHERE id = $1;

-- name: AddAdminRole :exec
UPDATE "user"
SET roles = array_append(roles, 'admin')
WHERE id = $1 AND NOT ('admin' = ANY(roles));

-- name: RemoveAdminRole :exec
UPDATE "user"
SET roles = array_remove(roles, 'admin')
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO public.user (name, surname, email, roles)
VALUES (@name, @surname, @email, @roles)
RETURNING *;

-- name: GetUserById :one
SELECT * FROM public.user WHERE id = $1;

-- name: ListUser :many
SELECT * FROM public.user ORDER BY id;

-- name: UpdateUser :one
UPDATE public.user
SET name = @name,
    surname = @surname,
    email = @email,
    roles = @roles,
    version = version + 1
WHERE id = @id AND version = @version
RETURNING version;

-- name: DeleteUser :exec
DELETE FROM public.user WHERE id = @id;

-- name: GetIdFromLdapid :one
  SELECT id FROM public.user WHERE ldapid = $1;