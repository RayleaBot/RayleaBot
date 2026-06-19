package subscriptions

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const HubPluginID = "raylea.subscription-hub"

type Subject struct {
	UID       string
	Name      string
	AvatarURL string
	RoomID    string
	Services  map[string]bool
}

type Provider interface {
	LoadSubjects(context.Context) (map[string]Subject, error)
}

type PluginConfigReader interface {
	ReadAll(context.Context, string) (map[string]any, error)
}

type PluginConfigProvider struct {
	config PluginConfigReader
}

func NewPluginConfigProvider(config PluginConfigReader) *PluginConfigProvider {
	return &PluginConfigProvider{config: config}
}

func (p *PluginConfigProvider) LoadSubjects(ctx context.Context) (map[string]Subject, error) {
	if p == nil || p.config == nil {
		return nil, fmt.Errorf("subscription hub settings reader is required")
	}
	values, err := p.config.ReadAll(ctx, HubPluginID)
	if err != nil {
		return nil, fmt.Errorf("read subscription hub settings: %w", err)
	}
	return SubjectsFromSettings(values), nil
}

func SubjectsFromSettings(values map[string]any) map[string]Subject {
	raw := values["subscriptions"]
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]map[string]any); ok {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		}
	}

	subjects := make(map[string]Subject)
	for _, item := range items {
		subscription, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if !boolValue(subscription["enabled"], true) {
			continue
		}
		if strings.TrimSpace(stringValue(subscription["platform"])) != "bilibili" {
			continue
		}
		uid := onlyDigits(stringValue(subscription["uid"]))
		if uid == "" {
			continue
		}
		subject := subjects[uid]
		subject.UID = uid
		if subject.Name == "" {
			subject.Name = strings.TrimSpace(stringValue(subscription["name"]))
		}
		if subject.AvatarURL == "" {
			subject.AvatarURL = strings.TrimSpace(stringValue(subscription["avatar_url"]))
		}
		if subject.Services == nil {
			subject.Services = make(map[string]bool)
		}
		for _, service := range stringList(subscription["services"]) {
			if service == "all" {
				addAllServices(subject.Services)
				continue
			}
			subject.Services[service] = true
		}
		if len(subject.Services) == 0 {
			addAllServices(subject.Services)
		}
		subjects[uid] = subject
	}
	return subjects
}

func addAllServices(services map[string]bool) {
	services["live"] = true
	services["video"] = true
	services["image_text"] = true
	services["article"] = true
	services["repost"] = true
}

func boolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		if typed == "" {
			return fallback
		}
		return typed == "true" || typed == "1"
	default:
		return fallback
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func stringList(value any) []string {
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, strings.TrimSpace(item))
		}
		return result
	default:
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(stringValue(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func onlyDigits(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}
