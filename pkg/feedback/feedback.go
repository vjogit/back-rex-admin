package feedback

import (
	"back-rex-common/pkg/services"
	"context"
	"net/http"

	"github.com/go-chi/render"
)

type FeedbackResponse struct {
	ID        int32  `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	Promotion string `json:"promotion"`
}

func GetAllFeedback(w http.ResponseWriter, r *http.Request) {
	pgctx := services.GetPgCtx(r.Context())
	query := New(pgctx.Db)

	feedbacks, err := query.ListFeedbacks(context.Background())
	if err != nil {
		services.ErrRender(err)
		return
	}
	// Réponse formatée avec items et itemCount
	render.JSON(w, r, feedbacks)
}
