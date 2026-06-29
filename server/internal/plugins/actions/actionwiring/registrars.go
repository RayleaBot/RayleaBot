package actionwiring

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/defaultmodules"
)

func DefaultRegistrars() []actions.Registrar {
	return defaultmodules.Registrars()
}
