package conversation

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

type deliveryKey struct{}
type DeliveryTarget struct {
	Owner     Owner
	Reference chatevent.SessionRef
}

// WithDelivery carries one already selected route through the ordinary bridge
// accounting path. It is not accepted from adapter payloads.
func WithDelivery(ctx context.Context, owner Owner, ref chatevent.SessionRef) context.Context {
	return context.WithValue(ctx, deliveryKey{}, DeliveryTarget{Owner: owner, Reference: ref})
}
func DeliveryFromContext(ctx context.Context) (DeliveryTarget, bool) {
	target, ok := ctx.Value(deliveryKey{}).(DeliveryTarget)
	return target, ok
}
