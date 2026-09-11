package fsguard

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestCopyExactRejectsMismatchAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		size        int64
		valid       bool
	}{
		{"exact", "abc", 3, true}, {"empty", "", 0, true},
		{"short", "ab", 3, false}, {"long", "abcd", 3, false}, {"negative", "", -1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := CopyExact(context.Background(), &out, strings.NewReader(tc.input), tc.size)
			if (err == nil) != tc.valid {
				t.Fatalf("error = %v", err)
			}
			if tc.size >= 0 && int64(out.Len()) > tc.size {
				t.Fatal("wrote beyond declared size")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CopyExact(ctx, io.Discard, strings.NewReader("abc"), 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled copy: %v", err)
	}
	checksumErr := errors.New("checksum mismatch")
	if err := CopyExact(context.Background(), io.Discard, &failedFooter{err: checksumErr}, 1); !errors.Is(err, checksumErr) {
		t.Fatalf("checksum error was lost: %v", err)
	}
}

type failedFooter struct {
	read bool
	err  error
}

func (r *failedFooter) Read(p []byte) (int, error) {
	if r.read {
		return 0, r.err
	}
	r.read = true
	p[0] = 'a'
	return 1, nil
}
