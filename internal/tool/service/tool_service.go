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
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	tooldomain "github.com/yourusername/tool-inventory-api/internal/tool/domain"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
)

var (
	ErrPlanLimitExceeded    = errors.New("Plan Gratuito superado (máximo 3 herramientas). Adquiere Plan Pro para publicar de forma ilimitada.")
	ErrValueLimitExceeded   = errors.New("Plan Gratuito superado (valor máximo $1,500 MXN). Adquiere Plan Pro para publicar activos de mayor valor.")
	ErrInsurancePayment     = errors.New("no se pudo procesar el pago del seguro")
	ErrInsuranceNotApproved = errors.New("el pago del seguro aún no está aprobado")
	ErrInsuranceRefMismatch = errors.New("el pago no corresponde a esta herramienta")
)

// InsuranceExternalRefPrefix marca las external_reference de MP que corresponden
// al pago del seguro mensual de una herramienta (en vez de una renta o suscripción Pro).
const InsuranceExternalRefPrefix = "ins:"

type toolService struct {
	toolRepo        toolports.ToolRepository
	toolPhotoRepo   toolports.ToolPhotoRepository
	userRepo        userports.UserRepository
	fileStorage     sharedports.FileStorage
	paymentProvider sharedports.PaymentProvider
}

func NewToolService(toolRepo toolports.ToolRepository, toolPhotoRepo toolports.ToolPhotoRepository, userRepo userports.UserRepository, fileStorage sharedports.FileStorage, paymentProvider sharedports.PaymentProvider) toolports.ToolService {
	return &toolService{toolRepo: toolRepo, toolPhotoRepo: toolPhotoRepo, userRepo: userRepo, fileStorage: fileStorage, paymentProvider: paymentProvider}
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
		// No disponible hasta subir el minimo de fotos (MinRequiredPhotos):
		// UploadPhoto la activa automaticamente al cumplirse. Antes de este
		// cambio la herramienta quedaba publicada sin ninguna foto real.
		IsAvailable: false,
		Brand:       inp.Brand,
		AgeMonths:   inp.AgeMonths,
		City:        inp.City,
		State:       inp.State,
		ConditionScore: func() float64 {
			if inp.ConditionScore > 0 {
				return inp.ConditionScore
			}
			return 0.70
		}(),
		PriceSource: "catalogo_semilla",
		// El seguro solo se activa mediante un pago confirmado con Mercado Pago
		// (ver ConfirmInsurancePayment), nunca directamente al crear/editar.
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

func (s *toolService) GetPhotos(ctx context.Context, toolID uuid.UUID) ([]*tooldomain.ToolPhoto, error) {
	return s.toolPhotoRepo.FindByToolID(ctx, toolID)
}

func (s *toolService) List(ctx context.Context, filter toolports.ToolFilter) ([]*tooldomain.Tool, error) {
	if filter.Search != "" {
		matchedKeywords, err := s.callSemanticSearch(ctx, filter.Search)
		if err == nil && len(matchedKeywords) > 0 {
			var allTools []*tooldomain.Tool
			seenIDs := make(map[uuid.UUID]bool)

			for _, kw := range matchedKeywords {
				kwFilter := filter
				kwFilter.Search = kw

				results, err := s.toolRepo.FindAll(ctx, kwFilter)
				if err == nil {
					for _, t := range results {
						if !seenIDs[t.ID] {
							seenIDs[t.ID] = true
							allTools = append(allTools, t)
						}
					}
				}
			}
			if len(allTools) > 0 {
				return allTools, nil
			}
		}
	}
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
	// wants_insurance NO se modifica aquí: solo cambia vía ConfirmInsurancePayment.

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

// CreateInsurancePreference crea una preferencia de Checkout Pro en MP para
// que el propietario pague la prima mensual del seguro de esta herramienta.
func (s *toolService) CreateInsurancePreference(ctx context.Context, toolID uuid.UUID, ownerID uuid.UUID) (sharedports.CreatePreferenceOutput, error) {
	if s.paymentProvider == nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: pasarela de pagos no configurada", ErrInsurancePayment)
	}

	tool, err := s.toolRepo.FindByID(ctx, toolID)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, err
	}
	if tool.OwnerID != ownerID {
		return sharedports.CreatePreferenceOutput{}, apperrors.ErrForbidden
	}

	owner, err := s.userRepo.FindByID(ctx, ownerID)
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, err
	}

	premium := tool.EstimatedValue * tooldomain.InsuranceMonthlyRate

	notificationURL := os.Getenv("MP_NOTIFICATION_URL")
	backURL := os.Getenv("MP_BACK_URL")
	if backURL == "" {
		// auto_return de MP exige una URL absoluta http(s); "toolshare://" no
		// es válida y deja el botón "Pagar" inerte. El WebView de Flutter
		// intercepta esta ruta antes de que la navegación se complete.
		backURL = "https://toolshare-api.up.railway.app/payment"
	}

	out, err := s.paymentProvider.CreatePreference(ctx, sharedports.CreatePreferenceInput{
		Title:           fmt.Sprintf("Seguro mensual ToolShare — %s", tool.Name),
		TotalAmount:     premium,
		PayerEmail:      owner.Email,
		ExternalRef:     InsuranceExternalRefPrefix + toolID.String(),
		NotificationURL: notificationURL,
		BackURLSuccess:  backURL + "/success",
		BackURLFailure:  backURL + "/failure",
		BackURLPending:  backURL + "/pending",
	})
	if err != nil {
		return sharedports.CreatePreferenceOutput{}, fmt.Errorf("%w: %v", ErrInsurancePayment, err)
	}
	return out, nil
}

