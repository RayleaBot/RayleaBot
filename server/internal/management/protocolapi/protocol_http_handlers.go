package protocolapi

type ProtocolHandlers struct {
	protocol protocolHTTPService
}

func NewProtocolHandlers(protocol protocolHTTPService) *ProtocolHandlers {
	return &ProtocolHandlers{protocol: protocol}
}
