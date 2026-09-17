package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintConsoleStatus(t *testing.T) {
	t.Setenv("WT_SESSION", "")
	var output bytes.Buffer
	printConsoleStatus(&output, "127.0.0.1:28741", "data/frameflow.log")
	for _, expected := range []string{"FrameFlow", "正在运行", "http://127.0.0.1:28741", "data/frameflow.log"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("status missing %q: %s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatal("plain console output contains terminal color codes")
	}
	t.Setenv("WT_SESSION", "windows-terminal")
	output.Reset()
	printConsoleStatus(&output, "127.0.0.1:28741", "data/frameflow.log")
	if !strings.Contains(output.String(), "\x1b[1;38;2;30;190;171mFrameFlow") {
		t.Fatal("Windows Terminal output is missing brand color")
	}
}
