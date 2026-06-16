package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yourusername/tool-inventory-api/internal/core/domain"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/input"
	"github.com/yourusername/tool-inventory-api/internal/core/ports/output"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists = errors.New("el email ya está registrado")
	ErrInvalidCredentials = errors.New("credenciales inválidas")
)

type authService struct {
	userRepo      output.UserRepository
	tokenProvider output.TokenProvider
}

func NewAuthService(userRepo output.UserRepository, tokenProvider output.TokenProvider) input.AuthService {
	return &authService{userRepo: userRepo, tokenProvider: tokenProvider}
}

func (s *authService) Register(ctx context.Context, inp input.RegisterInput) (*input.AuthOutput, error) {
	email := strings.ToLower(strings.TrimSpace(inp.Email))

	_, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil {
		return nil, ErrEmailAlreadyExists
	}
	if !errors.Is(err, output.ErrNotFound) {
		return nil, err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(inp.Password), 12)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Name:     inp.Name,
		Email:    email,
		Password: string(hashed),
		Role:     inp.Role,
	}

	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.Generate(created.ID, created.Role)
	if err != nil {
		return nil, err
	}

	return &input.AuthOutput{Token: token, User: created}, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*input.AuthOutput, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenProvider.Generate(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	return &input.AuthOutput{Token: token, User: user}, nil
}
