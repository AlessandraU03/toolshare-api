package toolservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
)

var (
	ErrPlanLimitExceeded  = errors.New("Plan Gratuito superado (máximo 3 herramientas). Adquiere Plan Pro para publicar de forma ilimitada.")
	ErrValueLimitExceeded = errors.New("Plan Gratuito superado (valor máximo $1,500 MXN). Adquiere Plan Pro para publicar activos de mayor valor.")
)

type toolService struct {
	toolRepo    toolports.ToolRepository
	userRepo    userports.UserRepository
	fileStorage sharedports.FileStorage
}

func NewToolService(toolRepo toolports.ToolRepository, userRepo userports.UserRepository, fileStorage sharedports.FileStorage) toolports.ToolService {
	return &toolService{toolRepo: toolRepo, userRepo: userRepo, fileStorage: fileStorage}
}

func (s *toolService) Create(ctx context.Context, inp toolports.CreateToolInput) (*tooldomain.Tool, error) {
	// Verificar plan
	user, err := s.userRepo.FindByID(ctx, inp.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("obtener usuario propietario: %w", err)
	}

	if !user.IsPro {
		if inp.EstimatedValue > 1500 {
			return nil, ErrValueLimitExceeded
		}
		// Contar herramientas actuales
		existing, err := s.toolRepo.FindAll(ctx, toolports.ToolFilter{OwnerID: &inp.OwnerID})
		if err != nil {
			return nil, fmt.Errorf("listar herramientas existentes: %w", err)
		}
		if len(existing) >= 3 {
			return nil, ErrPlanLimitExceeded
		}
	}

	tool := &tooldomain.Tool{
		OwnerID:        inp.OwnerID,
		Name:           inp.Name,
		Description:    inp.Description,
		Category:       inp.Category,
		EstimatedValue: inp.EstimatedValue,
		DailyRate:      inp.DailyRate,
		Latitude:       inp.Latitude,
		Longitude:      inp.Longitude,
		IsAvailable:    true,
	}

	// Aplicar mínimo del 50% del valor en 30 días
	if minRate := tool.SuggestedDailyRate(); tool.DailyRate < minRate {
		tool.DailyRate = minRate
	}

	return s.toolRepo.Create(ctx, tool)
}

func (s *toolService) GetByID(ctx context.Context, id uuid.UUID) (*tooldomain.Tool, error) {
	return s.toolRepo.FindByID(ctx, id)
}

func (s *toolService) List(ctx context.Context, filter toolports.ToolFilter) ([]*tooldomain.Tool, error) {
	return s.toolRepo.FindAll(ctx, filter)
}

func (s *toolService) Update(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, inp toolports.UpdateToolInput) (*tooldomain.Tool, error) {
	tool, err := s.toolRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != ownerID {
		return nil, apperrors.ErrForbidden
	}

	// Verificar plan si cambia el valor estimado
	if inp.EstimatedValue != nil && *inp.EstimatedValue > 1500 {
		user, err := s.userRepo.FindByID(ctx, ownerID)
		if err != nil {
			return nil, fmt.Errorf("obtener usuario: %w", err)
		}
		if !user.IsPro {
			return nil, ErrValueLimitExceeded
		}
	}

	if inp.Name != nil {
		tool.Name = *inp.Name
	}
	if inp.Description != nil {
		tool.Description = *inp.Description
	}
	if inp.Category != nil {
		tool.Category = *inp.Category
	}
	if inp.EstimatedValue != nil {
		tool.EstimatedValue = *inp.EstimatedValue
	}
	if inp.DailyRate != nil {
		tool.DailyRate = *inp.DailyRate
		if minRate := tool.SuggestedDailyRate(); tool.DailyRate < minRate {
			tool.DailyRate = minRate
		}
	}
	if inp.Latitude != nil {
		tool.Latitude = *inp.Latitude
	}
	if inp.Longitude != nil {
		tool.Longitude = *inp.Longitude
	}
	if inp.IsAvailable != nil {
		tool.IsAvailable = *inp.IsAvailable
	}

	return s.toolRepo.Update(ctx, tool)
}

func (s *toolService) Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	tool, err := s.toolRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if tool.OwnerID != ownerID {
		return apperrors.ErrForbidden
	}
	return s.toolRepo.Delete(ctx, id)
}

func (s *toolService) UploadPhoto(ctx context.Context, inp toolports.UploadPhotoInput) (*tooldomain.Tool, error) {
	tool, err := s.toolRepo.FindByID(ctx, inp.ToolID)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != inp.OwnerID {
		return nil, apperrors.ErrForbidden
	}

	filename := fmt.Sprintf("tools/%s_%d_%s", inp.ToolID.String(), time.Now().Unix(), inp.Filename)
	url, err := s.fileStorage.Upload(ctx, filename, inp.Content, inp.ContentType)
	if err != nil {
		return nil, err
	}

	tool.PhotoURL = url
	return s.toolRepo.Update(ctx, tool)
}

