package cohorte

import "github.com/go-chi/chi/v5"

func RouteCohorte(r chi.Router) {
	r.Post("/", ImportCohorte)
}
