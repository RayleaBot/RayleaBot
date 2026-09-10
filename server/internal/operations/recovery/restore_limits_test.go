package recovery

import (
	"archive/zip"
	"errors"
	"testing"
)

func TestRestoreRejectsArchiveResourceLimitsBeforeExtraction(t *testing.T) {
	for _, test := range []struct {
		name  string
		files []*zip.File
	}{
		{"entry count", make([]*zip.File, maxRestoreEntries+1)},
		{"single file", []*zip.File{{FileHeader: zip.FileHeader{Name: "data/large", UncompressedSize64: maxRestoreFileBytes + 1, CompressedSize64: maxRestoreFileBytes + 1}}}},
		{"compression ratio", []*zip.File{{FileHeader: zip.FileHeader{Name: "data/compressed", UncompressedSize64: maxRestoreRatio + 1, CompressedSize64: 1}}}},
		{"total size", []*zip.File{
			{FileHeader: zip.FileHeader{Name: "data/1", UncompressedSize64: maxRestoreFileBytes, CompressedSize64: maxRestoreFileBytes}},
			{FileHeader: zip.FileHeader{Name: "data/2", UncompressedSize64: maxRestoreFileBytes, CompressedSize64: maxRestoreFileBytes}},
			{FileHeader: zip.FileHeader{Name: "data/3", UncompressedSize64: maxRestoreFileBytes, CompressedSize64: maxRestoreFileBytes}},
			{FileHeader: zip.FileHeader{Name: "data/4", UncompressedSize64: maxRestoreFileBytes, CompressedSize64: maxRestoreFileBytes}},
			{FileHeader: zip.FileHeader{Name: "data/5", UncompressedSize64: 1, CompressedSize64: 1}},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := inspectRestoreArchive(test.files); !errors.Is(err, errRestoreResourceLimit) {
				t.Fatalf("resource limit was not rejected before reading entries: %v", err)
			}
		})
	}
}
