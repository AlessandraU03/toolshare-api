package toolservice

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

// --- fakes minimos, sin tocar Postgres/Supabase reales ---

type fakeToolRepo struct {
	tool *tooldomain.Tool
}

func (f *fakeToolRepo) Create(ctx context.Context, tool *tooldomain.Tool) (*tooldomain.Tool, error) {
	return tool, nil
}
func (f *fakeToolRepo) FindByID(ctx context.Context, id uuid.UUID) (*tooldomain.Tool, error) {
	return f.tool, nil
}
func (f *fakeToolRepo) FindAll(ctx context.Context, filter toolports.ToolFilter) ([]*tooldomain.Tool, error) {
	return nil, nil
}
func (f *fakeToolRepo) Update(ctx context.Context, tool *tooldomain.Tool) (*tooldomain.Tool, error) {
	f.tool = tool
	return tool, nil
}
func (f *fakeToolRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (f *fakeToolRepo) SetAvailability(ctx context.Context, id uuid.UUID, available bool) error {
	return nil
}

type fakeToolPhotoRepo struct {
	photos []*tooldomain.ToolPhoto
	nextID int
}

func (f *fakeToolPhotoRepo) Create(ctx context.Context, photo *tooldomain.ToolPhoto) (*tooldomain.ToolPhoto, error) {
	f.nextID++
	photo.ID = f.nextID
	f.photos = append(f.photos, photo)
	return photo, nil
}
func (f *fakeToolPhotoRepo) FindByToolID(ctx context.Context, toolID uuid.UUID) ([]*tooldomain.ToolPhoto, error) {
	var out []*tooldomain.ToolPhoto
	for _, p := range f.photos {
		if p.ToolID == toolID {
			out = append(out, p)
		}
	}
	return out, nil
}

// recordingStorage implementa sharedports.FileStorage guardando lo que recibe,
// sin llamar a Supabase real.
type recordingStorage struct {
	received    []byte
	uploadCalls int
	returnURL   string
}

func (r *recordingStorage) Upload(ctx context.Context, filename string, content io.Reader, contentType string) (string, error) {
	buf := &bytes.Buffer{}
	if _, err := buf.ReadFrom(content); err != nil {
		return "", err
	}
	r.received = buf.Bytes()
	r.uploadCalls++
	r.returnURL = "https://fake-storage.local/" + filename
	return r.returnURL, nil
}
func (r *recordingStorage) Delete(ctx context.Context, filename string) error { return nil }

var _ sharedports.FileStorage = (*recordingStorage)(nil)

func newTestService(t *testing.T) (*toolService, *fakeToolRepo, *fakeToolPhotoRepo, *recordingStorage, uuid.UUID, uuid.UUID) {
	t.Helper()
	if os.Getenv("ML_SERVICE_URL") == "" {
		t.Setenv("ML_SERVICE_URL", "http://127.0.0.1:8001")
	}
	ownerID := uuid.New()
	toolID := uuid.New()
	repo := &fakeToolRepo{
		tool: &tooldomain.Tool{
			ID:             toolID,
			OwnerID:        ownerID,
			ConditionScore: 0.70,
			IsAvailable:    false,
		},
	}
	photoRepo := &fakeToolPhotoRepo{}
	storage := &recordingStorage{}
	svc := NewToolService(repo, photoRepo, nil, storage, nil).(*toolService)
	return svc, repo, photoRepo, storage, ownerID, toolID
}

func readTestImage(t *testing.T, envVar string) []byte {
	t.Helper()
	path := os.Getenv(envVar)
	if path == "" {
		t.Skipf("define %s con una imagen real para correr esta prueba", envVar)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", path, err)
	}
	return b
}

func TestUploadPhoto_UnaFotoNoAlcanzaParaPublicar(t *testing.T) {
	imgBytes := readTestImage(t, "TEST_IMAGE_PATH")
	svc, _, photoRepo, storage, ownerID, toolID := newTestService(t)

	updated, err := svc.UploadPhoto(context.Background(), toolports.UploadPhotoInput{
		ToolID:      toolID,
		OwnerID:     ownerID,
		Filename:    "angulo1.jpg",
		Content:     bytes.NewReader(imgBytes),
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("UploadPhoto fallo: %v", err)
	}

	if storage.uploadCalls != 1 {
		t.Fatalf("se esperaba 1 llamada a Upload, hubo %d", storage.uploadCalls)
	}
	if !bytes.Equal(storage.received, imgBytes) {
		t.Fatalf("los bytes subidos no coinciden con la imagen original")
	}
	if len(photoRepo.photos) != 1 {
		t.Fatalf("se esperaba 1 foto guardada en tool_photos, hay %d", len(photoRepo.photos))
	}
	if updated.ConditionScore == 0.70 {
		t.Fatalf("ConditionScore no se actualizo desde el valor inicial 'de cliente'")
	}
	if updated.IsAvailable {
		t.Fatalf("con MinRequiredPhotos=%d, 1 sola foto NO deberia activar IsAvailable", MinRequiredPhotos)
	}
	t.Logf("1 foto: ConditionScore=%f IsAvailable=%v", updated.ConditionScore, updated.IsAvailable)
}

func TestUploadPhoto_PeorScoreGanaYActivaAlLlegarAlMinimo(t *testing.T) {
	imgBuena := readTestImage(t, "TEST_IMAGE_PATH")       // ej. herramienta que se ve nueva
	imgDesgastada := readTestImage(t, "TEST_IMAGE_PATH_2") // ej. herramienta con desgaste visible
	svc, _, photoRepo, _, ownerID, toolID := newTestService(t)

	if _, err := svc.UploadPhoto(context.Background(), toolports.UploadPhotoInput{
		ToolID: toolID, OwnerID: ownerID, Filename: "angulo1.jpg",
		Content: bytes.NewReader(imgBuena), ContentType: "image/jpeg",
	}); err != nil {
		t.Fatalf("primera foto fallo: %v", err)
	}

	updated, err := svc.UploadPhoto(context.Background(), toolports.UploadPhotoInput{
		ToolID: toolID, OwnerID: ownerID, Filename: "angulo2.jpg",
		Content: bytes.NewReader(imgDesgastada), ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("segunda foto fallo: %v", err)
	}

	if len(photoRepo.photos) != 2 {
		t.Fatalf("se esperaban 2 fotos guardadas, hay %d", len(photoRepo.photos))
	}

	peor := photoRepo.photos[0].ConditionScore
	for _, p := range photoRepo.photos {
		if p.ConditionScore < peor {
			peor = p.ConditionScore
		}
	}
	if updated.ConditionScore != peor {
		t.Fatalf("ConditionScore final (%f) deberia ser el minimo de las fotos (%f)", updated.ConditionScore, peor)
	}
	if !updated.IsAvailable {
		t.Fatalf("con %d fotos (MinRequiredPhotos=%d) la herramienta deberia quedar disponible", len(photoRepo.photos), MinRequiredPhotos)
	}
	t.Logf("2 fotos: scores=%v -> ConditionScore final=%f IsAvailable=%v",
		[]float64{photoRepo.photos[0].ConditionScore, photoRepo.photos[1].ConditionScore}, updated.ConditionScore, updated.IsAvailable)
}

func TestUploadPhoto_SiCNNFallaNoTumbaLaSubida(t *testing.T) {
	imgBytes := readTestImage(t, "TEST_IMAGE_PATH")
	svc, _, photoRepo, storage, ownerID, toolID := newTestService(t)
	// URL invalida a proposito para forzar el error de PredictCondition.
	t.Setenv("ML_SERVICE_URL", "http://127.0.0.1:1")

	updated, err := svc.UploadPhoto(context.Background(), toolports.UploadPhotoInput{
		ToolID:      toolID,
		OwnerID:     ownerID,
		Filename:    "angulo1.jpg",
		Content:     bytes.NewReader(imgBytes),
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("UploadPhoto no deberia fallar aunque la CNN no responda: %v", err)
	}
	if storage.uploadCalls != 1 {
		t.Fatalf("la foto deberia subirse igual aunque la CNN falle, uploadCalls=%d", storage.uploadCalls)
	}
	if len(photoRepo.photos) != 1 {
		t.Fatalf("la foto deberia guardarse igual (con el score anterior) aunque la CNN falle")
	}
	if updated.ConditionScore != 0.70 {
		t.Fatalf("ConditionScore deberia quedar en el valor previo (0.70) cuando la CNN falla, quedo=%f", updated.ConditionScore)
	}
}
