package rentalservice

import (
	"context"
	"log"

	rentaldomain "github.com/yourusername/tool-inventory-api/internal/rental/domain"
	rentalports "github.com/yourusername/tool-inventory-api/internal/rental/ports"
	sharedports "github.com/yourusername/tool-inventory-api/internal/shared/ports"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
)

type adminService struct {
	rentalRepo      rentalports.RentalRepository
	toolRepo        toolports.ToolRepository
	userRepo        userports.UserRepository
	paymentProvider sharedports.PaymentProvider
}

func NewAdminService(
	rentalRepo rentalports.RentalRepository,
	toolRepo toolports.ToolRepository,
	userRepo userports.UserRepository,
	paymentProvider sharedports.PaymentProvider,
) rentalports.AdminService {
	return &adminService{
		rentalRepo:      rentalRepo,
		toolRepo:        toolRepo,
		userRepo:        userRepo,
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

func (s *adminService) ResolveDispute(ctx context.Context, inp rentalports.ResolveDisputeInput) (*rentalports.ResolveDisputeOutput, error) {
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
		sellerToken, tokenErr := resolveSellerAccessToken(ctx, s.userRepo, s.paymentProvider, updated.OwnerID)
		if tokenErr != nil {
			log.Printf("WARN [Admin]: no se pudo obtener token del propietario para dictaminar pago %s: %v", updated.MPPaymentID, tokenErr)
		} else if inp.Action == "capture" {
			// Capturar el deducible a favor de la plataforma/propietario
			if err := s.paymentProvider.Capture(ctx, updated.MPPaymentID, updated.DeductibleAmount, sellerToken); err != nil {
				log.Printf("WARN [Admin]: no se pudo capturar pago %s: %v", updated.MPPaymentID, err)
			} else {
				updated.PaymentStatus = "captured_admin"
				updated, _ = s.rentalRepo.Update(ctx, updated)
			}
		} else if inp.Action == "refund" {
			// Cancelar pre-autorización para devolver dinero al solicitante
			if err := s.paymentProvider.Cancel(ctx, updated.MPPaymentID, sellerToken); err != nil {
				log.Printf("WARN [Admin]: no se pudo cancelar pago %s: %v", updated.MPPaymentID, err)
			} else {
				updated.PaymentStatus = "refunded_admin"
				updated, _ = s.rentalRepo.Update(ctx, updated)
			}
		}
	}

	out := &rentalports.ResolveDisputeOutput{Rental: updated}

	// Si el propietario ganó la disputa y la herramienta tiene el seguro
	// ToolShare activo, el seguro le cubre un 30% adicional del valor
	// estimado. No hay forma de transferirlo automático vía Mercado Pago
	// (ver nota en ResolveDisputeOutput), así que se le devuelven al
	// administrador el monto y los datos bancarios del propietario para que
	// haga la transferencia manual.
	if inp.Action == "capture" {
		tool, toolErr := s.toolRepo.FindByID(ctx, updated.ToolID)
		if toolErr != nil {
			log.Printf("WARN [Admin]: no se pudo consultar herramienta %s para calcular seguro: %v", updated.ToolID, toolErr)
		} else if claim := tool.CalculateInsuranceClaim(); claim > 0 {
			bankAccount, bankErr := s.userRepo.GetBankAccount(ctx, updated.OwnerID)
			if bankErr != nil {
				log.Printf("WARN [Admin]: no se pudo consultar datos bancarios del propietario %s: %v", updated.OwnerID, bankErr)
			}
			out.InsuranceClaim = &rentalports.InsuranceClaimOutput{
				Amount:      claim,
				BankAccount: bankAccount,
			}
		}
	}

	return out, nil
}
