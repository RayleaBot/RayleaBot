package config

import "strings"

func normalizeOneBotSection(document map[string]any) {
	onebot := section(document, "onebot")
	if onebot == nil {
		return
	}

	normalizeOneBotTransport(onebot, "reverse_ws")
	normalizeOneBotTransport(onebot, "forward_ws")
	normalizeOneBotTransport(onebot, "http_api")
	normalizeOneBotTransport(onebot, "webhook")
}

func normalizeOneBotTransport(onebot map[string]any, key string) {
	transport := transportSection(onebot, key)
	if transport == nil {
		transport = map[string]any{
			"enabled": false,
			"url":     "",
		}
		onebot[key] = transport
	}

	urlValue := strings.TrimSpace(stringValue(transport["url"]))
	transport["url"] = urlValue
	if _, ok := transport["enabled"].(bool); !ok {
		transport["enabled"] = false
	}
	transport["access_token"] = strings.TrimSpace(stringValue(transport["access_token"]))
}

func oneBotTransportDocument(enabled bool, urlValue string, accessToken string) map[string]any {
	return map[string]any{
		"enabled":      enabled,
		"url":          urlValue,
		"access_token": accessToken,
	}
}

func oneBotTransportConfigDocument(transport OneBotTransportConfig) map[string]any {
	return oneBotTransportDocument(transport.Enabled, transport.URL, transport.AccessToken)
}
