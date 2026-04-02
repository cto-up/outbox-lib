package api

import (
	"testing"
	"time"

	api "github.com/cto-up/outbox-lib/api/openapi"
	"github.com/cto-up/outbox-lib/pkg/db/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMapToDTO(t *testing.T) {
	id := uuid.New()
	now := time.Now().Truncate(time.Second)

	t.Run("minimal row maps correctly", func(t *testing.T) {
		row := repository.OutbOutboxMessage{
			ID:          id,
			EventType:   "test_event",
			TargetUrl:   "https://example.com",
			Status:      "pending",
			Attempts:    0,
			MaxAttempts: 5,
			NextRetryAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		dto := mapToDTO(row)

		if dto.Id != id {
			t.Errorf("Id: got %v, want %v", dto.Id, id)
		}
		if dto.EventType != "test_event" {
			t.Errorf("EventType: got %s", dto.EventType)
		}
		if dto.TargetUrl != "https://example.com" {
			t.Errorf("TargetUrl: got %s", dto.TargetUrl)
		}
		if dto.Status != api.Pending {
			t.Errorf("Status: got %s, want pending", dto.Status)
		}
		if dto.Attempts != 0 {
			t.Errorf("Attempts: got %d, want 0", dto.Attempts)
		}
		if dto.MaxAttempts != 5 {
			t.Errorf("MaxAttempts: got %d, want 5", dto.MaxAttempts)
		}
		if dto.LastError != nil {
			t.Errorf("LastError should be nil, got %v", dto.LastError)
		}
		if dto.SentAt != nil {
			t.Errorf("SentAt should be nil, got %v", dto.SentAt)
		}
		if dto.TenantId != nil {
			t.Errorf("TenantId should be nil, got %v", dto.TenantId)
		}
		if dto.CreatedBy != nil {
			t.Errorf("CreatedBy should be nil, got %v", dto.CreatedBy)
		}
	})

	t.Run("optional fields populated when valid", func(t *testing.T) {
		errMsg := "connection refused"
		sentAt := now.Add(-time.Minute)
		row := repository.OutbOutboxMessage{
			ID:          id,
			EventType:   "evt",
			TargetUrl:   "https://example.com",
			Status:      "failed",
			Attempts:    3,
			MaxAttempts: 5,
			NextRetryAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
			LastError:   pgtype.Text{String: errMsg, Valid: true},
			SentAt:      pgtype.Timestamptz{Time: sentAt, Valid: true},
			TenantID:    pgtype.Text{String: "tenant1", Valid: true},
			CreatedBy:   pgtype.Text{String: "user1", Valid: true},
		}
		dto := mapToDTO(row)

		if dto.LastError == nil || *dto.LastError != errMsg {
			t.Errorf("LastError: got %v, want %q", dto.LastError, errMsg)
		}
		if dto.SentAt == nil || !dto.SentAt.Equal(sentAt) {
			t.Errorf("SentAt: got %v, want %v", dto.SentAt, sentAt)
		}
		if dto.TenantId == nil || *dto.TenantId != "tenant1" {
			t.Errorf("TenantId: got %v, want tenant1", dto.TenantId)
		}
		if dto.CreatedBy == nil || *dto.CreatedBy != "user1" {
			t.Errorf("CreatedBy: got %v, want user1", dto.CreatedBy)
		}
		if dto.Status != api.Failed {
			t.Errorf("Status: got %s, want failed", dto.Status)
		}
		if dto.Attempts != 3 {
			t.Errorf("Attempts: got %d, want 3", dto.Attempts)
		}
	})
}
