-- name: CreateCohorte :exec
INSERT INTO public.cohorte (idExterne,nom) VALUES ($1, $2)
    on conflict (idExterne) do update set nom = EXCLUDED.nom;

-- name: InsertUserCohorte :exec
INSERT INTO user_cohorte (user_id, cohorte_id) VALUES ($1, $2);

-- name: DeleteUserCohortes :exec
DELETE FROM user_cohorte WHERE user_id = $1;

-- name: GetCohorteIdFromIdExterne :one
SELECT id FROM public.cohorte WHERE idExterne = $1;