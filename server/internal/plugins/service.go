package plugins

// Service is the immutable declaration of methods exported by one plugin.
type Service struct {
	Name    string   `json:"name"`
	Version int      `json:"version"`
	Methods []string `json:"methods"`
}

type ServiceCall struct {
	TargetPluginID string
	Service        string
	ServiceVersion int
	Method         string
	Params         map[string]any
}

func CloneServices(services []Service) []Service {
	if services == nil {
		return nil
	}
	out := make([]Service, len(services))
	for i, service := range services {
		out[i] = service
		out[i].Methods = append([]string(nil), service.Methods...)
	}
	return out
}
