package release

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"github.com/RayleaBot/RayleaBot/tools/internal/processoutput"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func compareNumber(a, b string) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// CompareVersions compares validated semver values and ignores build metadata.
func CompareVersions(a, b string) int {
	a = strings.SplitN(a, "+", 2)[0]
	b = strings.SplitN(b, "+", 2)[0]
	ac, ap, _ := strings.Cut(a, "-")
	bc, bp, _ := strings.Cut(b, "-")
	aa, ba := strings.Split(ac, "."), strings.Split(bc, ".")
	for i := 0; i < 3; i++ {
		if c := compareNumber(aa[i], ba[i]); c != 0 {
			return c
		}
	}
	if ap == "" && bp != "" {
		return 1
	}
	if bp == "" && ap != "" {
		return -1
	}
	aa, ba = strings.Split(ap, "."), strings.Split(bp, ".")
	for i := 0; i < min(len(aa), len(ba)); i++ {
		var c int
		an, bn := numeric(aa[i]), numeric(ba[i])
		switch {
		case an && bn:
			c = compareNumber(aa[i], ba[i])
		case an:
			c = -1
		case bn:
			c = 1
		default:
			c = strings.Compare(aa[i], ba[i])
		}
		if c != 0 {
			return c
		}
	}
	if len(aa) < len(ba) {
		return -1
	}
	if len(aa) > len(ba) {
		return 1
	}
	return 0
}
func RewriteManifest(root string, manifest *ordered.Object, base string) (*ordered.Object, error) {
	b, e := ordered.JSON(manifest, false)
	if e != nil {
		return nil, e
	}
	value, e := ordered.Parse(b)
	if e != nil {
		return nil, e
	}
	result := ordered.Obj(value)
	version := ordered.Str(result.Get("version"))
	for _, value := range ordered.List(result.Get("artifacts")) {
		a := ordered.Obj(value)
		a.Set("download_url", strings.TrimRight(base, "/")+"/v"+version+"/"+ordered.Str(a.Get("file_name")))
	}
	if e = contractdata.Validate(root, "release-manifest.schema.json", result); e != nil {
		return nil, e
	}
	return result, nil
}
func ChannelDocuments(root string, manifests []*ordered.Object) (*ordered.Object, error) {
	orderedManifests := slices.Clone(manifests)
	for _, m := range orderedManifests {
		if _, e := Channel(root, ordered.Str(m.Get("version")), ""); e != nil {
			return nil, e
		}
	}
	slices.SortStableFunc(orderedManifests, func(a, b *ordered.Object) int {
		return -CompareVersions(ordered.Str(a.Get("version")), ordered.Str(b.Get("version")))
	})
	orderedManifests = orderedManifests[:min(30, len(orderedManifests))]
	if len(orderedManifests) == 0 {
		return nil, errors.New("no compatible published release to mirror")
	}
	releases := []any{}
	for _, m := range orderedManifests {
		releases = append(releases, &ordered.Object{{Name: "version", Value: m.Get("version")}, {Name: "channel", Value: m.Get("channel")}, {Name: "release_notes_ref", Value: m.Get("release_notes_ref")}})
	}
	docs := &ordered.Object{{Name: "releases.json", Value: releases}, {Name: "beta.json", Value: orderedManifests[0]}}
	for _, m := range orderedManifests {
		if m.Get("channel") == "stable" {
			docs.Set("stable.json", m)
			break
		}
	}
	return docs, nil
}

type MirrorOptions struct{ Repository, BaseURL, Bucket, Endpoint string }
type MirrorRunner func(...string) (string, error)

