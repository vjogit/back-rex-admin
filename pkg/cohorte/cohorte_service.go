package cohorte

import (
	userAdmin "back-rex-admin/pkg/user"
	"back-rex-common/pkg/auth"
	"back-rex-common/pkg/services"
	"back-rex-common/pkg/user"

	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

func getFile(name string, r *http.Request) (*multipart.File, error) {
	// Récupère le fichier (le champ doit s'appeler "file")
	file, header, err := r.FormFile(name)
	if err != nil {
		return nil, errors.New("fichier manquant:cohortes")
	}

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		return nil, errors.New("le fichier n'a pas l'extension .xlsx")
	}

	// Optionnel : vérifier le Content-Type
	contentType := header.Header.Get("Content-Type")
	if contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		return nil, errors.New("le fichier n'est pas de type xlsx")
	}

	return &file, nil
}

func getEtudiantFromEmails(emails *multipart.File) ([]Etudiant, []string, error) {

	var warn []string

	rows, err := getRows(*emails)
	if err != nil {
		return nil, nil, err
	}

	var etudiants []Etudiant
	if len(*rows) < 2 {
		return nil, nil, fmt.Errorf("fichier emails: nombre de lignes insuffisantes")
	}

	for i, cells := range *rows {
		if i < 1 {
			// skip header
			continue
		}

		if len(cells) < 10 {
			warn = append(warn, fmt.Sprintf("fichier emails: ligne %d: nombre de colonnes insuffisant", i+1))
			continue
		}

		etudiant := Etudiant{
			Nom:       strings.TrimSpace(cells[1]),
			Prenom:    strings.TrimSpace(cells[2]),
			IdInterne: strings.TrimSpace(cells[10]),
			LdapIdentity: &auth.LdapIdentity{
				Mail: strings.TrimSpace(cells[9]),
			},
		}

		etudiants = append(etudiants, etudiant)
	}

	return etudiants, warn, nil
}

func getInfoEtudiantFromLdap(etudiant *[]Etudiant, ldapConfig services.LDAPConfig) ([]string, error) {
	var warns []string

	// Connexion au serveur LDAP
	l, err := ldap.DialURL(ldapConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("LDAP connection failed: %v", err)
	}

	defer l.Close()

	for i, e := range *etudiant {
		info, err := getLdapInformation(e, l, ldapConfig.BaseDN)
		if err != nil {
			warns = append(warns, fmt.Sprintf("ldap : %v", err))
			continue
		}
		(*etudiant)[i].LdapIdentity = info //e.LdapIdentity = info ne marche pas car e est une copie de l'élément du slice
	}

	return warns, nil
}

func getLdapInformation(etudiant Etudiant, l *ldap.Conn, baseDN string) (*auth.LdapIdentity, error) {

	filter := fmt.Sprintf("(mail=%s)", ldap.EscapeFilter(etudiant.LdapIdentity.Mail))

	searchRequest := ldap.NewSearchRequest(
		baseDN,
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
		return nil, fmt.Errorf("utilisateur inconnu: %s", etudiant.LdapIdentity.Mail)
	}

	identity, err := auth.GetLdapIdentity(sr.Entries[0])
	if err != nil {
		return nil, err
	}

	return identity, nil

}

func getRows(file multipart.File) (*[][]string, error) {

	// --- Réinitialiser le curseur de lecture ---
	// La méthode Seek ramène le curseur au début du fichier
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("erreur de réinitialisation du curseur : %w", err)
	}

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

func getCohorte(cohortesFiles *multipart.File) ([]Cohorte, error) {
	rows, err := getRows(*cohortesFiles)
	if err != nil {
		return nil, err
	}

	if len(*rows) < 2 {
		return nil, fmt.Errorf("nombre de colonnes insuffisant")
	}

	if len((*rows)[0]) < 4 && len((*rows)[1]) < 4 {
		return nil, fmt.Errorf("nombre de colonnes insuffisant")
	}

	if len((*rows)[0]) != len((*rows)[1]) {
		return nil, fmt.Errorf("nombre de colonnes différent de la ligne d'en-tête")
	}

	var cohortes []Cohorte
	for i, cell := range (*rows)[0] {
		if i < 3 {
			// skip header
			continue
		}

		id, err := strconv.Atoi(cell)
		if err != nil {
			return nil, fmt.Errorf("ligne %d: id invalide: %v", i+1, err)
		}
		nom := strings.TrimSpace((*rows)[1][i])
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

func affecteCohorteToEtudiant(etudiant2 []Etudiant, cohortesFiles *multipart.File, cohortes []Cohorte) ([]Etudiant, []string, error) {

	rows, err := getRows(*cohortesFiles)
	if err != nil {
		return nil, nil, err
	}

	etudiantCohorteFile, err := getEtudiantFromCohorteFile(*rows, cohortes)
	if err != nil {
		return nil, nil, err
	}

	// crée une map pour retrouver les étudiants par nom et prenom
	etudiantCohorteMap := make(map[string]*Etudiant)
	for i := range etudiantCohorteFile {
		etudiantCohorteMap[etudiantCohorteFile[i].IdInterne] = &etudiantCohorteFile[i]
	}

	var warns []string
	// pour chaque étudiant du fichier de cohorte, cherche l'étudiant correspondant dans la map
	for i, e := range etudiant2 {
		if e.LdapIdentity == nil {
			continue
		}

		if etu, ok := etudiantCohorteMap[e.IdInterne]; ok {
			// ajoute les cohortes
			etudiant2[i].Cohortes = append(etu.Cohortes, e.Cohortes...)
		} else {
			warns = append(warns, fmt.Sprintf("étudiant present dans fichier mail, mais pas dans fichier cohorte: %s %s\n", e.Prenom, e.Nom))
		}
	}

	return etudiant2, warns, nil
}

func getEtudiantFromCohorteFile(rows [][]string, cohorte []Cohorte) ([]Etudiant, error) {
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
				etudiants[len(etudiants)-1].IdInterne = cell
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
	}
	return etudiants, nil
}

func toDB(r *http.Request, cohortes []Cohorte, etudiants []Etudiant) error {
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

	return nil

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

func CreateEtudiant(tx pgx.Tx, e Etudiant, ctx context.Context) error {

	if e.LdapIdentity == nil {
		return nil // rien a faire
	}
	// doit regarder si l'utilisateur existe deja
	// Une erreur provoque la fin de la transaction

	queriesUerAdmin := userAdmin.New(tx)

	var (
		u   userAdmin.User
		id  int32
		err error
	)

	u, err = queriesUerAdmin.GetUserByMail(ctx, e.LdapIdentity.Mail)

	switch err {
	case nil:
		id = u.ID
	case pgx.ErrNoRows:
		idUser, err := user.CreateUser(tx, e.LdapIdentity, ctx, "etudiant", true)
		if err != nil {
			return err
		}
		id = int32(idUser)
	default:
		return err
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
		if err != nil {
			return err
		}

	}
	return nil

}

type Cohorte struct {
	Id  int    `json:"id"`
	Nom string `json:"nom"`
}
type Etudiant struct {
	IdInterne    string `json:"id_interne"`
	Nom          string `json:"nom"`
	Prenom       string `json:"prenom"`
	LdapIdentity *auth.LdapIdentity
	Cohortes     []int `json:"cohorte_id"`
}
