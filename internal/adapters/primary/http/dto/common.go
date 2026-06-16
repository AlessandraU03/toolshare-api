package dto

// ErrResponse es la estructura estándar de error en todos los endpoints.
type ErrResponse struct {
	Error string `json:"error" example:"mensaje de error"`
}

// MsgResponse es la respuesta de éxito con solo un mensaje.
type MsgResponse struct {
	Message string `json:"message" example:"operación exitosa"`
}
