package supportpostgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	supportdomain "github.com/yourusername/tool-inventory-api/internal/support/domain"
	supportports "github.com/yourusername/tool-inventory-api/internal/support/ports"

	"github.com/google/uuid"
)

type SupportRepository struct {
	db *pgxpool.Pool
}

func NewSupportRepository(db *pgxpool.Pool) supportports.SupportRepository {
	return &SupportRepository{db: db}
}

func (r *SupportRepository) Create(ctx context.Context, msg *supportdomain.SupportMessage) (*supportdomain.SupportMessage, error) {
	query := `INSERT INTO support_messages (owner_id, sender_id, message) VALUES ($1, $2, $3) RETURNING id, owner_id, sender_id, message, created_at`
	m := &supportdomain.SupportMessage{}
	err := r.db.QueryRow(ctx, query, msg.OwnerID, msg.SenderID, msg.Message).
		Scan(&m.ID, &m.OwnerID, &m.SenderID, &m.Message, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("crear mensaje de soporte: %w", err)
	}
	return m, nil
}

func (r *SupportRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*supportdomain.SupportMessage, error) {
	query := `SELECT id, owner_id, sender_id, message, created_at FROM support_messages WHERE owner_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("consultar mensajes de soporte: %w", err)
	}
	defer rows.Close()

	var msgs []*supportdomain.SupportMessage
	for rows.Next() {
		m := &supportdomain.SupportMessage{}
		if err := rows.Scan(&m.ID, &m.OwnerID, &m.SenderID, &m.Message, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *SupportRepository) ListThreads(ctx context.Context) ([]supportports.ThreadSummary, error) {
	query := `
		SELECT u.id, u.name, m.message, m.created_at
		FROM users u
		JOIN LATERAL (
			SELECT message, created_at FROM support_messages sm
			WHERE sm.owner_id = u.id ORDER BY created_at DESC LIMIT 1
		) m ON true
		WHERE u.role = 'owner'
		ORDER BY m.created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listar hilos de soporte: %w", err)
	}
	defer rows.Close()

	var threads []supportports.ThreadSummary
	for rows.Next() {
		var t supportports.ThreadSummary
		if err := rows.Scan(&t.OwnerID, &t.OwnerName, &t.LastMessage, &t.LastMessageAt); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}
