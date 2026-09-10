package plugins

import (
	"errors"
	"regexp"
)

var ErrInvalidPluginID = errors.New("invalid plugin identifier")

// pluginIDPattern follows contracts/plugin-info.schema.json properties.id.
var pluginIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$`)

func ValidPluginID(id string) bool { return pluginIDPattern.MatchString(id) }
