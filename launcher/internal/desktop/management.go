package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ManagementClient struct {
	client          *http.Client
	getControlToken func() string
}

func NewManagementClient(getControlToken func() string) *ManagementClient {
	return &ManagementClient{
		client:          &http.Client{Timeout: 5 * time.Second},
		getControlToken: getControlToken,
	}
}

func ResolveServerEndpoint(configPath string) (ServerEndpoint, string) {
	host := "127.0.0.1"
	port := 8080
	payload, err := os.ReadFile(configPath)
	warning := ""
	if err != nil {
		warning = "无法读取服务监听配置，已回退到 127.0.0.1:8080。"
	} else {
		var config struct {
			Server struct {
				Host string `yaml:"host"`
				Port any    `yaml:"port"`
			} `yaml:"server"`
		}
		if yaml.Unmarshal(payload, &config) == nil {
			if candidate := normalizeClientHost(config.Server.Host); candidate != "" {
				host = candidate
			}
			switch value := config.Server.Port.(type) {
			case int:
				if value > 0 && value <= 65535 {
					port = value
				}
			case string:
				if parsed, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil && parsed > 0 && parsed <= 65535 {
					port = parsed
				}
			}
		} else {
			warning = "服务监听配置格式无效，已回退到 127.0.0.1:8080。"
		}
	}
	return ServerEndpoint{Host: host, Port: port, BaseURL: (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: "/"}).String()}, warning
}

func normalizeClientHost(host string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || host == "0.0.0.0" || host == "::" || host == "*" {
		return "127.0.0.1"
	}
	return host
}

func (m *ManagementClient) IsHealthy(ctx context.Context, endpoint ServerEndpoint) bool {
	response, err := m.request(ctx, http.MethodGet, endpoint, "healthz", false)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func (m *ManagementClient) GetReadiness(ctx context.Context, endpoint ServerEndpoint) (JSONObject, error) {
	response, err := m.request(ctx, http.MethodGet, endpoint, "readyz", false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusServiceUnavailable {
		return nil, responseError(response)
	}
	return decodeObject(response.Body)
}

func (m *ManagementClient) GetLauncherStatus(ctx context.Context, endpoint ServerEndpoint) (JSONObject, error) {
	response, err := m.request(ctx, http.MethodGet, endpoint, "api/launcher/status", true)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}
	return decodeObject(response.Body)
}

func (m *ManagementClient) Shutdown(ctx context.Context, endpoint ServerEndpoint) error {
	response, err := m.request(ctx, http.MethodPost, endpoint, "api/launcher/shutdown", true)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseError(response)
	}
	return nil
}

func (m *ManagementClient) request(ctx context.Context, method string, endpoint ServerEndpoint, path string, controlled bool) (*http.Response, error) {
	base, err := url.Parse(endpoint.BaseURL)
	if err != nil {
		return nil, err
	}
	target, err := base.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return nil, err
	}
	if controlled {
		if token := strings.TrimSpace(m.getControlToken()); token != "" {
			request.Header.Set("X-Raylea-Launcher-Control", token)
		}
	}
	return m.client.Do(request)
}

func decodeObject(reader io.Reader) (JSONObject, error) {
	var payload JSONObject
	if err := json.NewDecoder(io.LimitReader(reader, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func responseError(response *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &envelope) == nil && strings.TrimSpace(envelope.Error.Message) != "" {
		return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
	}
	detail := strings.TrimSpace(string(payload))
	if detail == "" {
		detail = response.Status
	}
	return fmt.Errorf("管理接口返回 %s: %s", response.Status, detail)
}

func objectStatus(payload JSONObject) string {
	status, _ := payload["status"].(string)
	return strings.TrimSpace(status)
}
