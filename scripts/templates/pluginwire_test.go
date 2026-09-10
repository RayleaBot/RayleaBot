package pluginwire

import (
 "encoding/json"
 "errors"
 "os"
 "strings"
 "testing"
)

func TestContractFrames(t *testing.T) {
 data, err := os.ReadFile("testdata/frames.generated.json"); if err != nil { t.Fatal(err) }
 var cases []struct { Name string; Valid bool; Frames []json.RawMessage }
 if err := json.Unmarshal(data, &cases); err != nil { t.Fatal(err) }
 for _, c := range cases { t.Run(c.Name, func(t *testing.T) {
  rejected := false
  for _, frame := range c.Frames { if err := Validate(frame, 0); err != nil { rejected = true; if c.Valid { t.Fatalf("valid frame rejected: %v", err) } } }
  if !c.Valid && !rejected { t.Fatal("all invalid fixture frames accepted") }
 }) }
}

func TestWireBoundaries(t *testing.T) {
 cases := []struct { name, frame string; valid bool }{
  {"empty arrays", `{"type":"init","request_id":"i","protocol_version":"3","plugin_id":"p","timezone":"UTC","config":{},"effective_permissions":[],"super_admins":[],"command_prefixes":["/"],"concurrency":1,"bots":[]}`, true},
  {"missing result status", `{"type":"result","request_id":"r","data":{}}`, false},
  {"zero timestamp", `{"type":"event","request_id":"e","event":{"event_id":"e","source_protocol":"system","source_adapter":"scheduler","event_type":"scheduler.trigger","timestamp":0}}`, true},
  {"decimal timestamp", `{"type":"event","request_id":"e","event":{"event_id":"e","source_protocol":"system","source_adapter":"scheduler","event_type":"scheduler.trigger","timestamp":0.5}}`, false},
  {"explicit null value", `{"type":"action","request_id":"a","action":"storage.kv","data":{"operation":"set","key":"k","value":null}}`, true},
  {"absent value", `{"type":"action","request_id":"a","action":"storage.kv","data":{"operation":"set","key":"k"}}`, false},
  {"exclusive HTTP body", `{"type":"action","request_id":"a","action":"http.request","data":{"method":"POST","url":"https://example.com","body_text":"x","body_base64":"eA=="}}`, false},
  {"nested unknown", `{"type":"action","request_id":"a","action":"http.request","data":{"method":"GET","url":"https://example.com","extra":true}}`, false},
  {"unknown envelope", `{"type":"ping","request_id":"p","extra":false}`, false},
  {"double JSON", `{"type":"ping","request_id":"p"}{}`, false},
 }
 for _, c := range cases { t.Run(c.name, func(t *testing.T) { if err := Validate([]byte(c.frame), 0); (err == nil) != c.valid { t.Fatalf("valid=%v: %v", c.valid, err) } }) }
}

func TestWirePresenceRoundTrip(t *testing.T) {
 var kv ProtocolActionStorageKVFrame
 if err := json.Unmarshal([]byte(`{"operation":"set","key":"k","value":null}`), &kv); err != nil { t.Fatal(err) }
 if string(kv.Value) != "null" { t.Fatalf("null value lost: %s", kv.Value) }
 data, err := json.Marshal(kv); if err != nil { t.Fatal(err) }
 if !strings.Contains(string(data), `"value":null`) { t.Fatalf("null omitted: %s", data) }
 zero := int64(0)
 data, err = json.Marshal(ProtocolWebhookFrame{Route:"hook", ClientTimestamp:&zero}); if err != nil { t.Fatal(err) }
 if !strings.Contains(string(data), `"client_timestamp":0`) || !strings.Contains(string(data), `"received_at":0`) { t.Fatalf("zero omitted: %s", data) }
 disabled := false
 data, err = json.Marshal(ProtocolActionGovernanceWhitelistWriteFrame{Enabled:&disabled}); if err != nil { t.Fatal(err) }
 if !strings.Contains(string(data), `"enabled":false`) { t.Fatalf("false omitted: %s", data) }
 frame := Frame{Type:"init", RequestID:"i", ProtocolVersion:ProtocolVersion, PluginID:"p", Timezone:"UTC", Config:map[string]any{}, EffectivePermissions:[]string{}, SuperAdmins:[]string{}, CommandPrefixes:[]string{"/"}, Concurrency:1, Bots:&[]BotIdentity{}}
 data, err = json.Marshal(frame); if err != nil { t.Fatal(err) }
 if err := Validate(data, 0); err != nil { t.Fatalf("empty required arrays/maps lost: %s: %v", data, err) }
}

func TestFrameByteLimit(t *testing.T) {
 data := []byte(`{"type":"error","request_id":"r","code":"plugin.internal_error","message":"中文"}`)
 if err := Validate(append(append([]byte{}, data...), '\n'), len(data)); err != nil { t.Fatal(err) }
 if err := Validate(data, len(data)-1); !errors.Is(err, ErrFrameTooLarge) { t.Fatalf("byte limit not enforced: %v", err) }
 if err := Validate([]byte(`{"type":"ping","request_id":"p","password":"fixture-sensitive"}`), 0); err == nil || strings.Contains(err.Error(), "fixture-sensitive") { t.Fatalf("validation leaked input: %v", err) }
}
