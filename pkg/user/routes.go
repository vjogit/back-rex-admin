package user

import (
	"back-rex-common/pkg/services"
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func RouteUtilisateur(r chi.Router, cfg services.LDAPConfig) {
	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		CreateUser(w, r, cfg)
	})

	r.Route("/{userID}", func(r chi.Router) {
		r.Use(UserUse)            // Load the *Article on the request context
		r.Get("/", GetUser)       // GET /articles/123
		r.Put("/", UpdateUser)    // PUT /articles/123
		r.Delete("/", DeleteUser) // DELETE /articles/123
	})

	r.Get("/", ListUser)
	r.Get("/check-mail", func(w http.ResponseWriter, r *http.Request) {
		CheckMail(w, r, cfg)
	})
}

func UserUse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if userID := chi.URLParam(r, "userID"); userID != "" {
			pgCtx := services.GetPgCtx(r.Context())

			id, err := strconv.Atoi(userID)
			if err != nil {
				render.Render(w, r, services.ErrRender(err))
				return
			}

			queries := New(pgCtx.Db)
			user, err := queries.GetUserById(context.Background(), int32(id))
			if err != nil {
				render.Render(w, r, services.ErrRender(err))
				return
			}

			ctx := setUserFromCtx(r, &user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		render.Render(w, r, services.ErrRender(errors.New("pas d'id utilisateur")))
	})
}
