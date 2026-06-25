package protocolapi

type ProtocolHandlers struct {
	protocol protocolHTTPService
}

type ModuleDeps struct {
	Protocol protocolHTTPService
}

func NewProtocolHandlers(protocol protocolHTTPService) *ProtocolHandlers {
	return &ProtocolHandlers{protocol: protocol}
}

func NewModule(deps ModuleDeps) *ProtocolHandlers {
	return NewProtocolHandlers(deps.Protocol)
}
