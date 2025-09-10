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
	queriesAuth := auth.New(pgCtx.Db)

	userByMail, err := queriesAuth.GetUserByMail(r.Context(), ldapIdentity.Mail)

	if err == pgx.ErrNoRows {
		return nil, nil, fmt.Errorf("utilisateur inconnu")
	}

	if err != nil {
		return nil, nil, err
	}

	claims := jwt.MapClaims{"roles": userByMail.Roles}
	subject := strconv.Itoa(int(userByMail.ID))
	return &claims, &subject, nil // Pas de claims supplémentaires pour l'instant
}
