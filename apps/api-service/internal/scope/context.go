package scope

import "context"

type key struct{}

type Value struct {
	ActorID string
	OwnerID string
}

func WithValue(ctx context.Context, value Value) context.Context {
	return context.WithValue(ctx, key{}, value)
}

func FromContext(ctx context.Context) (Value, bool) {
	value, ok := ctx.Value(key{}).(Value)
	return value, ok
}

func OwnerID(ctx context.Context) string {
	value, ok := FromContext(ctx)
	if !ok {
		return ""
	}
	return value.OwnerID
}
