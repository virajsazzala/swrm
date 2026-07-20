package tracker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/virajsazzala/swrm/internal/torrent"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

const (
	maxAnnounceBodySize = 2 << 20
	maxPeersPerAnnounce = 1000
)

type Event int

const (
	EventNone Event = iota
	EventStarted
	EventCompleted
	EventStopped
)

func (e Event) httpValue() string {
	switch e {
	case EventStarted:
		return "started"
	case EventCompleted:
		return "completed"
	case EventStopped:
		return "stopped"
	default:
		return ""
	}
}

func (e Event) udpValue() uint32 {
	switch e {
	case EventCompleted:
		return 1
	case EventStarted:
		return 2
	case EventStopped:
		return 3
	default:
		return 0
	}
}

func Announce(ctx context.Context, logger *slog.Logger, tor *torrent.Torrent, peerID [20]byte, port uint16, event Event) (*Response, error) {
	urls := tor.Trackers()
	if len(urls) == 0 {
		return nil, fmt.Errorf("torrent has no tracker urls")
	}

	var errs []error

	for _, raw := range urls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		u, err := url.Parse(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid url: %w", raw, err))
			continue
		}

		logger.Debug("announcing to tracker", "tracker", raw)

		var resp *Response
		switch u.Scheme {
		case "http", "https":
			resp, err = announceHTTP(ctx, u, tor, peerID, port, event)
		case "udp":
			resp, err = announceUDP(ctx, logger, u, tor, peerID, port, event)
		default:
			errs = append(errs, fmt.Errorf("%s: unsupported tracker scheme %q", raw, u.Scheme))
			continue
		}

		if err != nil {
			logger.Debug("tracker announce failed", "tracker", raw, "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", raw, err))
			continue
		}

		if len(resp.Peers) > maxPeersPerAnnounce {
			logger.Warn("tracker returned more peers than allowed, truncating",
				"tracker", raw, "returned", len(resp.Peers), "max", maxPeersPerAnnounce)
			resp.Peers = resp.Peers[:maxPeersPerAnnounce]
		}

		logger.Info("announce succeeded", "tracker", raw, "peers", len(resp.Peers), "interval", resp.Interval)

		return resp, nil
	}

	return nil, fmt.Errorf("all trackers failed: %w", errors.Join(errs...))
}

func announceHTTP(ctx context.Context, u *url.URL, tor *torrent.Torrent, peerID [20]byte, port uint16, event Event) (*Response, error) {
	q := u.Query()
	q.Set("info_hash", string(tor.InfoHash[:]))
	q.Set("peer_id", string(peerID[:]))
	q.Set("port", strconv.Itoa(int(port)))
	q.Set("uploaded", "0")
	q.Set("downloaded", "0")
	q.Set("left", strconv.FormatInt(tor.Length, 10))
	q.Set("compact", "1")
	if ev := event.httpValue(); ev != "" {
		q.Set("event", ev)
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tracker request status expected 200, got %v", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnnounceBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAnnounceBodySize {
		return nil, fmt.Errorf("tracker response exceeds %d bytes", maxAnnounceBodySize)
	}

	return parseResponse(body)
}
