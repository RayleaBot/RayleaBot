package main

import (
	"os"
	"testing"
)

func TestRedirectedStderrIsNotInteractive(t *testing.T) {
	previous := os.Stderr
	defer func() { os.Stderr = previous }()
	file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	os.Stderr = file
	if isInteractiveConsole() {
		t.Fatal("null device misclassified as interactive console")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	defer func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	}()
	os.Stderr = writer
	if isInteractiveConsole() {
		t.Fatal("redirected output may expose setup token")
	}
}
