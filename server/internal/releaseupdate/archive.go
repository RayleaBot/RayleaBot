package releaseupdate

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

func ExtractWindowsArtifact(archivePath, destinationRoot string, verified VerifiedManifest, artifact Artifact) (string, error) {
	if artifact.ArtifactID != "windows-x64-full" || artifact.Platform != "windows-x64" {
		return "", errorWithCode(CodeUpdateNotSupported, "extract artifact", errors.New("transactional extraction is limited to windows-x64-full"))
	}
	if err := VerifyArtifactFile(archivePath, artifact); err != nil {
		return "", err
	}
	if err := ensurePathHasNoSymlink(destinationRoot); err != nil {
		return "", errorWithCode(CodeArtifactInvalid, "validate staging path", err)
	}
	if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
		return "", errorWithCode(CodeArtifactInvalid, "create staging directory", err)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", errorWithCode(CodeArtifactInvalid, "open artifact ZIP", err)
	}
	defer reader.Close()
	if len(reader.File) == 0 || len(reader.File) > MaxArtifactFiles+1 {
		return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", errors.New("ZIP entry count is outside the allowed range"))
	}

	expectedRoot := fmt.Sprintf("RayleaBot-v%s-%s", verified.Manifest.Version, artifact.ArtifactID)
	caseFolded := make(map[string]string, len(reader.File))
	var expandedBytes int64
	fileCount := 0
	for _, entry := range reader.File {
		cleanName, rootName, err := validateWindowsZIPEntry(entry)
		if err != nil {
			return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", err)
		}
		if rootName != expectedRoot {
			return "", errorWithCode(CodeArtifactInvalid, "inspect artifact root", fmt.Errorf("expected root %q, found %q", expectedRoot, rootName))
		}
		caseKey := strings.ToLower(cleanName)
		if previous, duplicate := caseFolded[caseKey]; duplicate {
			return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", fmt.Errorf("case-insensitive path collision between %q and %q", previous, cleanName))
		}
		caseFolded[caseKey] = cleanName
		if entry.FileInfo().IsDir() {
			continue
		}
		fileCount++
		if entry.UncompressedSize64 > uint64(MaxExpandedBytes) {
			return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", fmt.Errorf("entry %q exceeds the expanded size limit", cleanName))
		}
		expandedBytes += int64(entry.UncompressedSize64)
		if expandedBytes > artifact.ExpandedSizeBytes || expandedBytes > MaxExpandedBytes {
			return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", errors.New("expanded artifact exceeds the signed size"))
		}
		if entry.UncompressedSize64 > 0 {
			if entry.CompressedSize64 == 0 || entry.UncompressedSize64 > entry.CompressedSize64*MaxCompressionRatio {
				return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", fmt.Errorf("entry %q exceeds the compression ratio limit", cleanName))
			}
		}
	}
	if fileCount != artifact.FileCount || expandedBytes != artifact.ExpandedSizeBytes {
		return "", errorWithCode(CodeArtifactInvalid, "inspect artifact inventory", fmt.Errorf("signed inventory is files=%d bytes=%d, ZIP inventory is files=%d bytes=%d", artifact.FileCount, artifact.ExpandedSizeBytes, fileCount, expandedBytes))
	}
	if expandedBytes > 0 && expandedBytes > artifact.ArchiveSizeBytes*MaxCompressionRatio {
		return "", errorWithCode(CodeArtifactInvalid, "inspect artifact ZIP", errors.New("artifact exceeds the total compression ratio limit"))
	}

	for _, entry := range reader.File {
		cleanName := strings.TrimSuffix(path.Clean(strings.ReplaceAll(entry.Name, "\\", "/")), "/")
		localized, err := filepath.Localize(cleanName)
		if err != nil || !filepath.IsLocal(localized) {
			return "", errorWithCode(CodeArtifactInvalid, "localize artifact path", fmt.Errorf("invalid path %q", entry.Name))
		}
		targetPath := filepath.Join(destinationRoot, localized)
		if !pathInside(destinationRoot, targetPath) {
			return "", errorWithCode(CodeArtifactInvalid, "extract artifact", fmt.Errorf("path %q escaped staging root", entry.Name))
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return "", errorWithCode(CodeArtifactInvalid, "create artifact directory", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", errorWithCode(CodeArtifactInvalid, "create artifact parent directory", err)
		}
		if err := extractZIPFile(entry, targetPath); err != nil {
			return "", errorWithCode(CodeArtifactInvalid, "extract artifact file", err)
		}
	}

	payloadRoot := filepath.Join(destinationRoot, expectedRoot)
	if err := validateExtractedBuildInfo(payloadRoot, verified, artifact); err != nil {
		return "", err
	}
	return payloadRoot, nil
}

