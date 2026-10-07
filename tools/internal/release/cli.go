package release

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func flags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
func parse(fs *flag.FlagSet, args []string, required ...string) error {
	if e := cli.Parse(fs, args, 0, 0); e != nil {
		return e
	}
	for _, name := range required {
		if fs.Lookup(name).Value.String() == "" {
			return fmt.Errorf("--%s is required", name)
		}
	}
	return nil
}
func result(out, stderr io.Writer, message string, e error) int {
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	if message != "" {
		fmt.Fprintln(out, message)
	}
	return 0
}
func RunNotes(args []string, out, stderr io.Writer) int {
	fs := flags("check-release-notes", stderr)
	tag := fs.String("tag", "", "release tag")
	dir := fs.String("notes-dir", filepath.Join(repo.Root(), "docs/release/notes"), "release notes directory")
	if e := parse(fs, args, "tag"); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	p, e := ValidateNotes(repo.Root(), *tag, *dir)
	if e != nil {
		e = fmt.Errorf("release notes check failed: %w", e)
	}
	return result(out, stderr, "release notes checked: "+p, e)
}
func RunPolicy(args []string, out, stderr io.Writer) int {
	fs := flags("release-policy", stderr)
	tag := fs.String("tag", "", "release tag")
	channel := fs.String("channel", "", "stable or beta")
	dir := fs.String("notes-dir", filepath.Join(repo.Root(), "docs/release/notes"), "release notes directory")
	output := fs.String("github-output", "", "append GitHub outputs")
	if e := parse(fs, args, "tag"); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	if *channel != "" && *channel != "stable" && *channel != "beta" {
		return cli.ErrorTo(stderr, errors.New("--channel must be stable or beta"))
	}
	settings, e := PublicationSettings(repo.Root(), *tag)
	if e == nil {
		_, e = Channel(repo.Root(), settings.Value, *channel)
	}
	if e == nil {
		_, e = ValidateNotes(repo.Root(), *tag, *dir)
	}
	if e == nil {
		p := *output
		if p == "" {
			p = os.Getenv("GITHUB_OUTPUT")
		}
		if p != "" {
			e = WritePublication(p, settings)
		}
	}
	if e != nil {
		return result(out, stderr, "", fmt.Errorf("release policy check failed: %w", e))
	}
	b, e := singleLineJSON(settings)
	if e == nil {
		fmt.Fprintln(out, string(b))
	}
	return result(out, stderr, "", e)
}
func packageFlags(fs *flag.FlagSet, defaults bool) *PackageOptions {
	o := &PackageOptions{}
	fs.StringVar(&o.ArtifactID, "artifact-id", "", "release artifact ID")
	fs.StringVar(&o.Version, "version", "", "release version")
	fs.StringVar(&o.GitCommit, "git-commit", "", "commit SHA")
	if !defaults {
		fs.StringVar(&o.BuiltAt, "built-at", "", "build timestamp")
	}
	fs.StringVar(&o.ServerBin, "server-bin", "", "Server executable")
	web, deps, templates, output := "", "", "", ""
	if defaults {
		web, deps, templates, output = "web/dist", ".deps", "templates", "dist/release"
	}
	fs.StringVar(&o.WebDist, "web-dist", web, "built Web directory")
	fs.StringVar(&o.DepsDir, "deps-dir", deps, "managed resource manifest directory")
	fs.StringVar(&o.TemplatesDir, "templates-dir", templates, "template directory")
	fs.StringVar(&o.OutputDir, "output-dir", output, "release output directory")
	fs.StringVar(&o.LauncherBundle, "launcher-bundle", "", "Launcher bundle")
	fs.StringVar(&o.SystemdFile, "systemd-file", "", "systemd service")
	fs.StringVar(&o.ReleaseNotesRef, "release-notes-ref", "", "release notes URL")
	fs.StringVar(&o.LicenseFile, "license-file", "LICENSE", "project license")
	fs.StringVar(&o.ThirdPartyNotices, "third-party-notices", "THIRD_PARTY_NOTICES.md", "third-party notices")
	return o
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }
func artifactChoice(id string) error {
	matrix, e := Matrix(repo.Root())
	if e != nil {
		return e
	}
	if _, ok := matrix[id]; !ok {
		return fmt.Errorf("unsupported artifact_id: %s", id)
	}
	return nil
}
func RunTool(args []string, out, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.ErrorTo(stderr, errors.New("release-tool requires package, metadata or versions"))
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, "Usage: release-tool {package|metadata|versions} [options]")
		return 0
	}
	fs := flags("release-tool "+args[0], stderr)
	switch args[0] {
	case "package":
		o := packageFlags(fs, false)
		if e := parse(fs, args[1:], "artifact-id", "version", "git-commit", "server-bin", "web-dist", "deps-dir", "templates-dir", "output-dir"); e != nil {
			return cli.ErrorTo(stderr, e)
		}
		if e := artifactChoice(o.ArtifactID); e != nil {
			return cli.ErrorTo(stderr, e)
		}
		s, e := Stage(repo.Root(), *o)
		return result(out, stderr, filepath.Join(o.OutputDir, s.FileName), e)
	case "metadata":
		o := MetadataOptions{}
		for _, v := range []struct {
			p *string
			n string
		}{{&o.Version, "version"}, {&o.GitCommit, "git-commit"}, {&o.BuiltAt, "built-at"}, {&o.ConfigSchemaVersion, "config-schema-version"}, {&o.DBSchemaVersion, "db-schema-version"}, {&o.PluginProtocolVersion, "plugin-protocol-version"}, {&o.ReleaseNotesRef, "release-notes-ref"}, {&o.DownloadBaseURL, "download-base-url"}, {&o.OutputDir, "output-dir"}, {&o.Channel, "channel"}, {&o.PublishedAt, "published-at"}} {
			fs.StringVar(v.p, v.n, "", v.n)
		}
		var paths stringList
		fs.Var(&paths, "sidecar", "artifact sidecar (repeatable)")
		if e := parse(fs, args[1:], "version", "git-commit", "config-schema-version", "db-schema-version", "plugin-protocol-version", "release-notes-ref", "download-base-url", "sidecar", "output-dir"); e != nil {
			return cli.ErrorTo(stderr, e)
		}
		if o.Channel != "" && o.Channel != "stable" && o.Channel != "beta" {
			return cli.ErrorTo(stderr, errors.New("--channel must be stable or beta"))
		}
		var sidecars []Sidecar
		for _, p := range paths {
			s, e := LoadSidecar(p)
			if e != nil {
				return result(out, stderr, "", e)
			}
			sidecars = append(sidecars, s)
		}
		p, e := BuildMetadata(repo.Root(), o, sidecars)
		return result(out, stderr, p, e)
	case "versions":
		field := fs.String("field", "", "print one version: config, database, plugin-protocol, plugin-manifest")
		if e := parse(fs, args[1:]); e != nil {
			return cli.ErrorTo(stderr, e)
		}
		values, e := Versions(repo.Root())
		if e != nil {
			return result(out, stderr, "", e)
		}
		if *field != "" {
			v, ok := values[*field]
			if !ok {
				return cli.ErrorTo(stderr, fmt.Errorf("unknown version field: %s", *field))
			}
			fmt.Fprintln(out, v)
		} else {
			b, e := ordered.JSON(values, true)
			if e != nil {
				return result(out, stderr, "", e)
			}
			fmt.Fprint(out, string(b))
		}
		return 0
	default:
		return cli.ErrorTo(stderr, fmt.Errorf("unknown release-tool command: %s", args[0]))
	}
}
func Versions(root string) (map[string]string, error) {
	values := map[string]string{}
	for _, v := range [][3]string{{"config", "config.user.schema.json", "/properties/schema_version/const"}, {"plugin-protocol", "plugin-protocol.schema.json", "/$defs/init/allOf/1/properties/protocol_version/const"}, {"plugin-manifest", "plugin-info.schema.json", "/properties/manifest_version/const"}} {
		value, e := contractdata.Value(root, v[1], v[2])
		if e != nil {
			return nil, e
		}
		values[v[0]] = ordered.Str(value)
	}
	file, e := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "server/internal/storage/store_schema.go"), nil, 0)
	if e != nil {
		return nil, e
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			v := spec.(*ast.ValueSpec)
			for i, name := range v.Names {
				if name.Name == "currentSchemaVersion" && i < len(v.Values) {
					literal, ok := v.Values[i].(*ast.BasicLit)
					if ok && literal.Kind == token.STRING {
						values["database"], e = strconv.Unquote(literal.Value)
						if e != nil {
							return nil, e
						}
					}
				}
			}
		}
	}
	if values["database"] == "" {
		return nil, errors.New("currentSchemaVersion string constant not found")
	}
	return values, nil
}
func RunSmoke(args []string, out, stderr io.Writer) int {
	fs := flags("smoke-release", stderr)
	id := fs.String("artifact-id", "", "release artifact ID")
	archive := fs.String("archive", "", "release archive")
	if e := parse(fs, args, "artifact-id", "archive"); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	if e := artifactChoice(*id); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	return result(out, stderr, "release smoke passed", Smoke(repo.Root(), *id, *archive))
}
