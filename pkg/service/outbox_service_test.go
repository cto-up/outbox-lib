package service

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestMarshalPayload(t *testing.T) {
	t.Run("nil returns empty object", func(t *testing.T) {
		b, err := marshalPayload(nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "{}" {
			t.Errorf("got %s, want {}", b)
		}
	})

	t.Run("[]byte passthrough", func(t *testing.T) {
		input := []byte(`{"key":"value"}`)
		b, err := marshalPayload(input)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(input) {
			t.Errorf("got %s, want %s", b, input)
		}
	})

	t.Run("json.RawMessage passthrough", func(t *testing.T) {
		input := json.RawMessage(`{"raw":true}`)
		b, err := marshalPayload(input)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(input) {
			t.Errorf("got %s, want %s", b, input)
		}
	})

	t.Run("struct is marshalled", func(t *testing.T) {
		type Foo struct {
			Name string `json:"name"`
		}
		b, err := marshalPayload(Foo{Name: "bar"})
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `{"name":"bar"}` {
			t.Errorf("got %s", b)
		}
	})
}

func TestBuildParams(t *testing.T) {
	t.Run("uses default max attempts when zero", func(t *testing.T) {
		p := buildParams(Message{MaxAttempts: 0}, []byte("{}"))
		if p.MaxAttempts != defaultMaxAttempts {
			t.Errorf("got %d, want %d", p.MaxAttempts, defaultMaxAttempts)
		}
	})

	t.Run("uses default max attempts when negative", func(t *testing.T) {
		p := buildParams(Message{MaxAttempts: -1}, []byte("{}"))
		if p.MaxAttempts != defaultMaxAttempts {
			t.Errorf("got %d, want %d", p.MaxAttempts, defaultMaxAttempts)
		}
	})

	t.Run("respects explicit max attempts", func(t *testing.T) {
		p := buildParams(Message{MaxAttempts: 10}, []byte("{}"))
		if p.MaxAttempts != 10 {
			t.Errorf("got %d, want 10", p.MaxAttempts)
		}
	})

	t.Run("optional fields set when provided", func(t *testing.T) {
		p := buildParams(Message{
			EventType: "test_event",
			TargetURL: "https://example.com",
			TenantID:  "tenant1",
			CreatedBy: "user1",
		}, []byte(`{"x":1}`))

		if p.EventType != "test_event" {
			t.Errorf("EventType: got %s", p.EventType)
		}
		if p.TargetUrl != "https://example.com" {
			t.Errorf("TargetUrl: got %s", p.TargetUrl)
		}
		if !p.TenantID.Valid || p.TenantID.String != "tenant1" {
			t.Errorf("TenantID: got %+v", p.TenantID)
		}
		if !p.CreatedBy.Valid || p.CreatedBy.String != "user1" {
			t.Errorf("CreatedBy: got %+v", p.CreatedBy)
		}
	})

	t.Run("optional fields not set when empty", func(t *testing.T) {
		p := buildParams(Message{}, []byte("{}"))
		if p.TenantID != (pgtype.Text{}) {
			t.Errorf("TenantID should be zero value, got %+v", p.TenantID)
		}
		if p.CreatedBy != (pgtype.Text{}) {
			t.Errorf("CreatedBy should be zero value, got %+v", p.CreatedBy)
		}
	})
}
