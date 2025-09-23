package user

import (
	"back-rex-common/pkg/auth"
	"back-rex-common/pkg/services"
	"back-rex-common/pkg/user"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/go-ldap/ldap/v3"
	"github.com/jackc/pgx/v5"
)

type UserRequest struct {
	ID      int32  `json:"id"`
	Version int    `json:"version"`
	Email   string `json:"email"`
	Roles   string `json:"roles"`
	Blame   bool   `json:"blame"`
}

func CreateUser(w http.ResponseWriter, r *http.Request, cfg services.LDAPConfig) {
	var input UserRequest
	if err := render.DecodeJSON(r.Body, &input); err != nil {
		render.Render(w, r, services.ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	pgCtx := services.GetPgCtx(ctx)

	tx, err := pgCtx.Db.Begin(ctx)
	if err != nil {
		render.Render(w, r, services.ErrRender(err))
		return
	}

	defer tx.Rollback(ctx)

	etudiant := len(input.Roles) != 0 && strings.Contains(input.Roles, "etudiant")

	sr, err := getLdapUser(input.Email, cfg)
	if err != nil {
		render.Render(w, r, services.ErrRender(err))
		return
	}

	e, err := auth.GetLdapIdentity(sr.Entries[0])
	if err != nil {
		render.Render(w, r, services.ErrRender(err))
		return
	}

	id, err := user.CreateUser(tx, e, ctx, input.Roles, etudiant)
	if err != nil {
		http.Error(w, "Erreur lors de la maj d'un utilisateur", http.StatusInternalServerError)
		return
	}

	tx.Commit(r.Context())
	input.ID = int32(id)

	render.JSON(w, r, input)
}

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
		render.Render(w, r, services.ErrRender(errors.New("mis a jour par une autre personne")))
		return
	}

	if err != nil {
		render.Render(w, r, services.ErrRender(errors.New("erreur lors de la maj d'un utilisateur")))
		return

	}

	render.JSON(w, r, user)

}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	oldUser := getUserFromCtx(r)

	ctx := r.Context()
	pgctx := services.GetPgCtx(ctx)
	queries := New(pgctx.Db)

	err := queries.DeleteUser(ctx, oldUser.ID)
	if err != nil {
		render.Render(w, r, services.ErrRender(errors.New("erreur lors de la suppression d'un utilisateur")))
		return

	}

	w.WriteHeader(http.StatusNoContent)
}

type MailCheck struct {
	Exist bool `json:"exist"`
}

func CheckMail(w http.ResponseWriter, r *http.Request, cfg services.LDAPConfig) {

	email := r.URL.Query().Get("email")

	if email == "" {
		services.ErrInvalidRequest(errors.New("doit avoir le parametre email"))
		return
	}

	sr, err := getLdapUser(email, cfg)
	if err != nil {
		render.Render(w, r, services.ErrRender(err))
		return
	}
	render.JSON(w, r, MailCheck{len(sr.Entries) == 1})

}

func getLdapUser(email string, cfg services.LDAPConfig) (*ldap.SearchResult, error) {
	// Connexion au serveur LDAP
	l, err := ldap.DialURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("LDAP connection failed: %w", err)
	}
	defer l.Close()

	filter := fmt.Sprintf("(mail=%s)", ldap.EscapeFilter(email))

	searchRequest := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		filter,
		[]string{"*"}, // si nil, retourne ts les attibuts.
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}

	return sr, nil
}
