package userhandler

import (
	"time"

	userdomain "github.com/yourusername/tool-inventory-api/internal/user/domain"
	userports "github.com/yourusername/tool-inventory-api/internal/user/ports"
)

type RegisterRequest struct {
	Name     string          `json:"name"     binding:"required,min=2,max=100"  example:"María López"`
	Email    string          `json:"email"    binding:"required,email"           example:"maria@ejemplo.com"`
	Password string          `json:"password" binding:"required,min=8"           example:"segura123"`
	Role     userdomain.Role `json:"role"     binding:"required,oneof=owner requester" example:"owner"`
	Phone    string          `json:"phone"    binding:"required"                 example:"5512345678"`
	INE      string          `json:"ine"      binding:"required"                 example:"PRRLSS85010212H700"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email" example:"maria@ejemplo.com"`
	Password string `json:"password" binding:"required"       example:"segura123"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IsPro     bool   `json:"is_pro"`
	Phone     string `json:"phone"`
	INE       string `json:"ine"`
	CreatedAt string `json:"created_at"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type SubscribePreferenceResponse struct {
	InitPoint    string `json:"init_point"`
	PreferenceID string `json:"preference_id"`
}

type ConfirmSubscriptionRequest struct {
	PaymentID string `json:"payment_id" binding:"required"`
}

func ToAuthResponse(out *userports.AuthOutput) AuthResponse {
	return AuthResponse{
		Token: out.Token,
		User:  ToUserResponse(out.User),
	}
}

type AddCardRequest struct {
	CardToken string `json:"card_token" binding:"required" example:"ff8080814c11e237014c1ff593b57b4"`
}

type SavedCardResponse struct {
	ID              string `json:"id"`
	CardBrand       string `json:"card_brand"`
	LastFourDigits  string `json:"last_four_digits"`
	ExpirationMonth int    `json:"expiration_month"`
	ExpirationYear  int    `json:"expiration_year"`
	CreatedAt       string `json:"created_at"`
}

func ToSavedCardResponse(c *userdomain.SavedCard) SavedCardResponse {
	return SavedCardResponse{
		ID:              c.ID.String(),
		CardBrand:       c.CardBrand,
		LastFourDigits:  c.LastFourDigits,
		ExpirationMonth: c.ExpirationMonth,
		ExpirationYear:  c.ExpirationYear,
		CreatedAt:       c.CreatedAt.Format(time.RFC3339),
	}
}

func ToSavedCardListResponse(cards []*userdomain.SavedCard) []SavedCardResponse {
	resp := make([]SavedCardResponse, 0, len(cards))
	for _, c := range cards {
		resp = append(resp, ToSavedCardResponse(c))
	}
	return resp
}

type BankAccountRequest struct {
	CLABE         string `json:"clabe"          binding:"required,len=18,numeric" example:"012345678901234567"`
	AccountHolder string `json:"account_holder" binding:"required,min=2,max=255"  example:"María López"`
	BankName      string `json:"bank_name"      binding:"required,min=2,max=100"  example:"BBVA"`
}

type BankAccountResponse struct {
	CLABE         string `json:"clabe"`
	AccountHolder string `json:"account_holder"`
	BankName      string `json:"bank_name"`
	Registered    bool   `json:"registered"`
}

func ToBankAccountResponse(a *userdomain.BankAccount) BankAccountResponse {
	if a == nil {
		return BankAccountResponse{}
	}
	return BankAccountResponse{
		CLABE:         a.CLABE,
		AccountHolder: a.AccountHolder,
		BankName:      a.BankName,
		Registered:    a.HasBankAccount(),
	}
}

func ToUserResponse(u *userdomain.User) UserResponse {
	return UserResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		Role:      string(u.Role),
		IsPro:     u.IsPro,
		Phone:     u.Phone,
		INE:       u.INE,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
