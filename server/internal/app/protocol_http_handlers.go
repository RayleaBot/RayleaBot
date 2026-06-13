package app

type protocolHTTPHandlers struct {
	protocol protocolHTTPService
}

func newProtocolHTTPHandlers(protocol protocolHTTPService) *protocolHTTPHandlers {
	return &protocolHTTPHandlers{protocol: protocol}
}
