package rentalservice

import (
	"context"
	"log"

	"github.com/google/uuid"
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
		claim, claimErr := s.GetInsuranceClaim(ctx, updated.ID)
		if claimErr != nil {
			log.Printf("WARN [Admin]: no se pudo calcular el reclamo de seguro para la renta %s: %v", updated.ID, claimErr)
		} else if claim.Amount > 0 {
			out.InsuranceClaim = claim
		}
	}

	return out, nil
}

// GetInsuranceClaim recalcula lo que el seguro ToolShare le cubre al
// propietario de una renta (30% del valor estimado de la herramienta si
// tiene el seguro activo) junto con sus datos bancarios registrados. A
// diferencia del InsuranceClaim que devuelve ResolveDispute (que solo se ve
// una vez, en la respuesta de esa llamada), este método se puede invocar las
// veces que el administrador necesite para volver a consultar esos datos.
func (s *adminService) GetInsuranceClaim(ctx context.Context, rentalID uuid.UUID) (*rentalports.InsuranceClaimOutput, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	tool, err := s.toolRepo.FindByID(ctx, rental.ToolID)
	if err != nil {
		return nil, err
	}

	claim := tool.CalculateInsuranceClaim()
	bankAccount, bankErr := s.userRepo.GetBankAccount(ctx, rental.OwnerID)
	if bankErr != nil {
		log.Printf("WARN [Admin]: no se pudo consultar datos bancarios del propietario %s: %v", rental.OwnerID, bankErr)
	}

	return &rentalports.InsuranceClaimOutput{
		Amount:      claim,
		BankAccount: bankAccount,
	}, nil
}
