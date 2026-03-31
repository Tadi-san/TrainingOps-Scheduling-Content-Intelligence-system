package audit

import "context"

type contextKey string

const key contextKey = "audit_meta"

type Metadata struct {
	ActorUserID string
	Reason      string
	Who         string
	When        string
}

func WithMetadata(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, key, md)
}

func FromContext(ctx context.Context) (Metadata, bool) {
	md, ok := ctx.Value(key).(Metadata)
	return md, ok
}