func validateWindowsZIPEntry(entry *zip.File) (string, string, error) {
	rawName := strings.TrimSpace(entry.Name)
	if rawName == "" || strings.ContainsRune(rawName, '\x00') || strings.Contains(rawName, "\\") || strings.HasPrefix(rawName, "/") {
		return "", "", fmt.Errorf("unsafe ZIP entry path %q", entry.Name)
	}
	cleanName := strings.TrimSuffix(path.Clean(rawName), "/")
	if cleanName == "" || cleanName == "." || !slashPathIsLocal(cleanName) {
		return "", "", fmt.Errorf("unsafe ZIP entry path %q", entry.Name)
	}
	segments := strings.Split(cleanName, "/")
	for _, segment := range segments {
		if !safeWindowsPathSegment(segment) {
			return "", "", fmt.Errorf("unsafe Windows path segment %q", segment)
		}
	}
	mode := entry.Mode()
	if mode&os.ModeType != 0 && !mode.IsDir() {
		return "", "", fmt.Errorf("ZIP entry %q is not a regular file or directory", cleanName)
	}
	if entry.ExternalAttrs&0x400 != 0 {
		return "", "", fmt.Errorf("ZIP entry %q is a reparse point", cleanName)
	}
	return cleanName, segments[0], nil
}

func safeWindowsPathSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `<>:"|?*`) || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
		return false
	}
	for _, character := range segment {
		if character < 0x20 {
			return false
		}
	}
	base := segment
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	_, reserved := windowsReservedNames[strings.ToUpper(base)]
	return !reserved
}

func slashPathIsLocal(value string) bool {
	if value == "" || value == "." || strings.HasPrefix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func extractZIPFile(entry *zip.File, targetPath string) error {
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	limited := io.LimitReader(source, int64(entry.UncompressedSize64)+1)
	written, copyErr := io.Copy(target, limited)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != int64(entry.UncompressedSize64) {
		return fmt.Errorf("entry %q expanded to %d bytes, expected %d", entry.Name, written, entry.UncompressedSize64)
	}
	return nil
}

func validateExtractedBuildInfo(payloadRoot string, verified VerifiedManifest, artifact Artifact) error {
	payload, err := os.ReadFile(filepath.Join(payloadRoot, "build_info.json"))
	if err != nil {
		return errorWithCode(CodeArtifactInvalid, "read staged build_info.json", err)
	}
	buildInfo, err := DecodeBuildInfo(payload)
	if err != nil {
		return errorWithCode(CodeArtifactInvalid, "validate staged build_info.json", err)
	}
	if buildInfo.Version != verified.Manifest.Version || buildInfo.ArtifactID != artifact.ArtifactID || buildInfo.UpdateProtocolVersion < artifact.MinUpdaterProtocolVersion {
		return errorWithCode(CodeArtifactInvalid, "validate staged build identity", errors.New("artifact version, platform, or updater protocol does not match the signed manifest"))
	}
	if buildInfo.ReleaseManifestSHA256 != "" && buildInfo.ReleaseManifestSHA256 != verified.Digest {
		return errorWithCode(CodeArtifactInvalid, "validate staged manifest digest", errors.New("build_info manifest digest does not match the signed manifest"))
	}
	return nil
}

func ensurePathHasNoSymlink(candidate string) error {
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	current := absolute
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			reparse, reparseErr := isReparsePoint(current, info)
			if reparseErr != nil {
				return reparseErr
			}
			if reparse {
				return fmt.Errorf("path %q contains a symbolic link or reparse point", candidate)
			}
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func pathInside(root, candidate string) bool {
	absoluteRoot, rootErr := filepath.Abs(root)
	absoluteCandidate, candidateErr := filepath.Abs(candidate)
	if rootErr != nil || candidateErr != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteCandidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
