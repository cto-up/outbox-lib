package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cto-up/outbox-lib/pkg/db"
	"github.com/cto-up/outbox-lib/pkg/db/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const defaultMaxAttempts = 5

// Message is the caller-facing type for enqueuing an outbound HTTP call.
// Payload accepts any JSON-serialisable value; it is marshalled internally.
type Message struct {
	// EventType is a free-form label identifying what this call represents,
	// e.g. "trainer_session_feedback". Used for filtering in the admin UI.
	EventType string

	// TargetURL is the HTTP endpoint that will receive a POST with the payload.
	// It is persisted at enqueue time so retries are independent of config changes.
	TargetURL string

	// Payload is any JSON-serialisable value. It will be stored as jsonb.
	Payload any

	// TenantID is optional — used for filtering in the admin UI.
	TenantID string

	// CreatedBy is the user ID that triggered the outbound call (optional).
	CreatedBy string

	// MaxAttempts overrides the default of 5. Zero means use the default.
	MaxAttempts int
}

// Service provides the Enqueue / EnqueueTx surface for any module that needs
// reliable outbound HTTP delivery.
type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

// Enqueue persists a new outbox message outside any caller transaction.
// The message will be delivered asynchronously by the background dispatcher.
func (s *Service) Enqueue(ctx context.Context, msg Message) error {
	payload, err := marshalPayload(msg.Payload)
	if err != nil {
		return fmt.Errorf("outbox.Enqueue: marshal payload: %w", err)
	}
	_, err = s.store.CreateOutboxMessage(ctx, buildParams(msg, payload))
	return err
}

// EnqueueTx persists a new outbox message inside an existing pgx.Tx.
// Use this for atomic "write your data + enqueue the webhook in one transaction".
func (s *Service) EnqueueTx(ctx context.Context, tx pgx.Tx, msg Message) error {
	payload, err := marshalPayload(msg.Payload)
	if err != nil {
		return fmt.Errorf("outbox.EnqueueTx: marshal payload: %w", err)
	}
	qtx := s.store.Queries.WithTx(tx)
	_, err = qtx.CreateOutboxMessage(ctx, buildParams(msg, payload))
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func marshalPayload(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	switch p := v.(type) {
	case []byte:
		return p, nil
	case json.RawMessage:
		return p, nil
	default:
		return json.Marshal(v)
	}
}

func buildParams(msg Message, payload []byte) repository.CreateOutboxMessageParams {
	max := int32(msg.MaxAttempts)
	if max <= 0 {
		max = defaultMaxAttempts
	}
	return repository.CreateOutboxMessageParams{
		EventType:   msg.EventType,
		TargetUrl:   msg.TargetURL,
		Payload:     payload,
		MaxAttempts: max,
		TenantID:    pgtype.Text{String: msg.TenantID, Valid: msg.TenantID != ""},
		CreatedBy:   pgtype.Text{String: msg.CreatedBy, Valid: msg.CreatedBy != ""},
	}
}