// ConfirmInsurancePayment verifica directamente con MP que el pago del seguro
// fue aprobado y corresponde a esta herramienta, y activa wants_insurance.
func (s *toolService) ConfirmInsurancePayment(ctx context.Context, toolID uuid.UUID, ownerID uuid.UUID, paymentID string) (*tooldomain.Tool, error) {
	if s.paymentProvider == nil {
		return nil, fmt.Errorf("%w: pasarela de pagos no configurada", ErrInsurancePayment)
	}

	info, err := s.paymentProvider.GetPaymentInfo(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInsurancePayment, err)
	}

	tool, err := s.toolRepo.FindByID(ctx, toolID)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != ownerID {
		return nil, apperrors.ErrForbidden
	}

	if info.ExternalRef != InsuranceExternalRefPrefix+toolID.String() {
		return nil, ErrInsuranceRefMismatch
	}
	if info.Status != "approved" {
		return nil, ErrInsuranceNotApproved
	}

	tool.WantsInsurance = true
	tool.InsuranceMonthlyPremium = tool.CalculateInsurancePremium()
	return s.toolRepo.Update(ctx, tool)
}

// CancelInsurance desactiva el seguro de la herramienta. No hay reembolso: la
// prima ya pagada cubre el mes en curso, la cobertura simplemente no se
// renueva en el siguiente ciclo.
func (s *toolService) CancelInsurance(ctx context.Context, toolID uuid.UUID, ownerID uuid.UUID) (*tooldomain.Tool, error) {
	tool, err := s.toolRepo.FindByID(ctx, toolID)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != ownerID {
		return nil, apperrors.ErrForbidden
	}

	tool.WantsInsurance = false
	tool.InsuranceMonthlyPremium = 0
	return s.toolRepo.Update(ctx, tool)
}

// MinRequiredPhotos es el minimo de fotos (en distintos angulos, verificadas
// por la CNN en el servidor) que debe tener una herramienta antes de
// publicarse como disponible. Con menos de esto, alcanza con que UNA sola
// foto oculte el desgaste real para inflar el precio — con el minimo y la
// regla del peor score (ver peorScore) hace falta que todas las fotos
// escondan el daño a la vez.
const MinRequiredPhotos = 2

