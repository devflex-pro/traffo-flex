package mongostore

import (
	"context"
	"testing"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestOwnerFilterScopesDocumentAccess(t *testing.T) {
	ctx := scope.WithValue(
		context.Background(),
		scope.Value{ActorID: "usr_admin", OwnerID: "usr_owner"},
	)
	filter := ownerFilter(
		ctx,
		bson.M{"id": "cmp_1"},
	)
	if filter["owner_id"] != "usr_owner" || filter["id"] != "cmp_1" {
		t.Fatalf(
			"filter = %v",
			filter,
		)
	}
}
