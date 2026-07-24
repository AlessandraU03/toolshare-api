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
		out.InsuranceClaim = s.buildInsuranceClaim(ctx, updated.ToolID, updated.OwnerID)
	}

	return out, nil
}

// buildInsuranceClaim calcula el pago del seguro que le corresponde al
// propietario (un % del valor estimado si la herramienta tiene seguro activo)
// y adjunta sus datos bancarios registrados para la transferencia manual.
// Devuelve nil si la herramienta no tiene seguro activo. Se comparte entre el
// dictamen (ResolveDispute) y la consulta posterior (GetInsuranceClaim), para
// que el admin pueda ver estos datos de forma persistente y no solo una vez.
func (s *adminService) buildInsuranceClaim(ctx context.Context, toolID, ownerID uuid.UUID) *rentalports.InsuranceClaimOutput {
	tool, err := s.toolRepo.FindByID(ctx, toolID)
	if err != nil {
		log.Printf("WARN [Admin]: no se pudo consultar herramienta %s para calcular seguro: %v", toolID, err)
		return nil
	}
	claim := tool.CalculateInsuranceClaim()
	if claim <= 0 {
		return nil
	}
	bankAccount, bankErr := s.userRepo.GetBankAccount(ctx, ownerID)
	if bankErr != nil {
		log.Printf("WARN [Admin]: no se pudo consultar datos bancarios del propietario %s: %v", ownerID, bankErr)
	}
	return &rentalports.InsuranceClaimOutput{Amount: claim, BankAccount: bankAccount}
}

// GetInsuranceClaim devuelve, de forma persistente, cuánto le debe el seguro
// al propietario de una renta y sus datos bancarios. Es lo que consulta el
// panel de admin para volver a ver esos datos después de dictaminar (antes
// solo se mostraban una vez al resolver la disputa). Devuelve nil si la
// herramienta no tenía seguro activo.
func (s *adminService) GetInsuranceClaim(ctx context.Context, rentalID uuid.UUID) (*rentalports.InsuranceClaimOutput, error) {
	rental, err := s.rentalRepo.FindByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	return s.buildInsuranceClaim(ctx, rental.ToolID, rental.OwnerID), nil
}
