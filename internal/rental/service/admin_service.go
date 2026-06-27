package rentalservice

import (
	"context"
	"log"

	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

type adminService struct {
	rentalRepo      rentalports.RentalRepository
	toolRepo        toolports.ToolRepository
	paymentProvider sharedports.PaymentProvider
}

func NewAdminService(
	rentalRepo rentalports.RentalRepository,
	toolRepo toolports.ToolRepository,
	paymentProvider sharedports.PaymentProvider,
) rentalports.AdminService {
	return &adminService{
		rentalRepo:      rentalRepo,
		toolRepo:        toolRepo,
		paymentProvider: paymentProvider,
	}
}

func (s *adminService) GetStats(ctx context.Context) (*rentalports.AdminStatsOutput, error) {
	totalTools := 0
	tools, err := s.toolRepo.FindAll(ctx, toolports.ToolFilter{})
	if err == nil {
		totalTools = len(tools)
	}

	totalRentals, activeRentals, disputedRentals, frozenFunds, err := s.rentalRepo.GetAdminStats(ctx)
	if err != nil {
		return nil, err
	}

	return &rentalports.AdminStatsOutput{
		TotalTools:      totalTools,
		TotalRentals:    totalRentals,
		ActiveRentals:   activeRentals,
		DisputedRentals: disputedRentals,
		FrozenFunds:     frozenFunds,
	}, nil
}

func (s *adminService) ListRentals(ctx context.Context, status string) ([]*rentaldomain.Rental, error) {
	return s.rentalRepo.FindAll(ctx, status)
}

func (s *adminService) ResolveDispute(ctx context.Context, inp rentalports.ResolveDisputeInput) (*rentaldomain.Rental, error) {
	rental, err := s.rentalRepo.FindByID(ctx, inp.RentalID)
	if err != nil {
		return nil, err
	}

	if err := rental.ResolveDispute(inp.Action, inp.Notes); err != nil {
		return nil, err
	}

	updated, err := s.rentalRepo.Update(ctx, rental)
	if err != nil {
		return nil, err
	}

	// Restaurar disponibilidad de la herramienta
	_ = s.toolRepo.SetAvailability(ctx, updated.ToolID, true)

	// Dictaminar en Mercado Pago
	if s.paymentProvider != nil && updated.MPPaymentID != "" {
		if inp.Action == "capture" {
			// Capturar el deducible a favor de la plataforma/propietario
			if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, updated.DeductibleAmount); err != nil {
				log.Printf("WARN [Admin]: no se pudo capturar pago %s: %v", updated.MPPaymentID, err)
			} else {
				updated.PaymentStatus = "captured_admin"
				updated, _ = s.rentalRepo.Update(ctx, updated)
			}
		} else if inp.Action == "refund" {
			// Cancelar pre-autorización para devolver dinero al solicitante
			if err := s.paymentProvider.Cancel(ctx, updated.MPPaymentID); err != nil {
				log.Printf("WARN [Admin]: no se pudo cancelar pago %s: %v", updated.MPPaymentID, err)
			} else {
				updated.PaymentStatus = "refunded_admin"
				updated, _ = s.rentalRepo.Update(ctx, updated)
			}
		}
	}

	return updated, nil
}
