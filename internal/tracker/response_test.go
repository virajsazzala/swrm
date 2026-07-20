package tracker

import (
	"fmt"
	"strings"
	"testing"
)

func bStr(s string) string { return fmt.Sprintf("%d:%s", len(s), s) }
func bInt(i int64) string  { return fmt.Sprintf("i%de", i) }

func TestParseResponseSuccess(t *testing.T) {
	peers := string([]byte{
		192, 168, 1, 1, 0x1A, 0xE1,
		10, 0, 0, 1, 0x1A, 0xE2,
	})
	body := "d" + bStr("interval") + bInt(1800) + bStr("peers") + bStr(peers) + "e"

	resp, err := parseResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Interval != 1800 {
		t.Errorf("Interval = %d, want 1800", resp.Interval)
	}
	if len(resp.Peers) != 2 {
		t.Errorf("got %d peers, want 2", len(resp.Peers))
	}
}

func TestParseResponseFailureReason(t *testing.T) {
	body := "d" + bStr("failure reason") + bStr("unregistered torrent") + "e"

	_, err := parseResponse([]byte(body))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unregistered torrent") {
		t.Fatalf("expected failure reason in error, got: %v", err)
	}
}

func TestParseResponseMissingPeers(t *testing.T) {
	body := "d" + bStr("interval") + bInt(1800) + "e"
	if _, err := parseResponse([]byte(body)); err == nil {
		t.Fatal("expected error for missing peers field")
	}
}

func TestParseResponseMissingInterval(t *testing.T) {
	body := "d" + bStr("peers") + bStr("") + "e"
	if _, err := parseResponse([]byte(body)); err == nil {
		t.Fatal("expected error for missing interval field")
	}
}

func TestParseResponseNotADictionary(t *testing.T) {
	if _, err := parseResponse([]byte("le")); err == nil {
		t.Fatal("expected error for a non-dictionary response")
	}
}

func TestParseResponseMalformedCompactPeers(t *testing.T) {
	body := "d" + bStr("interval") + bInt(1800) + bStr("peers") + bStr("12345") + "e"
	if _, err := parseResponse([]byte(body)); err == nil {
		t.Fatal("expected error for a peers string not a multiple of 6 bytes")
	}
}
