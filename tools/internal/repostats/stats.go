package repostats

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed line.svg.tmpl
var lineTemplate string

//go:embed heatmap.svg.tmpl
var heatmapTemplate string
var palette = []string{"#161b22", "#0e4429", "#006d32", "#26a641", "#39d353"}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;").Replace(s)
}
func Fetch(client *http.Client, base, token string, since time.Time) ([]time.Time, error) {
	var commits []time.Time
	for page := 1; page <= 50; page++ {
		address := fmt.Sprintf("%s/commits?since=%s&per_page=100&page=%d", base, since.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05+00:00"), page)
		req, err := http.NewRequest("GET", address, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "rayleabot-repo-stats")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var data []struct {
			Commit struct {
				Committer struct{ Date string }
				Author    struct{ Date string }
			}
		}
		if response.StatusCode >= 400 {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			return nil, fmt.Errorf("GitHub API error: %s\n%s", response.Status, string(body))
		}
		err = json.NewDecoder(response.Body).Decode(&data)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			break
		}
		for _, c := range data {
			raw := c.Commit.Committer.Date
			if raw == "" {
				raw = c.Commit.Author.Date
			}
			if raw != "" {
				date, err := time.Parse(time.RFC3339Nano, raw)
				if err != nil {
					return nil, err
				}
				commits = append(commits, date)
			}
		}
		if !strings.Contains(response.Header.Get("Link"), `rel="next"`) {
			break
		}
	}
	return commits, nil
}
func Counts(commits []time.Time) (map[string]int, map[string]int) {
	monthly, daily := map[string]int{}, map[string]int{}
	for _, dt := range commits {
		monthly[dt.Format("2006-01")]++
		daily[dt.UTC().Format("2006-01-02")]++
	}
	return monthly, daily
}
func Line(repository string, today time.Time, monthly map[string]int) string {
	var months [12]string
	var values [12]int
	maxVal := 1
	cursor := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	for i := 11; i >= 0; i-- {
		months[i] = cursor.Format("2006-01")
		values[i] = monthly[months[i]]
		maxVal = max(maxVal, values[i])
		cursor = cursor.AddDate(0, -1, 0)
	}
	var xs, ys [12]float64
	for i, v := range values {
		xs[i] = 50 + float64(i)/11*740
		ys[i] = 180 - float64(v)/float64(maxVal)*140
	}
	path := fmt.Sprintf("M %.1f %.1f", xs[0], ys[0])
	for i := 1; i < 12; i++ {
		path += fmt.Sprintf(" L %.1f %.1f", xs[i], ys[i])
	}
	area := fmt.Sprintf("%s L %.1f 180.0 L %.1f 180.0 Z", path, xs[11], xs[0])
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(lineTemplate, "@@REPO@@", escape(repository)))
	for i := 0; i < 5; i++ {
		gy := 40 + float64(i)/4*140
		fmt.Fprintf(&b, "  <line class=\"grid\" x1=\"50\" y1=\"%.1f\" x2=\"790\" y2=\"%.1f\" stroke-dasharray=\"4 4\"/>\n", gy, gy)
	}
	for i, m := range months {
		fmt.Fprintf(&b, "  <text x=\"%.1f\" y=\"206\" text-anchor=\"middle\" class=\"axis-text\">%s</text>\n", xs[i], m[5:])
	}
	fmt.Fprintf(&b, "  <text x=\"40\" y=\"44\" text-anchor=\"end\" class=\"axis-text\">%d</text>\n", maxVal)
	fmt.Fprintf(&b, "  <path class=\"area\" d=\"%s\">\n    <animate attributeName=\"opacity\" from=\"0\" to=\"1\" dur=\"1s\" fill=\"freeze\"/>\n  </path>\n", area)
	length := 0.0
	for i := 1; i < 12; i++ {
		length += math.Sqrt(math.Pow(xs[i]-xs[i-1], 2) + math.Pow(ys[i]-ys[i-1], 2))
	}
	fmt.Fprintf(&b, "  <path class=\"line\" d=\"%s\" stroke-dasharray=\"%.1f\" stroke-dashoffset=\"%.1f\">\n", path, length, length)
	fmt.Fprintf(&b, "    <animate attributeName=\"stroke-dashoffset\" from=\"%.1f\" to=\"0\" dur=\"1.5s\" fill=\"freeze\" calcMode=\"spline\" keySplines=\"0.4 0 0.2 1\" keyTimes=\"0;1\"/>\n  </path>\n", length)
	for i := range months {
		fmt.Fprintf(&b, "  <circle class=\"point\" cx=\"%.1f\" cy=\"%.1f\" r=\"4\" opacity=\"0\">\n", xs[i], ys[i])
		fmt.Fprintf(&b, "    <animate attributeName=\"opacity\" from=\"0\" to=\"1\" begin=\"%.2fs\" dur=\"0.3s\" fill=\"freeze\"/>\n  </circle>\n", 0.8+float64(i)*0.08)
		fmt.Fprintf(&b, "  <title>%s: %d commits</title>\n", months[i], values[i])
	}
	b.WriteString("</svg>")
	return b.String()
}
func Heatmap(repository string, today time.Time, daily map[string]int) string {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	weekday := (int(today.Weekday()) + 6) % 7
	start := today.AddDate(0, 0, -weekday-52*7)
	count := weekday + 52*7 + 1
	weeks := (count + 6) / 7
	width := 60 + weeks*15
	maxCount := 0
	for i := 0; i < count; i++ {
		maxCount = max(maxCount, daily[start.AddDate(0, 0, i).Format("2006-01-02")])
	}
	var b strings.Builder
	b.WriteString(strings.NewReplacer("@@REPO@@", escape(repository), "@@WIDTH@@", fmt.Sprint(width)).Replace(heatmapTemplate))
	prevMonth := time.Month(0)
	for i := 0; i < count; i += 7 {
		d := start.AddDate(0, 0, i)
		if d.Month() != prevMonth {
			fmt.Fprintf(&b, "  <text x=\"%.1f\" y=\"42\" class=\"axis-text\">%s</text>\n", float64(40+i/7*15), d.Format("Jan"))
			prevMonth = d.Month()
		}
	}
	for i, wd := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		if i%2 == 0 {
			fmt.Fprintf(&b, "  <text x=\"32\" y=\"%d\" text-anchor=\"end\" class=\"axis-text\">%s</text>\n", 64+i*15, wd)
		}
	}
	for i := 0; i < count; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		value := daily[d]
		level := 0
		if value != 0 {
			if maxCount <= 1 {
				level = 4
			} else {
				level = min(4, max(1, int(float64(value)/float64(maxCount)*4)))
			}
		}
		fmt.Fprintf(&b, "  <rect class=\"cell\" x=\"%d\" y=\"%d\" fill=\"%s\">\n", 40+i/7*15, 52+i%7*15, palette[level])
		fmt.Fprintf(&b, "    <animate attributeName=\"opacity\" from=\"0\" to=\"1\" begin=\"%.3fs\" dur=\"0.4s\" fill=\"freeze\"/>\n", float64(i)*0.003)
		plural := "s"
		if value == 1 {
			plural = ""
		}
		fmt.Fprintf(&b, "    <title>%s: %d commit%s</title>\n  </rect>\n", d, value, plural)
	}
	lx := width - 20 - 90
	fmt.Fprintf(&b, "  <text x=\"%d\" y=\"181\" text-anchor=\"end\" class=\"axis-text\">Less</text>\n", lx-6)
	for i, color := range palette {
		fmt.Fprintf(&b, "  <rect class=\"cell\" x=\"%d\" y=\"171\" fill=\"%s\"/>\n", lx+i*16, color)
	}
	fmt.Fprintf(&b, "  <text x=\"%d\" y=\"181\" class=\"axis-text\">More</text>\n</svg>", lx+84)
	return b.String()
}
func Run(out, stderr io.Writer) int {
	env := func(k, d string) string {
		if v, ok := os.LookupEnv(k); ok {
			return v
		}
		return d
	}
	repository := env("REPO_OWNER", "RayleaBot") + "/" + env("REPO_NAME", "RayleaBot")
	now := time.Now()
	since := now.UTC().Add(-365 * 24 * time.Hour)
	fmt.Fprintf(out, "Fetching commits for %s since %s...\n", repository, since.Format("2006-01-02"))
	commits, err := Fetch(&http.Client{Timeout: 30 * time.Second}, "https://api.github.com/repos/"+repository, os.Getenv("GITHUB_TOKEN"), since)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(out, "Fetched %d commits\n", len(commits))
	monthly, daily := Counts(commits)
	if err := os.MkdirAll("dist", 0755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, output := range []struct{ name, svg string }{{"repo-activity-line.svg", Line(repository, now, monthly)}, {"repo-activity-heatmap.svg", Heatmap(repository, now, daily)}} {
		if err := os.WriteFile(filepath.Join("dist", output.name), []byte(output.svg), 0644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(out, "Wrote dist/"+output.name)
	}
	return 0
}
