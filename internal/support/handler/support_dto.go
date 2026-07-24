package supporthandler

import (
	"time"

	supportdomain "github.com/yourusername/tool-inventory-api/internal/support/domain"
	supportports "github.com/yourusername/tool-inventory-api/internal/support/ports"
)

type SendSupportMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

type SupportMessageResponse struct {
	ID        string `json:"id"`
	OwnerID   string `json:"owner_id"`
	SenderID  string `json:"sender_id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func ToSupportMessageResponse(m *supportdomain.SupportMessage) SupportMessageResponse {
	return SupportMessageResponse{
		ID:        m.ID.String(),
		OwnerID:   m.OwnerID.String(),
		SenderID:  m.SenderID.String(),
		Message:   m.Message,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
}

func ToSupportMessageListResponse(msgs []*supportdomain.SupportMessage) []SupportMessageResponse {
	out := make([]SupportMessageResponse, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ToSupportMessageResponse(m))
	}
	return out
}

type SupportThreadResponse struct {
	OwnerID       string `json:"owner_id"`
	OwnerName     string `json:"owner_name"`
	LastMessage   string `json:"last_message"`
	LastMessageAt string `json:"last_message_at"`
}

func ToSupportThreadListResponse(threads []supportports.ThreadSummary) []SupportThreadResponse {
	out := make([]SupportThreadResponse, 0, len(threads))
	for _, t := range threads {
		out = append(out, SupportThreadResponse{
			OwnerID:       t.OwnerID.String(),
			OwnerName:     t.OwnerName,
			LastMessage:   t.LastMessage,
			LastMessageAt: t.LastMessageAt.Format(time.RFC3339),
		})
	}
	return out
}
