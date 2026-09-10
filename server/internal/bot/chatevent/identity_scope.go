package chatevent

import "encoding/json"

// IdentityScope names a protocol's identity namespace. Only OneBot QQ IDs may
// be matched globally; openids require an adapter and receiving bot identity.
type IdentityScope struct {
	Kind           string `json:"kind"`
	SourceProtocol string `json:"source_protocol"`
	SourceAdapter  string `json:"source_adapter"`
	BotID          string `json:"bot_id"`
}

func (s IdentityScope) Valid() bool {
	if s.SourceProtocol != "onebot11" && s.SourceProtocol != "qqofficial" {
		return false
	}
	if s.SourceAdapter == "" || s.BotID == "" {
		return s.Kind == "global" && s.SourceProtocol == "onebot11" && s.SourceAdapter == "" && s.BotID == ""
	}
	return s.Kind == "instance"
}

// Key encodes all identity components without delimiter collisions.
func (s IdentityScope) Key(kind, id string) string {
	encoded, _ := json.Marshal([5]string{s.SourceProtocol, s.SourceAdapter, s.BotID, kind, id})
	return string(encoded)
}

func (e NormalizedEvent) IdentityScope() IdentityScope {
	return IdentityScope{Kind: "instance", SourceProtocol: e.SourceProtocol, SourceAdapter: e.SourceAdapter, BotID: e.BotID}
}
