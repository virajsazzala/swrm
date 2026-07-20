package tracker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/virajsazzala/swrm/internal/torrent"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validResponseBody(interval int64, peers string) []byte {
	return []byte("d" + bStr("interval") + bInt(interval) + bStr("peers") + bStr(peers) + "e")
}

func testPeerID() [20]byte {
	var id [20]byte
	copy(id[:], "ABCDEFGHIJKLMNOPQRST")
	return id
}

func TestEventValues(t *testing.T) {
	cases := []struct {
		event    Event
		wantHTTP string
		wantUDP  uint32
	}{
		{EventNone, "", 0},
		{EventStarted, "started", 2},
		{EventCompleted, "completed", 1},
		{EventStopped, "stopped", 3},
	}
	for _, c := range cases {
		if got := c.event.httpValue(); got != c.wantHTTP {
			t.Errorf("Event(%d).httpValue() = %q, want %q", c.event, got, c.wantHTTP)
		}
		if got := c.event.udpValue(); got != c.wantUDP {
			t.Errorf("Event(%d).udpValue() = %d, want %d", c.event, got, c.wantUDP)
		}
	}
}

func TestAnnounceHTTPRequestParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write(validResponseBody(1800, ""))
	}))
	defer srv.Close()

	tor := &torrent.Torrent{Announce: srv.URL, Length: 12345}
	peerID := testPeerID()

	_, err := Announce(context.Background(), discardLogger(), tor, peerID, 6881, EventStarted)
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	if gotQuery.Get("port") != "6881" {
		t.Errorf("port = %q, want 6881", gotQuery.Get("port"))
	}
	if gotQuery.Get("left") != "12345" {
		t.Errorf("left = %q, want 12345", gotQuery.Get("left"))
	}
	if gotQuery.Get("compact") != "1" {
		t.Errorf("compact = %q, want 1", gotQuery.Get("compact"))
	}
	if gotQuery.Get("event") != "started" {
		t.Errorf("event = %q, want started", gotQuery.Get("event"))
	}
	if gotQuery.Get("info_hash") != string(tor.InfoHash[:]) {
		t.Errorf("info_hash mismatch")
	}
	if gotQuery.Get("peer_id") != string(peerID[:]) {
		t.Errorf("peer_id mismatch")
	}
}

func TestAnnounceHTTPEventNoneOmitsParam(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write(validResponseBody(1800, ""))
	}))
	defer srv.Close()

	tor := &torrent.Torrent{Announce: srv.URL, Length: 100}

	_, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone)
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	if _, present := gotQuery["event"]; present {
		t.Errorf("expected no 'event' param for EventNone, got %q", gotQuery.Get("event"))
	}
}

func TestAnnounceMultiTrackerFallback(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(validResponseBody(60, ""))
	}))
	defer working.Close()

	tor := &torrent.Torrent{
		Announce:     dead.URL,
		AnnounceList: [][]string{{working.URL}},
	}

	resp, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone)
	if err != nil {
		t.Fatalf("expected fallback to the working tracker to succeed, got: %v", err)
	}
	if resp.Interval != 60 {
		t.Errorf("Interval = %d, want 60", resp.Interval)
	}
}

func TestAnnounceAllTrackersFail(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	tor := &torrent.Torrent{Announce: dead.URL}

	if _, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone); err == nil {
		t.Fatal("expected an error when every tracker fails")
	}
}

func TestAnnounceNoTrackerURLs(t *testing.T) {
	tor := &torrent.Torrent{}
	if _, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone); err == nil {
		t.Fatal("expected an error for a torrent with no tracker urls")
	}
}

func TestAnnounceHTTPFailureReasonPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("d" + bStr("failure reason") + bStr("unregistered torrent") + "e"))
	}))
	defer srv.Close()

	tor := &torrent.Torrent{Announce: srv.URL}

	_, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone)
	if err == nil {
		t.Fatal("expected failure reason to propagate as an error")
	}
}

func TestAnnounceHTTPBodySizeCap(t *testing.T) {
	oversized := bytes.Repeat([]byte("x"), maxAnnounceBodySize+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(oversized)
	}))
	defer srv.Close()

	tor := &torrent.Torrent{Announce: srv.URL}

	if _, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone); err == nil {
		t.Fatal("expected an oversized tracker response to be rejected")
	}
}

func TestAnnouncePeerCountCapTruncates(t *testing.T) {
	var peers bytes.Buffer
	for i := 0; i < maxPeersPerAnnounce+50; i++ {
		peers.Write([]byte{127, 0, 0, 1, byte(i >> 8), byte(i)})
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(validResponseBody(60, peers.String()))
	}))
	defer srv.Close()

	tor := &torrent.Torrent{Announce: srv.URL}

	resp, err := Announce(context.Background(), discardLogger(), tor, testPeerID(), 6881, EventNone)
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}
	if len(resp.Peers) != maxPeersPerAnnounce {
		t.Fatalf("got %d peers, want truncation to %d", len(resp.Peers), maxPeersPerAnnounce)
	}
}
