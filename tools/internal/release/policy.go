package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
)

func ValidateTag(root, tag string) (string, error) {
	pattern, e := contractdata.Value(root, "release-manifest.schema.json", "/$defs/semver/pattern")
	if e != nil {
		return "", e
	}
	re, e := regexp.Compile("^(?:" + ordered.Str(pattern) + ")$")
	if e != nil {
		return "", e
	}
	if !strings.HasPrefix(tag, "v") || !re.MatchString(tag[1:]) {
		return "", errors.New("release tag must be v followed by a version accepted by the release contract")
	}
	return tag[1:], nil
}
func Channel(root, version, requested string) (string, error) {
	if _, e := ValidateTag(root, "v"+version); e != nil {
		return "", e
	}
	channel := "stable"
	if strings.Contains(strings.SplitN(version, "+", 2)[0], "-") {
		channel = "beta"
	}
	if requested != "" && requested != channel {
		return "", fmt.Errorf("version %s requires channel=%s, not %s", version, channel, requested)
	}
	return channel, nil
}

type Publication struct {
	Value      string `json:"value"`
	Channel    string `json:"channel"`
	Prerelease string `json:"prerelease"`
	MakeLatest string `json:"make_latest"`
}

func PublicationSettings(root, tag string) (Publication, error) {
	version, e := ValidateTag(root, tag)
	if e != nil {
		return Publication{}, e
	}
	channel, e := Channel(root, version, "")
	if e != nil {
		return Publication{}, e
	}
	settings := Publication{version, channel, "false", "legacy"}
	if channel == "beta" {
		settings.Prerelease = "true"
		settings.MakeLatest = "false"
	}
	return settings, nil
}

var placeholder = regexp.MustCompile(`(?s)\{\{.*?\}\}`)
var comment = regexp.MustCompile(`(?s)<!--.*?(?:-->|$)`)
var heading = regexp.MustCompile(`^\s*(?:#{1,6}(?:\s.*)?|[-*_]{3,})\s*$`)

func ValidateNotes(root, tag, notesDir string) (string, error) {
	if _, e := ValidateTag(root, tag); e != nil {
		return "", e
	}
	p := filepath.Join(notesDir, tag+".md")
	b, e := os.ReadFile(p)
	if e != nil {
		return "", e
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("%s is not UTF-8", p)
	}
	body := strings.TrimPrefix(string(b), "\ufeff")
	if loc := placeholder.FindStringIndex(body); loc != nil {
		return "", fmt.Errorf("%s:%d: unresolved release notes placeholder", p, strings.Count(body[:loc[0]], "\n")+1)
	}
	visible := comment.ReplaceAllString(body, "")
	for _, line := range strings.Split(visible, "\n") {
		if strings.TrimSpace(line) != "" && !heading.MatchString(line) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s: release notes have no body content", p)
}
func WritePublication(path string, p Publication) error {
	f, e := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(f, "value=%s\nchannel=%s\nprerelease=%s\nmake_latest=%s\n", p.Value, p.Channel, p.Prerelease, p.MakeLatest)
	return errors.Join(e, f.Close())
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if !utf8.Valid(b) {
		return fmt.Errorf("%s is not UTF-8", path)
	}
	return json.Unmarshal(b, v)
}
func writeJSON(path string, v any) error {
	b, e := ordered.JSON(v, true)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, b, 0644)
}

// Single-line release documents retain spaces after separators, including when
// strings contain punctuation, quotes or escapes.
func singleLineJSON(value any) ([]byte, error) {
	raw, err := ordered.JSON(value, false)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	quoted, escaped := false, false
	for _, c := range bytes.TrimSuffix(raw, []byte("\n")) {
		out.WriteByte(c)
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
		} else if c == ',' || c == ':' {
			out.WriteByte(' ')
		}
	}
	return out.Bytes(), nil
}