func (s *toolService) UploadPhoto(ctx context.Context, inp toolports.UploadPhotoInput) (*tooldomain.Tool, error) {
	tool, err := s.toolRepo.FindByID(ctx, inp.ToolID)
	if err != nil {
		return nil, err
	}
	if tool.OwnerID != inp.OwnerID {
		return nil, apperrors.ErrForbidden
	}

	// Se lee el contenido completo una sola vez porque hace falta enviarlo
	// dos veces (storage + CNN) y un io.Reader solo se puede consumir una vez.
	content, err := io.ReadAll(inp.Content)
	if err != nil {
		return nil, fmt.Errorf("leer contenido de la foto: %w", err)
	}

	filename := fmt.Sprintf("tools/%s_%d_%s", inp.ToolID.String(), time.Now().Unix(), inp.Filename)
	url, err := s.fileStorage.Upload(ctx, filename, bytes.NewReader(content), inp.ContentType)
	if err != nil {
		return nil, err
	}

	// El score de condición de ESTA foto SIEMPRE se calcula aquí, sobre el
	// archivo real que se acaba de subir — nunca se confía en un
	// condition_score que el cliente haya podido mandar. Si el servicio de
	// ML no responde, se usa el score que ya tenía la herramienta (o el
	// default de creación) en vez de tumbar la subida por un problema ajeno
	// al usuario. Esta es la ÚNICA llamada a la CNN por foto: ya no hay una
	// llamada previa de "vista previa" del lado del cliente.
	photoScore := tool.ConditionScore
	if prediction, predErr := s.PredictCondition(ctx, inp.Filename, bytes.NewReader(content), inp.ContentType); predErr == nil {
		photoScore = prediction.ScoreCondicion
	}

	if _, err := s.toolPhotoRepo.Create(ctx, &tooldomain.ToolPhoto{
		ToolID:         inp.ToolID,
		PhotoURL:       url,
		ConditionScore: photoScore,
	}); err != nil {
		return nil, fmt.Errorf("guardar foto de la herramienta: %w", err)
	}

	photos, err := s.toolPhotoRepo.FindByToolID(ctx, inp.ToolID)
	if err != nil {
		return nil, fmt.Errorf("listar fotos de la herramienta: %w", err)
	}

	tool.PhotoURL = url // portada = ultima foto subida (Flutter aun no tiene galeria)
	tool.ConditionScore = peorScore(photos)

	// La herramienta solo queda disponible para renta una vez que cumple el
	// minimo de fotos verificadas por la CNN.
	if len(photos) >= MinRequiredPhotos {
		tool.IsAvailable = true
	}

	return s.toolRepo.Update(ctx, tool)
}

// peorScore devuelve el menor condition_score entre todas las fotos de una
// herramienta — el angulo que muestra mas desgaste manda, en vez de que un
// promedio lo diluya con fotos mas favorecedoras.
func peorScore(photos []*tooldomain.ToolPhoto) float64 {
	if len(photos) == 0 {
		return 0.70
	}
	peor := photos[0].ConditionScore
	for _, p := range photos[1:] {
		if p.ConditionScore < peor {
			peor = p.ConditionScore
		}
	}
	return peor
}

type pythonPricingResponse struct {
	ValorReal           float64 `json:"valor_real_depreciado"`
	TopeGarantia        float64 `json:"tope_cobertura_garantia"`
	DeducibleGarantia   float64 `json:"deducible_garantia_sugerido"`
	PrecioRentaSugerido float64 `json:"precio_renta_sugerido"`
}

