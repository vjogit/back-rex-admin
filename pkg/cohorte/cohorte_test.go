package cohorte

import (
	"back-rex-common/pkg/services"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
)

func TestImportCohorte(t *testing.T) {

	// Ouvre le fichier à uploader
	filePath := "/home/vjo/Bureau/eleves-gropes-1A-FIG-2024-2025.partiel.xlsx"
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("impossible d'ouvrir le fichier: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Crée une partie personnalisée pour le fichier
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="eleves-gropes-1A-FIG-2024-2025.xlsx"`)
	partHeader.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("impossible de créer le champ file: %v", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		t.Fatalf("impossible de copier le fichier: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()

	dsn := "host=localhost port=5432 user=postgres password=root dbname=db_rex sslmode=disable"

	pg := services.NewPG(context.Background(), dsn)
	ctx := context.WithValue(req.Context(), services.PgCtxKey2, pg)
	req = req.WithContext(ctx)

	ImportCohorte(rr, req)

	// Optionally, check output or error rendering
}
