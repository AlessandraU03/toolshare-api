package supportservice

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	supportdomain "github.com/yourusername/tool-inventory-api/internal/support/domain"
	supportports "github.com/yourusername/tool-inventory-api/internal/support/ports"
	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
)

var (
	ErrEmptyMessage = errors.New("el mensaje no puede estar vacío")
	ErrNotAnOwner   = errors.New("el usuario indicado no es un propietario")
)

type supportService struct {
	supportRepo supportports.SupportRepository
	userRepo    userports.UserRepository
}

func NewSupportService(supportRepo supportports.SupportRepository, userRepo userports.UserRepository) supportports.SupportService {
	return &supportService{supportRepo: supportRepo, userRepo: userRepo}
}

func (s *supportService) GetOwnerThread(ctx context.Context, ownerID uuid.UUID) ([]*supportdomain.SupportMessage, error) {
	return s.supportRepo.ListByOwner(ctx, ownerID)
}

func (s *supportService) SendAsOwner(ctx context.Context, ownerID uuid.UUID, message string) (*supportdomain.SupportMessage, error) {
	return s.create(ctx, ownerID, ownerID, message)
}

func (s *supportService) GetThreadForAdmin(ctx context.Context, ownerID uuid.UUID) ([]*supportdomain.SupportMessage, error) {
	if err := s.verifyOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	return s.supportRepo.ListByOwner(ctx, ownerID)
}

func (s *supportService) SendAsAdmin(ctx context.Context, ownerID, adminID uuid.UUID, message string) (*supportdomain.SupportMessage, error) {
	if err := s.verifyOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	return s.create(ctx, ownerID, adminID, message)
}

func (s *supportService) ListThreads(ctx context.Context) ([]supportports.ThreadSummary, error) {
	return s.supportRepo.ListThreads(ctx)
}

func (s *supportService) verifyOwner(ctx context.Context, ownerID uuid.UUID) error {
	user, err := s.userRepo.FindByID(ctx, ownerID)
	if err != nil {
		return err
	}
	if user.Role != userdomain.RoleOwner {
		return ErrNotAnOwner
	}
	return nil
}

func (s *supportService) create(ctx context.Context, ownerID, senderID uuid.UUID, message string) (*supportdomain.SupportMessage, error) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return nil, ErrEmptyMessage
	}
	return s.supportRepo.Create(ctx, &supportdomain.SupportMessage{
		OwnerID:  ownerID,
		SenderID: senderID,
		Message:  trimmed,
	})
}