func (s *toolService) GetPricingSuggestion(ctx context.Context, estimatedValue float64, scoreCondicion float64, category string, brand string, name string, ageMonths int) *toolports.PricingSuggestion {
	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := fmt.Sprintf("%s/suggest-price?precio_base=%f&score_condicion=%f&sector=%s&marca=%s&nombre_herramienta=%s&age_months=%d",
		mlBaseURL,
		estimatedValue,
		scoreCondicion,
		url.QueryEscape(category),
		url.QueryEscape(brand),
		url.QueryEscape(name),
		ageMonths,
	)

	// Crear cliente HTTP con timeout corto (1 segundo) para evitar colgar la API Go
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)

	// Fallback por defecto si hay error en la red o servicio abajo (cálculo original)
	fallback := func() *toolports.PricingSuggestion {
		minDaily := estimatedValue * 0.5 / 30
		return &toolports.PricingSuggestion{
			EstimatedValue:      estimatedValue,
			SuggestedDaily:      minDaily,
			MinimumDaily:        minDaily,
			GuaranteeCap:        estimatedValue,
			SuggestedDeductible: estimatedValue * 0.10,
			Description:         "Precio mínimo sugerido (Fallback local): recuperar el 50% del valor en 30 días de renta",
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
		EstimatedValue:      apiResp.ValorReal,
		SuggestedDaily:      apiResp.PrecioRentaSugerido,
		MinimumDaily:        apiResp.PrecioRentaSugerido,
		GuaranteeCap:        apiResp.TopeGarantia,
		SuggestedDeductible: apiResp.DeducibleGarantia,
		Description:         fmt.Sprintf("Precio sugerido por Inteligencia Artificial (Random Forest). Garantía máxima: $%.2f MXN", apiResp.TopeGarantia),
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

	// 30s (no 8s): la primera llamada despues de que el servicio de ML
	// arranca carga el modelo Keras en memoria desde disco, lo cual puede
	// tardar mas de 8s por si solo. Con un timeout corto esa primera
	// llamada fallaba en silencio y UploadPhoto se quedaba con el score
	// anterior sin que nada avisara del error. Las llamadas siguientes son
	// rapidas (~100-300ms) porque el modelo ya queda cacheado en memoria.
	client := &http.Client{Timeout: 30 * time.Second}
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

func (s *toolService) AutoValuate(ctx context.Context, name string, scoreCondicion float64, category string, brand string, ageMonths int, precioBaseManual *float64, ticketValidado bool) (*toolports.AutoValuateOutput, error) {
	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := fmt.Sprintf("%s/auto-valuate?nombre_herramienta=%s&score_condicion=%f&sector=%s&marca=%s&age_months=%d",
		mlBaseURL,
		url.QueryEscape(name),
		scoreCondicion,
		url.QueryEscape(category),
		url.QueryEscape(brand),
		ageMonths,
	)

	if precioBaseManual != nil {
		apiURL = fmt.Sprintf("%s&precio_base_manual=%f&ticket_validado=%t", apiURL, *precioBaseManual, ticketValidado)
	}

	mpToken := os.Getenv("MP_ACCESS_TOKEN")
	if mpToken != "" {
		apiURL = fmt.Sprintf("%s&access_token=%s", apiURL, url.QueryEscape(mpToken))
	}

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
		PrecioBaseMercado      float64 `json:"precio_base_mercado"`
		PrecioRentaSugerido    float64 `json:"precio_renta_sugerido"`
		PrecioRentaMinimo      float64 `json:"precio_renta_minimo"`
		RequiereRevisionManual bool    `json:"requiere_revision_manual"`
		DetallesCalculo        struct {
			ModeloMlUtilizado    string `json:"modelo_ml_utilizado"`
			FuentePrecioCatalogo string `json:"fuente_precio_catalogo"`
		} `json:"detalles_calculo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodificar respuesta de ML auto-valuate: %w", err)
	}

	return &toolports.AutoValuateOutput{
		EstimatedValue:       result.PrecioBaseMercado,
		SuggestedDaily:       result.PrecioRentaSugerido,
		MinimumDaily:         result.PrecioRentaMinimo,
		RequiresManualReview: result.RequiereRevisionManual,
		Description: fmt.Sprintf("Precio sugerido por minería de datos (fuente: %s). Modelo de renta: %s",
			result.DetallesCalculo.FuentePrecioCatalogo, result.DetallesCalculo.ModeloMlUtilizado),
	}, nil
}

func (s *toolService) ExtractTicketPrice(ctx context.Context, filename string, content io.Reader, contentType string) (*toolports.ExtractTicketPriceOutput, error) {
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
	apiURL := mlBaseURL + "/extract-ticket-price"
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
		Valid           bool    `json:"valid"`
		PrecioDetectado float64 `json:"precio_detectado"`
		Confianza       string  `json:"confianza"`
		Error           string  `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodificar respuesta de ML: %w", err)
	}

	return &toolports.ExtractTicketPriceOutput{
		Valid:         result.Valid,
		DetectedPrice: result.PrecioDetectado,
		Confidence:    result.Confianza,
		Error:         result.Error,
	}, nil
}

func (s *toolService) callSemanticSearch(ctx context.Context, query string) ([]string, error) {
	mlBaseURL := os.Getenv("ML_SERVICE_URL")
	if mlBaseURL == "" {
		mlBaseURL = "http://localhost:8000"
	}
	apiURL := fmt.Sprintf("%s/search?query=%s", mlBaseURL, url.QueryEscape(query))

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var result struct {
		Resultados []struct {
			Nombre string `json:"nombre"`
		} `json:"resultados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var keywords []string
	for _, item := range result.Resultados {
		if item.Nombre != "" {
			keywords = append(keywords, item.Nombre)
		}
	}
	return keywords, nil
}
