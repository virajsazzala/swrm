package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/virajsazzala/swrm/internal/api"
)

func TestReorderArgsFlagsBeforePositional(t *testing.T) {
	args := []string{"-output-dir", "/tmp/x", "-socket", "/tmp/y.sock", "-d", "file.torrent"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"-output-dir", "/tmp/x", "-socket", "/tmp/y.sock", "-d", "file.torrent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReorderArgsFlagsAfterPositional(t *testing.T) {
	args := []string{"file.torrent", "-output-dir", "/tmp/x", "-socket", "/tmp/y.sock", "-d"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"-output-dir", "/tmp/x", "-socket", "/tmp/y.sock", "-d", "file.torrent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReorderArgsInterspersed(t *testing.T) {
	args := []string{"-output-dir", "/tmp/x", "file.torrent", "-d"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"-output-dir", "/tmp/x", "-d", "file.torrent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReorderArgsEqualsForm(t *testing.T) {
	args := []string{"file.torrent", "-output-dir=/tmp/x"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"-output-dir=/tmp/x", "file.torrent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReorderArgsNoFlags(t *testing.T) {
	args := []string{"file.torrent"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"file.torrent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReorderArgsNoPositional(t *testing.T) {
	args := []string{"-socket", "/tmp/y.sock", "-d"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"-socket", "/tmp/y.sock", "-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReorderArgsBoolFlagDoesNotConsumePositional(t *testing.T) {
	args := []string{"-d", "file.torrent"}
	got := reorderArgs(args, map[string]bool{"d": true})
	want := []string{"-d", "file.torrent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestIsTerminalStatus(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"announcing", false},
		{"downloading", false},
		{"reconnecting", false},
		{"completed", true},
		{"error", true},
		{"stopped", true},
		{"", false},
	}
	for _, c := range cases {
		if got := isTerminalStatus(c.status); got != c.want {
			t.Errorf("isTerminalStatus(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestWaitForSocketSucceedsOnceListening(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	go func() {
		time.Sleep(300 * time.Millisecond)
		ln, err := net.Listen("unix", sockPath)
		if err != nil {
			return
		}
		defer ln.Close()
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	if !waitForSocket(sockPath, 3*time.Second) {
		t.Fatal("expected waitForSocket to succeed once the listener comes up")
	}
}

func TestWaitForSocketTimesOutWhenNothingListens(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "never-listens.sock")

	if waitForSocket(sockPath, 500*time.Millisecond) {
		t.Fatal("expected waitForSocket to time out when nothing ever listens")
	}
}

func TestFindSwrmdViaPath(t *testing.T) {
	dir := t.TempDir()
	fakeSwrmd := filepath.Join(dir, "swrmd")
	if err := os.WriteFile(fakeSwrmd, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write fake swrmd: %v", err)
	}

	t.Setenv("PATH", dir)

	got, err := findSwrmd()
	if err != nil {
		t.Fatalf("findSwrmd: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", got, err)
	}
	wantResolved, err := filepath.EvalSymlinks(fakeSwrmd)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", fakeSwrmd, err)
	}
	if resolved != wantResolved {
		t.Fatalf("findSwrmd() = %q, want %q", got, fakeSwrmd)
	}
}

func TestFindSwrmdNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := findSwrmd(); err == nil {
		t.Fatal("expected an error when swrmd cannot be found anywhere")
	}
}

func TestFindSwrmdViaSiblingDirectory(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}

	sibling := filepath.Join(filepath.Dir(exe), "swrmd")
	if _, err := os.Stat(sibling); err == nil {
		t.Skip("a swrmd binary already exists next to the test binary")
	}

	if err := os.WriteFile(sibling, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Skipf("cannot write into the test binary's directory: %v", err)
	}
	t.Cleanup(func() { os.Remove(sibling) })

	t.Setenv("PATH", t.TempDir())

	got, err := findSwrmd()
	if err != nil {
		t.Fatalf("findSwrmd: %v", err)
	}
	if got != sibling {
		t.Fatalf("findSwrmd() = %q, want %q", got, sibling)
	}
}

func TestPrintProgressLine(t *testing.T) {
	var buf bytes.Buffer
	printProgressLine(&buf, &api.StatusResponse{
		Name:          "Big Buck Bunny",
		Status:        "downloading",
		Completed:     50,
		Total:         100,
		ActiveWorkers: 4,
		BytesPerSec:   123456,
		ETASeconds:    30,
	})

	out := buf.String()
	for _, want := range []string{"Big Buck Bunny", "downloading", "50.00%", "(50/100)", "123456", "30s", "workers 4"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestPrintProgressLineUnknownETA(t *testing.T) {
	var buf bytes.Buffer
	printProgressLine(&buf, &api.StatusResponse{Status: "announcing", ETASeconds: -1})

	if !strings.Contains(buf.String(), "unknown") {
		t.Errorf("output %q should show eta as unknown", buf.String())
	}
}

func TestPrintStatusBlock(t *testing.T) {
	var buf bytes.Buffer
	printStatusBlock(&buf, &api.StatusResponse{
		Name:          "Big Buck Bunny",
		Status:        "error",
		Completed:     10,
		Total:         100,
		Pending:       90,
		ActiveWorkers: 0,
		BytesPerSec:   0,
		ETASeconds:    -1,
		LastError:     "no peers connected",
	})

	out := buf.String()
	for _, want := range []string{"Big Buck Bunny", "error", "10/100", "pending 90", "unknown", "no peers connected"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestDefaultSocketPathIsUnderHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	got := defaultSocketPath()
	if !strings.HasPrefix(got, home) {
		t.Fatalf("defaultSocketPath() = %q, want prefix %q", got, home)
	}
	if filepath.Base(got) != "swrmd.sock" {
		t.Fatalf("defaultSocketPath() = %q, want basename swrmd.sock", got)
	}
}
