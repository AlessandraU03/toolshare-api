package errors

import (
	"errors"
)

var (
	ErrNotFound  = errors.New("registro no encontrado")
	ErrForbidden = errors.New("acceso no autorizado")
)
