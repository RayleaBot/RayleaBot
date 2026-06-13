package app

type appHTTPHandlers struct {
	auth       *authHTTPHandlers
	management *managementHTTPHandlers
	tasks      *taskHTTPHandlers
	eventsWS   *eventsWSHandler
}
