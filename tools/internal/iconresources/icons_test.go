package iconresources

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func TestSourceAndResourceComparison(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repo.Root(), "launcher/assets/icon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := sourceFrames(data)
	if err != nil {
		t.Fatal(err)
	}
	group := resourceGroup{Name: 1, Language: 1033, Data: make([]byte, 6), Icons: map[uint16][]byte{}}
	binary.LittleEndian.PutUint16(group.Data[2:], 1)
	binary.LittleEndian.PutUint16(group.Data[4:], uint16(len(sizes)))
	var frames []frame
	for i, size := range sizes {
		f := source[size]
		entry := append([]byte{}, f.Directory...)
		entry = binary.LittleEndian.AppendUint16(entry, uint16(i+1))
		group.Data = append(group.Data, entry...)
		group.Icons[uint16(i+1)] = f.Payload
		frames = append(frames, f)
	}
	_, failures, extracted, err := compare(source, []resourceGroup{group})
	if err != nil || len(failures) != 0 {
		t.Fatalf("%v %v", failures, err)
	}
	decoded, err := sourceFrames(encodeICO(extracted))
	if err != nil {
		t.Fatal(err)
	}
	for size, f := range source {
		if !bytes.Equal(decoded[size].Payload, f.Payload) || !bytes.Equal(decoded[size].Directory, f.Directory) {
			t.Fatalf("round trip frame %d", size)
		}
	}
	group.Icons[1] = []byte("wrong payload")
	_, failures, _, err = compare(source, []resourceGroup{group})
	if err != nil || len(failures) == 0 {
		t.Fatal("accepted different payload")
	}
	group.Icons[1] = frames[0].Payload
	group.Data[6+6] ^= 1
	_, failures, _, err = compare(source, []resourceGroup{group})
	if err != nil || len(failures) == 0 {
		t.Fatal("accepted different directory")
	}
	for _, invalid := range [][]byte{nil, data[:5], data[:10]} {
		if _, err := sourceFrames(invalid); err == nil {
			t.Fatal("accepted truncated ICO")
		}
	}
	if _, err := groupEntries(group.Data[:len(group.Data)-1]); err == nil {
		t.Fatal("accepted truncated group")
	}
	_, failures, _, err = compare(source, nil)
	if err != nil || len(failures) != 1 {
		t.Fatal("accepted missing resources")
	}
}
