package authentification

import (
	"back-rex-common/pkg/auth"
	"back-rex-common/pkg/services"
	"fmt"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

func PostLdap(r *http.Request, ldapIdentity *auth.LdapIdentity) (*jwt.MapClaims, *string, error) {

	pgCtx := services.GetPgCtx(r.Context())
	queries := auth.New(pgCtx.Db)

	user, err := queries.GetUserByLdapId(r.Context(), int32(ldapIdentity.Id))

	if err == pgx.ErrNoRows {
		return nil, nil, fmt.Errorf("utilisateur inconnu")
	}

	if err != nil {
		return nil, nil, err
	}

	claims := jwt.MapClaims{"roles": user.Roles}
	subject := strconv.Itoa(int(user.ID))
	return &claims, &subject, nil // Pas de claims supplémentaires pour l'instant
}
