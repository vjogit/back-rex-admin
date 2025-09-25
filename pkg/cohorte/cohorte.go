package cohorte

import (
	"back-rex-common/pkg/services"

	"net/http"

	"github.com/go-chi/render"
)

func ImportCohorte(w http.ResponseWriter, r *http.Request, ldapConfig services.LDAPConfig) {

	// Parse le multipart form (taille max 10 Mo ici)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		services.InvalidRequestError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}

	// Récupère le fichier (le champ doit s'appeler "file")
	emailsFile, err := getFile("emails", r)
	if err != nil {
		services.InvalidRequestError(w, r, "pas de fichier emails", services.NO_INFORMATION, nil)
		return
	}
	defer (*emailsFile).Close()

	// Récupère le fichier (le champ doit s'appeler "file")
	cohortesFiles, err := getFile("cohortes", r)
	if err != nil {
		services.InvalidRequestError(w, r, "pas de fichier cohortes", services.NO_INFORMATION, nil)
		return
	}
	defer (*cohortesFiles).Close()

	etudiants, warns, err := getEtudiantFromEmails(emailsFile)
	if err != nil {
		services.InvalidRequestError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}

	warnsLdap, err := getInfoEtudiantFromLdap(&etudiants, ldapConfig)
	if err != nil {
		// ne peux avoir qu'une erreur interne
		services.InternalServerError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}
	warns = append(warns, warnsLdap...)

	cohortes, err := getCohorte(cohortesFiles)
	if err != nil {
		// ne peux avoir qu'une erreur d'analyse du fichier
		services.InvalidRequestError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}

	etudiants, warnsCohortes, err := affecteCohorteToEtudiant(etudiants, cohortesFiles, cohortes)
	if err != nil {
		// ne peux avoir qu'une erreur d'analyse du fichier
		services.InvalidRequestError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}

	warns = append(warns, warnsCohortes...)

	err = toDB(r, cohortes, etudiants)
	if err != nil {
		// ne peux avoir qu'une erreur interne
		services.InternalServerError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}

	// Réponse formatée avec items et itemCount
	render.JSON(w, r, warns)

}

func GetCohortes(w http.ResponseWriter, r *http.Request) {
	pgctx := services.GetPgCtx(r.Context())
	query := New(pgctx.Db)

	cohortes, err := query.GetCohortes(r.Context())
	if err != nil {
		services.InternalServerError(w, r, err.Error(), services.NO_INFORMATION, nil)
		return
	}
	// Réponse formatée avec items et itemCount
	render.JSON(w, r, cohortes)
}