type pythonPricingResponse struct {
	ValorReal           float64 `json:"valor_real_depreciado"`
	TopeGarantia        float64 `json:"tope_cobertura_garantia"`
	PrecioRentaSugerido float64 `json:"precio_renta_sugerido"`
}

func (s *toolService) GetPricingSuggestion(ctx context.Context, estimatedValue float64, scoreCondicion float64, category string, brand string) *toolports.PricingSuggestion {
	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := fmt.Sprintf("%s/suggest-price?precio_base=%f&score_condicion=%f&sector=%s&marca=%s",
		mlBaseURL,
		estimatedValue,
		scoreCondicion,
		url.QueryEscape(category),
		url.QueryEscape(brand),
	)

	// Crear cliente HTTP con timeout corto (1 segundo) para evitar colgar la API Go
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)

	// Fallback por defecto si hay error en la red o servicio abajo (cálculo original)
	fallback := func() *toolports.PricingSuggestion {
		minDaily := estimatedValue * 0.5 / 30
		return &toolports.PricingSuggestion{
			EstimatedValue: estimatedValue,
			SuggestedDaily: minDaily,
			MinimumDaily:   minDaily,
			Description:    "Precio mínimo sugerido (Fallback local): recuperar el 50% del valor en 30 días de renta",
		}
	}

	if err != nil {
		return fallback()
	}

	resp, err := client.Do(req)
	if err != nil {
		return fallback()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fallback()
	}

	var apiResp pythonPricingResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fallback()
	}

	return &toolports.PricingSuggestion{
		EstimatedValue: apiResp.ValorReal,
		SuggestedDaily: apiResp.PrecioRentaSugerido,
		MinimumDaily:   apiResp.PrecioRentaSugerido,
		Description:    fmt.Sprintf("Precio sugerido por Inteligencia Artificial (Random Forest). Garantía máxima: $%.2f MXN", apiResp.TopeGarantia),
	}
}

func (s *toolService) PredictCondition(ctx context.Context, filename string, content io.Reader, contentType string) (*toolports.PredictConditionOutput, error) {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	fileWriter, err := bodyWriter.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("crear campo file en multipart: %w", err)
	}

	if _, err := io.Copy(fileWriter, content); err != nil {
		return nil, fmt.Errorf("copiar archivo a multipart: %w", err)
	}

	bodyWriter.Close()

	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := mlBaseURL + "/predict-condition"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bodyBuf)
	if err != nil {
		return nil, fmt.Errorf("crear request a ML: %w", err)
	}

	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ejecutar request a ML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ML respondió con error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ClasePredicha  string             `json:"clase_predicha"`
		ScoreCondicion float64            `json:"score_condicion"`
		Probabilidades map[string]float64 `json:"probabilidades"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodificar respuesta de ML: %w", err)
	}

	return &toolports.PredictConditionOutput{
		ClasePredicha:  result.ClasePredicha,
		ScoreCondicion: result.ScoreCondicion,
	}, nil
}

func (s *toolService) AutoValuate(ctx context.Context, name string, scoreCondicion float64, category string, brand string) (*toolports.AutoValuateOutput, error) {
	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := fmt.Sprintf("%s/auto-valuate?nombre_herramienta=%s&score_condicion=%f&sector=%s&marca=%s",
		mlBaseURL,
		url.QueryEscape(name),
		scoreCondicion,
		url.QueryEscape(category),
		url.QueryEscape(brand),
	)

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("crear request auto-valuate: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ejecutar request auto-valuate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ML auto-valuate respondió con error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		PrecioBaseMercado   float64 `json:"precio_base_mercado"`
		PrecioRentaSugerido float64 `json:"precio_renta_sugerido"`
		PrecioRentaMinimo   float64 `json:"precio_renta_minimo"`
		DetallesCalculo     struct {
			ModeloMlUtilizado string `json:"modelo_ml_utilizado"`
		} `json:"detalles_calculo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodificar respuesta de ML auto-valuate: %w", err)
	}

	return &toolports.AutoValuateOutput{
		EstimatedValue: result.PrecioBaseMercado,
		SuggestedDaily: result.PrecioRentaSugerido,
		MinimumDaily:   result.PrecioRentaMinimo,
		Description:    fmt.Sprintf("Precio sugerido por Inteligencia Artificial (Mercado Libre API + Regresión). Modelo: %s", result.DetallesCalculo.ModeloMlUtilizado),
	}, nil
}
