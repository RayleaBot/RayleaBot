package config

import "strings"

func normalizeOneBotSection(document map[string]any) {
	onebot := section(document, "onebot")
	if onebot == nil {
		return
	}

	normalizeOneBotTransport(onebot, "reverse_ws", true)
	normalizeOneBotTransport(onebot, "forward_ws", true)
	normalizeOneBotTransport(onebot, "http_api", false)
	normalizeOneBotTransport(onebot, "webhook", true)
}

func normalizeOneBotTransport(onebot map[string]any, key string, allowQueryCompat bool) {
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
	if allowQueryCompat {
		if _, ok := transport["access_token_query_compat"].(bool); !ok {
			transport["access_token_query_compat"] = false
		}
	} else {
		delete(transport, "access_token_query_compat")
	}
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

func oneBotTransportCompatDocument(transport OneBotTransportConfig) map[string]any {
	document := oneBotTransportDocument(transport.Enabled, transport.URL, transport.AccessToken)
	document["access_token_query_compat"] = transport.AccessTokenQueryCompat
	return document
}
