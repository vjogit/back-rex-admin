package cohorte

import (
	userAdmin "back-rex-admin/pkg/user"
	"back-rex-common/pkg/auth"
	"back-rex-common/pkg/services"
	"back-rex-common/pkg/user"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-ldap/ldap/v3"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/unicode/norm"
)

func ImportCohorte(w http.ResponseWriter, r *http.Request) {

	// Parse le multipart form (taille max 10 Mo ici)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		services.ErrRender(errors.New("erreur lors du parsing du formulaire"))
		return
	}

	// Récupère le fichier (le champ doit s'appeler "file")
	file, header, err := r.FormFile("file")
	if err != nil {
		services.ErrRender(errors.New("fichier manquant"))
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		services.ErrRender(errors.New("le fichier n'a pas l'extension .xlsx"))
		return
	}

	// Optionnel : vérifier le Content-Type
	contentType := header.Header.Get("Content-Type")
	if contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		services.ErrRender(errors.New("le fichier n'est pas de type xlsx"))
		return
	}

	err = cohorteToDB(r, file)
	if err != nil {
		services.ErrRender(err)
		return
	}
}

func cohorteToDB(r *http.Request, file multipart.File) error {

	rows, err := getRows(file)
	if err != nil {
		return err
	}

	var cohortes []Cohorte
	cohortes, err = getCohorte(*rows)
	if err != nil {
		return err
	}

	etudiants, err := getEtudiant(*rows, cohortes)
	if err != nil {
		return err
	}

	ctx := r.Context()
	pgCtx := services.GetPgCtx(ctx)

	tx, err := pgCtx.Db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// doit creer tt lles cohortes
	for _, c := range cohortes {
		err = CreateCohorte(tx, c, ctx)
		if err != nil {
			return err
		}
	}

	// sauvegarde tous les etudiants
	for _, e := range etudiants {
		if e.LdapIdentity != nil {
			err = CreateEtudiant(tx, e, ctx)
			if err != nil {
				return err
			}
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	fmt.Println(len(etudiants))
	return nil
}

func getRows(file multipart.File) (*[][]string, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Close the spreadsheet.
		if err := f.Close(); err != nil {
			services.ErrRender(err)
		}
	}()

	// ne prends que la premiere feuille
	sheetNames := f.GetSheetList()

	if len(sheetNames) == 0 {
		return nil, errors.New("pas de données dans le fichier a importer")
	}

	rows, err := f.GetRows(sheetNames[0])
	if err != nil {
		return nil, err
	}
	return &rows, nil
}

func getCohorte(rows [][]string) ([]Cohorte, error) {
	var cohortes []Cohorte

	if len(rows) < 2 {
		return nil, fmt.Errorf("nombre de colonnes insuffisant")
	}

	if len(rows[0]) < 4 && len(rows[1]) < 4 {
		return nil, fmt.Errorf("nombre de colonnes insuffisant")
	}

	if len(rows[0]) != len(rows[1]) {
		return nil, fmt.Errorf("nombre de colonnes différent de la ligne d'en-tête")
	}

	for i, cell := range rows[0] {
		if i < 3 {
			// skip header
			continue
		}

		id, err := strconv.Atoi(cell)
		if err != nil {
			return nil, fmt.Errorf("ligne %d: id invalide: %v", i+1, err)
		}
		nom := strings.TrimSpace(rows[1][i])
		if nom == "" {
			return nil, fmt.Errorf("ligne %d: nom vide", i+1)
		}
		cohortes = append(cohortes, Cohorte{
			Id:  id,
			Nom: nom,
		})
	}
	return cohortes, nil

}

func CreateCohorte(tx pgx.Tx, c Cohorte, ctx context.Context) error {
	query := New(tx)

	err := query.CreateCohorte(ctx, CreateCohorteParams{
		Idexterne: int32(c.Id),
		Nom:       c.Nom,
	})

	if err != nil {
		return err
	}
	return nil
}

type Cohorte struct {
	Id  int    `json:"id"`
	Nom string `json:"nom"`
}

func getEtudiant(rows [][]string, cohorte []Cohorte) ([]Etudiant, error) {
	var etudiants []Etudiant

	for i, cells := range rows {
		if i < 2 {
			// skip header
			continue
		}
		for j, cell := range cells {
			switch j {
			case 0:
				etudiants = append(etudiants, Etudiant{})
				id, err := strconv.Atoi(cell)
				if err != nil {
					return nil, fmt.Errorf("ligne %d: id invalide: %v", i+1, err)
				}
				etudiants[len(etudiants)-1].Id = id
			case 1:
				// nom
				etudiants[len(etudiants)-1].Nom = strings.TrimSpace(cell)
			case 2:
				// prenom
				etudiants[len(etudiants)-1].Prenom = strings.TrimSpace(cell)
			default:
				// cohortes
				if strings.TrimSpace(cell) == "1" {
					if j-3 < len(cohorte) {
						etudiants[len(etudiants)-1].Cohortes = append(etudiants[len(etudiants)-1].Cohortes, cohorte[j-3].Id)
					} else {
						return nil, fmt.Errorf("ligne %d: cohorte inconnue", i+1)
					}
				}
			}
		}

		// recupere l'identifiant ldap.

		ldapId, err := getLdapInformation(etudiants[len(etudiants)-1])
		if err == nil {
			etudiants[len(etudiants)-1].LdapIdentity = ldapId
			fmt.Printf("%s %d\n", etudiants[len(etudiants)-1].Nom, ldapId.Id)
		} else {
			fmt.Printf("\tinconnu: %s %s: %v\n", etudiants[len(etudiants)-1].Prenom, etudiants[len(etudiants)-1].Nom, err)
		}

	}

	return etudiants, nil
}

type Etudiant struct {
	Id           int
	Nom          string `json:"nom"`
	Prenom       string `json:"prenom"`
	LdapIdentity *auth.LdapIdentity
	Cohortes     []int `json:"cohorte_id"`
}

func getLdapInformation(etudiant Etudiant) (*auth.LdapIdentity, error) {

	cfg := services.LDAPConfig{
		URL: "ldap://localhost:3890",
		//URL:    "ldap://ldap.mines-ales.fr:389",
		BaseDN: "dc=ema,dc=fr",
	}

	// Connexion au serveur LDAP
	l, err := ldap.DialURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("LDAP connection failed: %v", err)
	}

	defer l.Close()

	nom := Normalise(etudiant.Nom)
	prenom := Normalise(etudiant.Prenom)
	filter := fmt.Sprintf("(&(sn=%s)(givenName=%s))", ldap.EscapeFilter(nom), ldap.EscapeFilter(prenom))

	searchRequest := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		filter,
		[]string{"*"}, // si nil, retourne ts les attibuts.
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %v", err)
	}
	if len(sr.Entries) != 1 {
		return nil, fmt.Errorf("utilisateur inconnu")
	}

	identity, err := auth.GetLdapIdentity(sr.Entries[0])
	if err != nil {
		return nil, err
	}

	return identity, nil

}

