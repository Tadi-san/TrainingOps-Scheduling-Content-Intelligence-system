package tenant

import "context"

type contextKey string

const key contextKey = "tenant_id"

func WithID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, key, tenantID)
}

func ID(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(key).(string)
	return value, ok && value != ""
}
