package runtime

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadProtocolLinePreservesBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		bufferSize int
		limit      int
		want       string
		wantErr    error
	}{
		{name: "empty line", input: "\n", bufferSize: 16, limit: 1, want: "\n"},
		{name: "default limit", input: "{}\n", bufferSize: 16, want: "{}\n"},
		{name: "exact limit", input: "12345678\n", bufferSize: 16, limit: 8, want: "12345678\n"},
		{name: "over limit", input: "123456789\n", bufferSize: 16, limit: 8, wantErr: errProtocolFrameTooLarge},
		{name: "carriage return retained", input: "1234567\r\n", bufferSize: 16, limit: 8, want: "1234567\r\n"},
		{name: "carriage return counts toward limit", input: "12345678\r\n", bufferSize: 16, limit: 8, wantErr: errProtocolFrameTooLarge},
		{name: "fragmented exact limit", input: strings.Repeat("x", 32) + "\n", bufferSize: 16, limit: 32, want: strings.Repeat("x", 32) + "\n"},
		{name: "fragmented over limit", input: strings.Repeat("x", 33) + "\n", bufferSize: 16, limit: 32, wantErr: errProtocolFrameTooLarge},
		{name: "large frame", input: strings.Repeat("x", 64*1024) + "\n", bufferSize: 4096, limit: 64 * 1024, want: strings.Repeat("x", 64*1024) + "\n"},
		{name: "empty eof", bufferSize: 16, limit: 32, wantErr: io.EOF},
		{name: "partial eof", input: "12345678", bufferSize: 16, limit: 8, wantErr: io.EOF},
		{name: "oversized eof", input: "123456789", bufferSize: 16, limit: 8, wantErr: errProtocolFrameTooLarge},
		{name: "fragmented partial eof", input: strings.Repeat("x", 32), bufferSize: 16, limit: 32, wantErr: io.EOF},
		{name: "fragmented oversized eof", input: strings.Repeat("x", 33), bufferSize: 16, limit: 32, wantErr: errProtocolFrameTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reader := bufio.NewReaderSize(strings.NewReader(test.input), test.bufferSize)
			line, err := readProtocolLine(reader, test.limit)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil && line != nil {
				t.Fatalf("failed read retained %d bytes", len(line))
			}
			if string(line) != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

func TestReadProtocolLineReturnsOwnedBytes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		size int
	}{
		{name: "short frame", size: 8},
		{name: "fragmented frame", size: 32},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			first := strings.Repeat("a", test.size-1) + "\n"
			second := strings.Repeat("b", 32) + "\n"
			reader := bufio.NewReaderSize(strings.NewReader(first+second), 16)
			line, err := readProtocolLine(reader, 64)
			if err != nil {
				t.Fatal(err)
			}
			next, err := readProtocolLine(reader, 64)
			if err != nil {
				t.Fatal(err)
			}
			if string(line) != first || string(next) != second {
				t.Fatalf("reader overwrite affected returned lines: first=%q, second=%q", line, next)
			}
			line[0] = 'z'
			if string(next) != second {
				t.Fatal("returned lines share writable bytes")
			}
		})
	}
}