func Normalise(s string) string {

	t := norm.NFD.String(s)
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1 // retire le caractère accentué
		}
		return r
	}, t)
}

func CreateEtudiant(tx pgx.Tx, e Etudiant, ctx context.Context) error {

	if e.LdapIdentity == nil {
		return nil // rien a faire
	}
	// doit regarder si l'utilisateur existe deja
	// Une erreur provoque la fin de la transaction

	queriesUerAdmin := userAdmin.New(tx)

	var (
		id  int32
		err error
	)

	id, err = queriesUerAdmin.GetIdFromLdapid(ctx, int32(e.LdapIdentity.Id))
	if err != nil {
		if err != pgx.ErrNoRows {
			return err
		}
		idUser, err := user.CreateUser(tx, e.LdapIdentity, ctx, "etudiant", true)
		if err != nil {
			return err
		}
		id = int32(idUser)

	}

	// doit mettre a jour les cohortes de l'utilisateur
	queries := New(tx)

	err = queries.DeleteUserCohortes(ctx, int32(id))
	if err != nil {
		return err
	}

	for _, c := range e.Cohortes {
		idCOhorte, err := queries.GetCohorteIdFromIdExterne(ctx, int32(c)) // verifie que la cohorte existe
		if err != nil {
			return err
		}
		err = queries.InsertUserCohorte(ctx, InsertUserCohorteParams{
			UserID:    id,
			CohorteID: idCOhorte,
		})
		fmt.Println(id, c)
		if err != nil {
			return err
		}

	}
	return nil

}