func mirrorCommand(args ...string) (string, error) {
	r, e := processoutput.Run(args, "")
	if e != nil {
		return "", e
	}
	if r.Code != 0 {
		return "", fmt.Errorf("%s failed with exit code %d: %s", args[0], r.Code, strings.TrimSpace(r.Stderr))
	}
	return r.Stdout, nil
}
func SyncMirror(root string, o MirrorOptions, run MirrorRunner) (int, error) {
	base, e := url.Parse(o.BaseURL)
	if e != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return 0, errors.New("mirror base URL must use HTTPS without credentials, query or fragment")
	}
	endpoint, e := url.Parse(o.Endpoint)
	if e != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" {
		return 0, errors.New("object storage endpoint must use HTTPS")
	}
	raw, e := run("gh", "api", "repos/"+o.Repository+"/releases?per_page=100")
	if e != nil {
		return 0, e
	}
	var releases []struct {
		Tag    string `json:"tag_name"`
		Draft  bool   `json:"draft"`
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if e = json.Unmarshal([]byte(raw), &releases); e != nil {
		return 0, e
	}
	candidates := []string{}
	for _, r := range releases {
		if r.Draft {
			continue
		}
		if _, e = ValidateTag(root, r.Tag); e != nil {
			continue
		}
		for _, a := range r.Assets {
			if a.Name == "release_manifest.v2.json" {
				candidates = append(candidates, r.Tag)
				break
			}
		}
	}
	slices.SortStableFunc(candidates, func(a, b string) int { return -CompareVersions(a[1:], b[1:]) })
	candidates = candidates[:min(30, len(candidates))]
	temp, e := os.MkdirTemp("", "raylea-mirror-")
	if e != nil {
		return 0, e
	}
	defer os.RemoveAll(temp)
	upload := func(path, key string, mutable bool) error {
		cache := "public,max-age=31536000,immutable"
		if mutable {
			cache = "public,max-age=300"
		}
		content := "application/octet-stream"
		if filepath.Ext(path) == ".json" {
			content = "application/json"
		}
		_, e := run("aws", "s3", "cp", path, "s3://"+o.Bucket+"/"+key, "--endpoint-url", o.Endpoint, "--cache-control", cache, "--content-type", content, "--only-show-errors")
		return e
	}
	var manifests []*ordered.Object
	for _, tag := range candidates {
		dir := filepath.Join(temp, tag)
		if e = os.Mkdir(dir, 0755); e != nil {
			return 0, e
		}
		if _, e = run("gh", "release", "download", tag, "--repo", o.Repository, "--pattern", "release_manifest.v2.json", "--dir", dir); e != nil {
			return 0, e
		}
		source, e := ordered.Read(filepath.Join(dir, "release_manifest.v2.json"))
		if e != nil {
			return 0, e
		}
		version := ordered.Str(source.Get("version"))
		if _, e = Channel(root, version, ""); e != nil {
			return 0, e
		}
		if CompareVersions(version, "0.4.0-0") < 0 {
			continue
		}
		if tag != "v"+version {
			return 0, errors.New("release tag and manifest version disagree")
		}
		mirrored, e := RewriteManifest(root, source, o.BaseURL)
		if e != nil {
			return 0, e
		}
		raw, e := run("aws", "s3api", "list-objects-v2", "--bucket", o.Bucket, "--prefix", tag+"/", "--endpoint-url", o.Endpoint, "--output", "json")
		if e != nil {
			return 0, e
		}
		var inventory struct {
			Contents []struct {
				Key  string
				Size int64
			}
		}
		if e = json.Unmarshal([]byte(raw), &inventory); e != nil {
			return 0, e
		}
		existing := map[string]int64{}
		for _, item := range inventory.Contents {
			existing[item.Key] = item.Size
		}
		for _, v := range ordered.List(source.Get("artifacts")) {
			a := ordered.Obj(v)
			name := ordered.Str(a.Get("file_name"))
			sizeJSON, _ := json.Marshal(a.Get("archive_size_bytes"))
			var size int64
			if e = json.Unmarshal(sizeJSON, &size); e != nil {
				return 0, e
			}
			if previous, ok := existing[tag+"/"+name]; ok && previous == size {
				continue
			}
			if _, e = run("gh", "release", "download", tag, "--repo", o.Repository, "--pattern", name, "--dir", dir); e != nil {
				return 0, e
			}
			path := filepath.Join(dir, name)
			info, e := os.Stat(path)
			if e != nil {
				return 0, e
			}
			if info.Size() != size {
				return 0, fmt.Errorf("archive size mismatch: %s/%s", tag, name)
			}
			if e = upload(path, tag+"/"+name, false); e != nil {
				return 0, e
			}
			if e = os.Remove(path); e != nil {
				return 0, e
			}
		}
		path := filepath.Join(dir, "release_manifest.v2.json")
		if e = writeCompactJSON(path, mirrored); e != nil {
			return 0, e
		}
		if e = upload(path, tag+"/release_manifest.v2.json", false); e != nil {
			return 0, e
		}
		manifests = append(manifests, mirrored)
	}
	// Mutable channel pointers are written only after every indexed asset exists.
	docs, e := ChannelDocuments(root, manifests)
	if e != nil {
		return 0, e
	}
	for _, f := range *docs {
		path := filepath.Join(temp, f.Name)
		if e = writeCompactJSON(path, f.Value); e != nil {
			return 0, e
		}
		if e = upload(path, f.Name, true); e != nil {
			return 0, e
		}
	}
	return len(manifests), nil
}
func writeCompactJSON(path string, v any) error {
	b, e := singleLineJSON(v)
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0644)
}
func RunMirror(args []string, out, stderr io.Writer) int {
	fs := flags("sync-update-mirror", stderr)
	o := MirrorOptions{}
	fs.StringVar(&o.Repository, "repository", "", "GitHub repository")
	fs.StringVar(&o.BaseURL, "base-url", "", "public HTTPS mirror base")
	fs.StringVar(&o.Bucket, "bucket", "", "S3 bucket")
	fs.StringVar(&o.Endpoint, "endpoint", "", "HTTPS object storage endpoint")
	if e := parse(fs, args, "repository", "base-url", "bucket", "endpoint"); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	count, e := SyncMirror(repo.Root(), o, mirrorCommand)
	return result(out, stderr, fmt.Sprintf("Published %d complete releases to %s", count, o.BaseURL), e)
}
