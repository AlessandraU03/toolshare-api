package rentalservice

import (
	"context"
	"errors"
	"fmt"
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

// ErrDisputeActionInvalid señala un valor de `action` distinto a "capture"/"refund".
var ErrDisputeActionInvalid = errors.New("acción de dictamen inválida: debe ser \"capture\" o \"refund\"")

// ErrDisputePaymentFailed señala que Mercado Pago rechazó la operación de
// cobro/reembolso del dictamen. La renta se deja en "disputed" (sin tocar su
// estado ni el del pago) para que el admin pueda reintentar, en vez de
// marcarla resuelta con el dinero sin moverse de verdad.
var ErrDisputePaymentFailed = errors.New("no se pudo procesar el pago en Mercado Pago para este dictamen")

func (s *adminService) ResolveDispute(ctx context.Context, inp rentalports.ResolveDisputeInput) (*rentaldomain.Rental, error) {
	if inp.Action != "capture" && inp.Action != "refund" {
		return nil, ErrDisputeActionInvalid
	}

	rental, err := s.rentalRepo.FindByID(ctx, inp.RentalID)
	if err != nil {
		return nil, err
	}
	if rental.Status != rentaldomain.RentalStatusDisputed {
		return nil, errors.New("la renta no está en disputa")
	}

	// El dictamen en Mercado Pago se resuelve ANTES de tocar la base de
	// datos: si el cobro/reembolso falla, la renta se queda tal cual
	// (sigue en "disputed") en vez de marcarse resuelta con el dinero sin
	// moverse — así el admin puede ver el error y reintentar.
	newPaymentStatus := rental.PaymentStatus
	if s.paymentProvider != nil && rental.MPPaymentID != "" {
		sellerToken, tokenErr := resolveSellerAccessToken(ctx, s.userRepo, s.paymentProvider, rental.OwnerID)
		if tokenErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrDisputePaymentFailed, tokenErr)
		}

		if inp.Action == "capture" {
			// El depósito ya fue capturado como penalización al reportar la
			// disputa (ver RentalService.Dispute); aquí solo se confirma el
			// dictamen a favor del propietario, sin volver a cobrar.
			newPaymentStatus = "captured_admin"
		} else {
			// inp.Action == "refund": el depósito ya está capturado
			// (aprobado) para este punto, así que hay que reembolsarlo de
			// verdad — Cancel() no sirve porque solo libera
			// pre-autorizaciones sin capturar.
			if err := s.paymentProvider.Refund(ctx, rental.MPPaymentID, rental.DeductibleAmount, sellerToken); err != nil {
				log.Printf("ERROR [Admin]: no se pudo reembolsar pago %s: %v", rental.MPPaymentID, err)
				return nil, fmt.Errorf("%w: %v", ErrDisputePaymentFailed, err)
			}
			newPaymentStatus = "refunded_admin"
		}
	}

	if err := rental.ResolveDispute(inp.Action, inp.Notes); err != nil {
		return nil, err
	}
	rental.PaymentStatus = newPaymentStatus

	updated, err := s.rentalRepo.Update(ctx, rental)
	if err != nil {
		return nil, err
	}

	// Restaurar disponibilidad de la herramienta
	_ = s.toolRepo.SetAvailability(ctx, updated.ToolID, true)

	return updated, nil
}
