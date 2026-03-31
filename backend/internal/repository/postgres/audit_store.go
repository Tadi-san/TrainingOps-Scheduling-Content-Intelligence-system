package postgres

import (
	"context"
	"encoding/json"

	"trainingops/internal/service"
)

func (s *Store) WriteAuditTransition(ctx context.Context, entry service.AuditEntry) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, entry.TenantID)
	if err != nil {
		return err
	}

	metadata, err := json.Marshal(map[string]any{
		"old_state": entry.OldState,
		"new_state": entry.NewState,
		"reason":    entry.Reason,
		"who":       entry.Who,
		"when":      entry.When,
	})
	if err != nil {
		return err
	}

	_, err = s.Pool.Exec(ctx, `
		INSERT INTO audit_logs (tenant_id, actor_user_id, action, entity_type, entity_id, metadata, created_at)
		VALUES (
			$1::uuid,
			NULLIF($2, '')::uuid,
			$3,
			$4,
			$5,
			$6::jsonb,
			$7
		)`,
		tenantUUID,
		entry.ActorUserID,
		entry.Action,
		entry.EntityType,
		entry.EntityID,
		string(metadata),
		entry.CreatedAt,
	)
	return err
}
