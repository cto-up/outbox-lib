package api

import (
	"errors"
	"net/http"

	"ctoup.com/coreapp/api/helpers"
	"ctoup.com/coreapp/pkg/shared/auth"
	api "github.com/cto-up/outbox-lib/api/openapi"
	"github.com/cto-up/outbox-lib/pkg/db"
	"github.com/cto-up/outbox-lib/pkg/db/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oapi-codegen/runtime/types"
)

type OutboxHandler struct {
	store *db.Store
}

func NewOutboxHandler(store *db.Store) *OutboxHandler {
	return &OutboxHandler{store: store}
}

func RegisterHandler(store *db.Store, options api.GinServerOptions, router *gin.Engine) {
	h := NewOutboxHandler(store)
	api.RegisterHandlersWithOptions(router, h, options)
}

// ListOutboxMessages implements api.ServerInterface.
// SUPER_ADMIN sees all tenants; ADMIN sees only their own.
func (h *OutboxHandler) ListOutboxMessages(c *gin.Context, params api.ListOutboxMessagesParams) {

	tenantID, _ := c.Get(auth.AUTH_TENANT_ID_KEY)
	tid := tenantID.(string)

	// Build the tenant filter: SUPER_ADMIN can override it (or request all-tenants via nil).
	// If caller passes ?tenant_id=X and is SUPER_ADMIN → use that.
	// Otherwise → scope to their own tenant (unless SUPER_ADMIN with no filter → all).
	tenantFilter := pgtype.Text{}
	if auth.IsSuperAdmin(c) {
		if params.TenantId != nil && *params.TenantId != "" {
			tenantFilter = pgtype.Text{String: *params.TenantId, Valid: true}
		}
		// else: leave invalid = no filter = all tenants
	} else {
		tenantFilter = pgtype.Text{String: tid, Valid: true}
	}

	statusFilter := pgtype.Text{}
	if params.Status != nil {
		statusFilter = pgtype.Text{String: string(*params.Status), Valid: true}
	}

	eventTypeFilter := pgtype.Text{}
	if params.EventType != nil && *params.EventType != "" {
		eventTypeFilter = pgtype.Text{String: *params.EventType, Valid: true}
	}

	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}
	pageSize := 50
	if params.PageSize != nil && *params.PageSize > 0 {
		pageSize = *params.PageSize
	}
	offset := (page - 1) * pageSize

	rows, err := h.store.ListOutboxMessages(c, repository.ListOutboxMessagesParams{
		TenantID:  tenantFilter,
		Status:    statusFilter,
		EventType: eventTypeFilter,
		Limit:     int32(pageSize),
		Offset:    int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, helpers.ErrorResponse(err))
		return
	}

	result := make([]api.OutboxMessage, len(rows))
	for i, row := range rows {
		result[i] = mapToDTO(row)
	}
	c.JSON(http.StatusOK, result)
}

// RetryOutboxMessage implements api.ServerInterface.
func (h *OutboxHandler) RetryOutboxMessage(c *gin.Context, id types.UUID) {
	row, err := h.store.ResetOutboxMessage(c, id)
	if err != nil {
		if errors.Is(err, noRows) {
			c.JSON(http.StatusNotFound, helpers.ErrorResponse(err))
			return
		}
		c.JSON(http.StatusInternalServerError, helpers.ErrorResponse(err))
		return
	}

	c.JSON(http.StatusOK, mapToDTO(row))
}

// DeleteOutboxMessage implements api.ServerInterface.
func (h *OutboxHandler) DeleteOutboxMessage(c *gin.Context, id types.UUID) {

	if err := h.store.DeleteOutboxMessage(c, id); err != nil {
		c.JSON(http.StatusInternalServerError, helpers.ErrorResponse(err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// pgx returns pgx.ErrNoRows; import it via the error variable.
var noRows = errors.New("no rows in result set")

func mapToDTO(row repository.OutbOutboxMessage) api.OutboxMessage {
	dto := api.OutboxMessage{
		Id:          row.ID,
		EventType:   row.EventType,
		TargetUrl:   row.TargetUrl,
		Status:      api.OutboxMessageStatus(row.Status),
		Attempts:    int(row.Attempts),
		MaxAttempts: int(row.MaxAttempts),
		NextRetryAt: row.NextRetryAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.LastError.Valid {
		dto.LastError = &row.LastError.String
	}
	if row.SentAt.Valid {
		t := row.SentAt.Time
		dto.SentAt = &t
	}
	if row.TenantID.Valid {
		dto.TenantId = &row.TenantID.String
	}
	if row.CreatedBy.Valid {
		dto.CreatedBy = &row.CreatedBy.String
	}
	return dto
}
