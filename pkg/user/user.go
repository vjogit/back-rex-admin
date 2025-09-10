package user

import (
	"back-rex-common/pkg/services"
	"net/http"

	"github.com/go-chi/render"
)

func ListUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pgctx := services.GetPgCtx(ctx)
	queries := New(pgctx.Db)

	students, err := queries.ListUser(ctx)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des étudiants", http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, students)
}
