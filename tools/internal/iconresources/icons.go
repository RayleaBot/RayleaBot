package iconresources

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

var sizes = []int{16, 24, 32, 48, 64, 128, 256}

type frame struct{ Directory, Payload []byte }
type resourceGroup struct {
	Name     any
	Language uint16
	Data     []byte
	Icons    map[uint16][]byte
}
type frameReport struct {
	Type           string  `json:"resource_type"`
	ID             uint16  `json:"id"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Depth          uint16  `json:"bit_depth"`
	Bytes          int     `json:"bytes"`
	SHA            string  `json:"sha256"`
	SourceSHA      *string `json:"source_sha256"`
	PayloadEqual   bool    `json:"payload_equal"`
	DirectoryEqual bool    `json:"directory_equal"`
}
type groupReport struct {
	Type     string        `json:"resource_type"`
	ID       any           `json:"id"`
	Language uint16        `json:"language"`
	Frames   []frameReport `json:"frames"`
}
type report struct {
	CheckedAt string        `json:"checked_at"`
	Method    string        `json:"method"`
	EXE       string        `json:"exe"`
	EXESHA    string        `json:"exe_sha256"`
	ICO       string        `json:"source_ico"`
	ICOSHA    string        `json:"source_ico_sha256"`
	Sizes     []int         `json:"expected_sizes"`
	Groups    []groupReport `json:"groups"`
	Errors    []string      `json:"errors"`
	Passed    bool          `json:"passed"`
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func dimension(b byte) int {
	if b == 0 {
		return 256
	}
	return int(b)
}
func sourceFrames(data []byte) (map[int]frame, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, fmt.Errorf("Source file is not an ICO")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if len(data) < 6+count*16 {
		return nil, fmt.Errorf("Invalid source ICO directory")
	}
	frames := map[int]frame{}
	for i := 0; i < count; i++ {
		entry := data[6+i*16 : 22+i*16]
		size := dimension(entry[0])
		length, offset := uint64(binary.LittleEndian.Uint32(entry[8:])), uint64(binary.LittleEndian.Uint32(entry[12:]))
		if _, exists := frames[size]; exists || size != dimension(entry[1]) || offset+length > uint64(len(data)) {
			return nil, fmt.Errorf("Invalid source ICO frame")
		}
		frames[size] = frame{entry[:12], data[offset : offset+length]}
	}
	var actual []int
	for size := range frames {
		actual = append(actual, size)
	}
	slices.Sort(actual)
	if !slices.Equal(actual, sizes) {
		return nil, fmt.Errorf("Source ICO sizes differ from %v", sizes)
	}
	return frames, nil
}
func groupEntries(data []byte) ([][]byte, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, fmt.Errorf("Invalid RT_GROUP_ICON data")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if len(data) != 6+count*14 {
		return nil, fmt.Errorf("Invalid RT_GROUP_ICON data")
	}
	entries := make([][]byte, count)
	for i := range entries {
		entries[i] = data[6+i*14 : 20+i*14]
	}
	return entries, nil
}
func compare(source map[int]frame, groups []resourceGroup) ([]groupReport, []string, []frame, error) {
	reports := []groupReport{}
	failures := []string{}
	var extracted []frame
	for _, g := range groups {
		entries, err := groupEntries(g.Data)
		if err != nil {
			return nil, nil, nil, err
		}
		r := groupReport{"RT_GROUP_ICON", g.Name, g.Language, []frameReport{}}
		var actual []int
		var frames []frame
		for _, e := range entries {
			size := dimension(e[0])
			id := binary.LittleEndian.Uint16(e[12:])
			payload := g.Icons[id]
			expected, exists := source[size]
			equal := exists && bytes.Equal(payload, expected.Payload)
			directoryEqual := exists && bytes.Equal(e[:12], expected.Directory)
			var sourceSHA *string
			if exists {
				s := digest(expected.Payload)
				sourceSHA = &s
			}
			r.Frames = append(r.Frames, frameReport{"RT_ICON", id, size, dimension(e[1]), binary.LittleEndian.Uint16(e[6:]), len(payload), digest(payload), sourceSHA, equal, directoryEqual})
			if !equal || !directoryEqual || uint64(binary.LittleEndian.Uint32(e[8:])) != uint64(len(payload)) {
				failures = append(failures, fmt.Sprintf("Group %v, language %d, frame %dpx differs from source ICO", g.Name, g.Language, size))
			}
			actual = append(actual, size)
			frames = append(frames, frame{e[:12], payload})
		}
		slices.Sort(actual)
		if !slices.Equal(actual, sizes) {
			failures = append(failures, fmt.Sprintf("Group %v, language %d has unexpected sizes", g.Name, g.Language))
		}
		reports = append(reports, r)
		if len(extracted) == 0 {
			extracted = frames
		}
	}
	if len(reports) == 0 {
		failures = append(failures, "No RT_GROUP_ICON resources found")
	}
	return reports, failures, extracted, nil
}
func encodeICO(frames []frame) []byte {
	data := make([]byte, 6+16*len(frames))
	binary.LittleEndian.PutUint16(data[2:], 1)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(frames)))
	for i, f := range frames {
		copy(data[6+i*16:], f.Directory)
		binary.LittleEndian.PutUint32(data[18+i*16:], uint32(len(data)))
		data = append(data, f.Payload...)
	}
	return data
}
func verify(exe, ico, evidence string, out io.Writer) error {
	exe, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	ico, err = filepath.Abs(ico)
	if err != nil {
		return err
	}
	exeBytes, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	icoBytes, err := os.ReadFile(ico)
	if err != nil {
		return err
	}
	source, err := sourceFrames(icoBytes)
	if err != nil {
		return err
	}
	groups, err := loadGroups(exe)
	if err != nil {
		return err
	}
	r := report{CheckedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00"), Method: "LoadLibraryExW(LOAD_LIBRARY_AS_DATAFILE | LOAD_LIBRARY_AS_IMAGE_RESOURCE), EnumResourceNamesW, EnumResourceLanguagesW, FindResourceExW", EXE: exe, EXESHA: digest(exeBytes), ICO: ico, ICOSHA: digest(icoBytes), Sizes: sizes}
	var extracted []frame
	r.Groups, r.Errors, extracted, err = compare(source, groups)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if digest(current) != r.EXESHA {
		r.Errors = append(r.Errors, "EXE changed during resource verification")
	}
	r.Passed = len(r.Errors) == 0
	if evidence != "" {
		if err := os.MkdirAll(evidence, 0755); err != nil {
			return err
		}
		for _, f := range extracted {
			if f.Directory[0] == 0 && bytes.HasPrefix(f.Payload, []byte("\x89PNG\r\n\x1a\n")) {
				if err := os.WriteFile(filepath.Join(evidence, "launcher-exe-icon-256.png"), f.Payload, 0644); err != nil {
					return err
				}
			}
		}
		if err := os.WriteFile(filepath.Join(evidence, "launcher-exe-icons.ico"), encodeICO(extracted), 0644); err != nil {
			return err
		}
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(evidence, "native-icon-resources.json"), append(b, '\n'), 0644); err != nil {
			return err
		}
	}
	if !r.Passed {
		return fmt.Errorf("%s", strings.Join(r.Errors, "\n"))
	}
	fmt.Fprintf(out, "Verified %d RT_GROUP_ICON resource(s): 16,24,32,48,64,128,256px; every RT_ICON payload and directory matches source ICO.\nEXE SHA256: %s\n", len(r.Groups), r.EXESHA)
	if evidence != "" {
		absolute, err := filepath.Abs(evidence)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Evidence:", absolute)
	}
	return nil
}
func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify-windows-icon-resources", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cli.Usage(fs, "verify-windows-icon-resources [--exe EXE] [--ico ICO] [--evidence-dir DIR]", "Compare final EXE icon resources with the launcher source ICO using read-only Windows resource APIs.")
	exe := fs.String("exe", filepath.Join(repo.Root(), "launcher/dist/package/win-unpacked/RayleaLauncher.exe"), "EXE to inspect")
	ico := fs.String("ico", filepath.Join(repo.Root(), "launcher/assets/icon.ico"), "source ICO")
	evidence := fs.String("evidence-dir", "", "directory for extracted icons and JSON report")
	if err := cli.Parse(fs, args, 0, 0); err != nil {
		return cli.ErrorTo(stderr, err)
	}
	if runtime.GOOS != "windows" {
		fmt.Fprintln(stderr, "Native Windows resource verification requires Windows")
		return 2
	}
	if err := verify(*exe, *ico, *evidence, out); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
