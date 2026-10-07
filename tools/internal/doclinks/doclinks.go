package doclinks

import (
	"flag"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

var (
	fenceRE       = regexp.MustCompile("^\\s{0,3}(`{3,}|~{3,})")
	referenceRE   = regexp.MustCompile(`^\s{0,3}\[[^]]+\]:\s*(?:<([^>]+)>|(\S+))`)
	htmlRE        = regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	anchorRE      = regexp.MustCompile(`(?i)<(?:a|span|h[1-6])\b[^>]*\b(?:id|name)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	headingRE     = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$`)
	setextRE      = regexp.MustCompile(`^\s{0,3}(?:=+|-+)\s*$`)
	schemeRE      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	headingLinkRE = regexp.MustCompile(`!?\[([^]]*)\]\([^)]*\)`)
	tagRE         = regexp.MustCompile(`<[^>]+>`)
)

type Reference struct {
	Source string
	Line   int
	Target string
}

func read(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("%s is not UTF-8", path)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), nil
}

func resolve(path string) string {
	path, _ = filepath.Abs(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	// Resolve existing parents even when the final target is missing.
	parent := filepath.Dir(path)
	if parent != path {
		return filepath.Join(resolve(parent), filepath.Base(path))
	}
	return path
}

func tracked(root string) ([]string, error) {
	cmd := exec.Command("git", "-c", "core.quotepath=false", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.md")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(string(b), "\x00") {
		if p == "" {
			continue
		}
		p = resolve(filepath.Join(root, p))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func removeCode(line string) string {
	for cursor := 0; cursor < len(line); {
		i := strings.IndexByte(line[cursor:], '`')
		if i < 0 {
			break
		}
		i += cursor
		end := i
		for end < len(line) && line[end] == '`' {
			end++
		}
		close := strings.Index(line[end:], line[i:end])
		if close < 0 {
			cursor = end
			continue
		}
		line = line[:i] + line[end+close+end-i:]
		cursor = i
	}
	return line
}

func inlineTargets(line string) []string {
	var targets []string
	for cursor := 0; cursor < len(line); {
		marker := strings.Index(line[cursor:], "](")
		if marker < 0 {
			break
		}
		marker += cursor
		pos := marker + 2
		for pos < len(line) {
			r, n := utf8.DecodeRuneInString(line[pos:])
			if !unicode.IsSpace(r) {
				break
			}
			pos += n
		}
		if pos < len(line) && line[pos] == '<' {
			if end := strings.IndexByte(line[pos+1:], '>'); end >= 0 {
				targets = append(targets, line[pos+1:pos+1+end])
				cursor = pos + end + 2
				continue
			}
		}
		start, depth, escaped := pos, 0, false
		for pos < len(line) {
			r, n := utf8.DecodeRuneInString(line[pos:])
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '(' {
				depth++
			} else if r == ')' {
				if depth == 0 {
					break
				}
				depth--
			} else if unicode.IsSpace(r) && depth == 0 {
				break
			}
			pos += n
		}
		targets = append(targets, line[start:pos])
		cursor = pos + 1
	}
	return targets
}

func links(path string) ([]Reference, error) {
	source, err := read(path)
	if err != nil {
		return nil, err
	}
	var result []Reference
	fence := ""
	for i, raw := range strings.Split(source, "\n") {
		if m := fenceRE.FindStringSubmatch(raw); m != nil {
			if fence == "" {
				fence = m[1]
			} else if m[1][0] == fence[0] && len(m[1]) >= len(fence) {
				fence = ""
			}
			continue
		}
		if fence != "" || strings.HasPrefix(raw, "    ") || strings.HasPrefix(raw, "\t") {
			continue
		}
		line := removeCode(raw)
		add := func(target string) { result = append(result, Reference{path, i + 1, target}) }
		if m := referenceRE.FindStringSubmatch(line); m != nil {
			add(m[1] + m[2])
		}
		for _, target := range inlineTargets(line) {
			add(target)
		}
		for _, m := range htmlRE.FindAllStringSubmatch(line, -1) {
			add(m[1] + m[2])
		}
	}
	return result, nil
}

func normalize(value string) string {
	value = html.UnescapeString(value)
	value = headingLinkRE.ReplaceAllString(value, "$1")
	value = tagRE.ReplaceAllString(value, "")
	value = strings.NewReplacer("`", "", "*", "", "~", "").Replace(value)
	var out []rune
	pending := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsSpace(r) {
			pending = len(out) > 0
			continue
		}
		if unicode.IsPunct(r) && r != '-' && r != '_' {
			continue
		}
		if pending && out[len(out)-1] != '-' {
			out = append(out, '-')
		}
		out = append(out, r)
		pending = false
	}
	return strings.Trim(string(out), "-")
}

func anchors(path string) (map[string]bool, error) {
	source, err := read(path)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	counts := map[string]int{}
	previous := ""
	for _, line := range strings.Split(source, "\n") {
		for _, m := range anchorRE.FindAllStringSubmatch(line, -1) {
			result[m[1]+m[2]] = true
		}
		heading := ""
		if m := headingRE.FindStringSubmatch(line); m != nil {
			heading = m[1]
		} else if previous != "" && setextRE.MatchString(line) {
			heading = strings.TrimSpace(previous)
		}
		if base := normalize(heading); base != "" {
			key := base
			if counts[base] > 0 {
				key = fmt.Sprintf("%s-%d", base, counts[base])
			}
			result[key] = true
			counts[base]++
		}
		previous = line
		if strings.TrimSpace(line) == "" {
			previous = ""
		}
	}
	return result, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func exactCase(root, path string) bool {
	if !within(root, path) {
		return false
	}
	rel, _ := filepath.Rel(root, path)
	if rel == "." {
		return true
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return false
		}
		found := false
		for _, e := range entries {
			if e.Name() == part {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		root = filepath.Join(root, part)
	}
	return true
}

func validate(ref Reference, root string, cache map[string]map[string]bool) error {
	target := strings.TrimSpace(ref.Target)
	if schemeRE.MatchString(target) || strings.HasPrefix(target, "//") {
		for _, r := range target {
			if unicode.IsSpace(r) || r < 32 {
				return fmt.Errorf("external URI contains whitespace or control characters")
			}
		}
		u, err := url.Parse(target)
		if err != nil {
			return fmt.Errorf("invalid URI: %w", err)
		}
		if (u.Scheme == "http" || u.Scheme == "https" || strings.HasPrefix(target, "//")) && u.Hostname() == "" {
			return fmt.Errorf("HTTP(S) URI has no host")
		}
		if u.Scheme == "mailto" && u.Opaque == "" && u.Path == "" {
			return fmt.Errorf("mailto URI has no recipient")
		}
		if u.Host == "" && u.Path == "" && u.Opaque == "" {
			return fmt.Errorf("URI has no scheme-specific target")
		}
		return nil
	}
	raw, frag, _ := strings.Cut(strings.NewReplacer(`\(`, "(", `\)`, ")").Replace(target), "#")
	raw, _, _ = strings.Cut(raw, "?")
	decode := func(s string) string {
		v, err := url.PathUnescape(s)
		if err != nil {
			return s
		}
		return v
	}
	raw, frag = decode(raw), decode(frag)
	if strings.ContainsRune(raw, 0) {
		return fmt.Errorf("local path contains a NUL byte")
	}
	path := ref.Source
	if strings.HasPrefix(raw, "/") {
		path = filepath.Join(root, strings.TrimLeft(raw, "/"))
	} else if raw != "" {
		path = filepath.Join(filepath.Dir(ref.Source), raw)
	}
	// Resolving a path on Windows also corrects its casing, so the casing is checked on the path as written.
	written := filepath.Clean(path)
	path = resolve(written)
	if !within(root, path) {
		return fmt.Errorf("local path escapes repository: %s", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing local target: %s", target)
	}
	if !exactCase(root, written) {
		return fmt.Errorf("local target has incorrect path casing: %s", target)
	}
	if frag == "" {
		return nil
	}
	if info.IsDir() {
		path = filepath.Join(path, "README.md")
	}
	if !strings.EqualFold(filepath.Ext(path), ".md") {
		return nil
	}
	a, ok := cache[path]
	if !ok {
		a, err = anchors(path)
		if err != nil {
			return err
		}
		cache[path] = a
	}
	if !a[frag] {
		rel, _ := filepath.Rel(root, path)
		return fmt.Errorf("missing anchor #%s in %s", frag, filepath.ToSlash(rel))
	}
	return nil
}

func Check(root string, files []string) ([]string, error) {
	var failures []string
	cache := map[string]map[string]bool{}
	for _, path := range files {
		refs, err := links(path)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if err := validate(ref, root, cache); err != nil {
				rel, _ := filepath.Rel(root, path)
				failures = append(failures, fmt.Sprintf("%s:%d: %v", filepath.ToSlash(rel), ref.Line, err))
			}
		}
	}
	return failures, nil
}

func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("check-doc-links", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cli.Usage(fs, "check-doc-links [--root ROOT] [files ...]", "Check tracked Markdown links without making network requests.")
	root := fs.String("root", repo.Root(), "repository root")
	if err := cli.Parse(fs, args, 0, -1); err != nil {
		return cli.ErrorTo(stderr, err)
	}
	*root = resolve(*root)
	var files []string
	var err error
	for _, p := range fs.Args() {
		if !filepath.IsAbs(p) {
			p = filepath.Join(*root, p)
		}
		files = append(files, resolve(p))
	}
	if len(files) == 0 {
		files, err = tracked(*root)
	}
	var failures []string
	if err == nil {
		failures, err = Check(*root, files)
	}
	if err != nil {
		fmt.Fprintf(stderr, "documentation link check failed: %v\n", err)
		return 1
	}
	if len(failures) > 0 {
		fmt.Fprintln(stderr, "documentation link check failed:")
		for _, e := range failures {
			fmt.Fprintln(stderr, "- "+e)
		}
		return 1
	}
	fmt.Fprintf(out, "documentation link check passed (%d Markdown files)\n", len(files))
	return 0
}
