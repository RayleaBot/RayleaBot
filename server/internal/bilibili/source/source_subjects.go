package source

import (
	"context"
	"fmt"
	"strings"
)

func (s *Source) loadSubjects(ctx context.Context) (map[string]Subject, error) {
	values, err := s.pluginConfig.ReadAll(ctx, subscriptionHubPluginID)
	if err != nil {
		return nil, fmt.Errorf("read subscription hub settings: %w", err)
	}
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
				subject.Services["live"] = true
				subject.Services["video"] = true
				subject.Services["image_text"] = true
				subject.Services["article"] = true
				subject.Services["repost"] = true
				continue
			}
			subject.Services[service] = true
		}
		if len(subject.Services) == 0 {
			subject.Services["live"] = true
			subject.Services["video"] = true
			subject.Services["image_text"] = true
			subject.Services["article"] = true
			subject.Services["repost"] = true
		}
		subjects[uid] = subject
	}
	return subjects, nil
}
