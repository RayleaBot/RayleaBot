package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"unicode"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

//go:embed data/characters.json
var characterData []byte

type character struct {
	Name    string   `json:"name"`
	Slug    string   `json:"slug"`
	Aliases []string `json:"aliases"`
}

type characterCatalog struct {
	Characters []character `json:"characters"`
}

type searchResponse struct {
	Data struct {
		Posts []struct {
			Post struct {
				PostID  string   `json:"post_id"`
				Subject string   `json:"subject"`
				Images  []string `json:"images"`
			} `json:"post"`
			User struct {
				Nickname string `json:"nickname"`
			} `json:"user"`
		} `json:"posts"`
	} `json:"data"`
}

var catalog = mustCharacterCatalog()

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{
		PluginID: "raylea.game-guide",
		Subscriptions: []string{
			"message.group", "message.private", "plugin.started", "bot.identity.changed",
		},
		MaxConcurrentHandlers: 3,
	}, rayleabot.HandlerFunc(handleEvent))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handleEvent(ctx context.Context, event *rayleabot.EventContext) error {
	command := strings.TrimSpace(event.Event.Command())
	if command == "" {
		return event.Result(map[string]any{"handled": false})
	}
	if command == "角色列表" {
		return sendCharacterList(ctx, event)
	}
	query := ""
	if command == "游戏攻略" {
		query = strings.TrimSpace(strings.Join(event.Event.Args(), " "))
	} else if strings.HasSuffix(command, "攻略") {
		query = strings.TrimSpace(strings.TrimSuffix(command, "攻略"))
	}
	if query == "" {
		return event.Result(map[string]any{"handled": false})
	}
	matched, ok := findCharacter(query)
	if !ok {
		return event.SendText("暂未适配角色“" + query + "”。发送“角色列表”查看已适配角色。")
	}
	return sendGuide(ctx, event, matched)
}

func sendCharacterList(ctx context.Context, event *rayleabot.EventContext) error {
	names := make([]string, 0, len(catalog.Characters))
	for _, item := range catalog.Characters {
		names = append(names, item.Name)
	}
	sort.Strings(names)
	fallback := "已适配角色（共 " + fmt.Sprint(len(names)) + " 位）：\n" + strings.Join(names, "、")
	result, err := event.Actions().RenderImage(ctx, rayleabot.RenderImageRequest{
		Template: "character-list", Data: map[string]any{"title": "已适配角色", "characters": names}, Output: "png", FallbackText: fallback,
	})
	if err == nil {
		if imagePath, _ := result["image_path"].(string); imagePath != "" {
			return event.Send(event.Event.Target.Type, event.Event.Target.ID, rayleabot.Image(imagePath))
		}
	}
	return event.SendText(fallback)
}

func sendGuide(ctx context.Context, event *rayleabot.EventContext, item character) error {
	endpoint := "https://bbs-api.miyoushe.com/post/wapi/searchPosts?gids=6&page_size=10&keyword=" + url.QueryEscape(item.Name+"攻略")
	result, err := event.Actions().HTTPRequest(ctx, rayleabot.HTTPRequest{
		Method: "GET", URL: endpoint, TimeoutSeconds: 20,
		Headers: map[string]string{"Accept": "application/json", "Referer": "https://www.miyoushe.com/sr/"},
	})
	if err != nil {
		return event.SendText(guideFallback(item))
	}
	body, _ := result["body_text"].(string)
	var response searchResponse
	if json.Unmarshal([]byte(body), &response) != nil {
		return event.SendText(guideFallback(item))
	}
	nodes := buildGuideNodes(item, response)
	if len(nodes) == 0 {
		return event.SendText(guideFallback(item))
	}
	request := map[string]any{
		"target_type": event.Event.Target.Type,
		"target_id":   event.Event.Target.ID,
		"messages":    nodes,
	}
	var output map[string]any
	if err := event.Actions().OneBot(ctx, rayleabot.ActionMessageForwardSend, request, &output); err != nil {
		return event.SendText(guideFallback(item))
	}
	return event.Result(map[string]any{"character": item.Name, "sources": len(nodes)})
}

func buildGuideNodes(item character, response searchResponse) []map[string]any {
	nodes := make([]map[string]any, 0, 12)
	for _, source := range response.Data.Posts {
		if len(source.Post.Images) == 0 {
			continue
		}
		content := []map[string]any{{
			"type": "text", "data": map[string]any{"text": source.Post.Subject + "\n作者：" + source.User.Nickname},
		}}
		for _, image := range source.Post.Images {
			if parsed, err := url.Parse(image); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") {
				content = append(content, map[string]any{"type": "image", "data": map[string]any{"file": image}})
			}
			if len(content) >= 6 {
				break
			}
		}
		if len(content) == 1 {
			continue
		}
		nodes = append(nodes, map[string]any{
			"type": "node", "data": map[string]any{"name": item.Name + "攻略", "uin": "10000", "content": content},
		})
		if len(nodes) >= 10 {
			break
		}
	}
	return nodes
}

func guideFallback(item character) string {
	return item.Name + "攻略暂时无法获取，请稍后重试。\n搜索入口：https://www.miyoushe.com/sr/search?keyword=" + url.QueryEscape(item.Name+"攻略")
}

func findCharacter(query string) (character, bool) {
	key := normalize(query)
	for _, item := range catalog.Characters {
		if normalize(item.Name) == key || normalize(item.Slug) == key {
			return item, true
		}
		for _, alias := range item.Aliases {
			if normalize(alias) == key {
				return item, true
			}
		}
	}
	return character{}, false
}

func normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '·' || r == '•' {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func mustCharacterCatalog() characterCatalog {
	var value characterCatalog
	if err := json.Unmarshal(characterData, &value); err != nil || len(value.Characters) == 0 {
		panic("invalid embedded character catalog")
	}
	return value
}
