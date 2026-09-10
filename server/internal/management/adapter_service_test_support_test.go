package management

import (
	"testing"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
)

func newAdapterTestService(t *testing.T, source adapterservice.ConfigSource, instances adapterservice.Instances) *adapterservice.Service {
	t.Helper()
	service, err := adapterservice.NewService(source, instances)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
