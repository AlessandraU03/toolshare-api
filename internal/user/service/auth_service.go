package userservice

import (
	"context"
	"errors"
	"strings"

	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	apperrors "github.com/yourusername/tool-inventory-api/internal/shared/errors"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists = errors.New("el email ya está registrado")
	ErrInvalidCredentials = errors.New("credenciales inválidas")
)

type authService struct {
	userRepo      userports.UserRepository
	tokenProvider sharedports.TokenProvider
}

func NewAuthService(userRepo userports.UserRepository, tokenProvider sharedports.TokenProvider) userports.AuthService {
	return &authService{userRepo: userRepo, tokenProvider: tokenProvider}
}

func (s *authService) Register(ctx context.Context, inp userports.RegisterInput) (*userports.AuthOutput, error) {
	email := strings.ToLower(strings.TrimSpace(inp.Email))

	_, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil {
		return nil, ErrEmailAlreadyExists
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(inp.Password), 12)
	if err != nil {
		return nil, err
	}

	user := &userdomain.User{
		Name:     inp.Name,
		Email:    email,
		Password: string(hashed),
		Role:     inp.Role,
		Phone:    inp.Phone,
		INE:      inp.INE,
	}

	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.Generate(created.ID, created.Role)
	if err != nil {
		return nil, err
	}

	return &userports.AuthOutput{Token: token, User: created}, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*userports.AuthOutput, error) {
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

	return &userports.AuthOutput{Token: token, User: user}, nil
}
