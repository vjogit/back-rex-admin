package user

import (
	"back-rex-common/pkg/services"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5"
)

func GetUser(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	render.JSON(w, r, user)
}

func ListUser(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	pgctx := services.GetPgCtx(ctx)
	queries := New(pgctx.Db)

	users, err := queries.ListUser(ctx)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des étudiants", http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, users)
}

type UserRequest struct {
	Roles string `json:"roles"`
	Blame bool   `json:"blame"`
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {

	var input UserRequest
	if err := render.DecodeJSON(r.Body, &input); err != nil {
		render.Render(w, r, services.ErrInvalidRequest(err))
		return
	}

	oldUser := getUserFromCtx(r)

	ctx := r.Context()
	pgctx := services.GetPgCtx(ctx)
	queries := New(pgctx.Db)

	user, err := queries.UpdatePartialUser(ctx, UpdatePartialUserParams{
		ID:      oldUser.ID,
		Version: oldUser.Version,
		Roles:   services.ToPgText(input.Roles),
		Blame:   services.ToPgBool(input.Blame),
	})

	if err == pgx.ErrNoRows {
		http.Error(w, "Mis a jour par une autre personne", http.StatusInternalServerError)
		return
	}

	if err != nil {
		http.Error(w, "Erreur lors de la maj d'un utilisateur", http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, user)

}

type MailCheck struct {
	Exist bool `json:"exist"`
}

func CheckMail(w http.ResponseWriter, r *http.Request) {

	email := r.URL.Query().Get("email")

	if email == "" {
		http.Error(w, "doit avoir le parametre email", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pgctx := services.GetPgCtx(ctx)
	queries := New(pgctx.Db)

	_, err := queries.GetUserByMail(r.Context(), email)

	if err == pgx.ErrNoRows {
		render.JSON(w, r, MailCheck{false})
	}

	if err != nil {
		services.ErrRender(err)
		return
	}

	render.JSON(w, r, MailCheck{true})

}
