package app

import (
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
)

type appHTTPHandlers struct {
	auth       *managementhttp.AuthHandlers
	management *managementhttp.ManagementHandlers
	tasks      *managementhttp.TaskHandlers
	eventsWS   *managementws.EventsHandler
}
