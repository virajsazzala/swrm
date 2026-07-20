package tracker

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/virajsazzala/swrm/internal/torrent"
)

const udpProtocolMagic = 0x41727101980

const (
	udpActionConnect  = 0
	udpActionAnnounce = 1
	udpActionError    = 3
)

const (
	udpDialTimeout = 5 * time.Second
	udpBaseTimeout = 15 * time.Second
	udpMaxRetries  = 3
)

func announceUDP(ctx context.Context, logger *slog.Logger, u *url.URL, tor *torrent.Torrent, peerID [20]byte, port uint16, event Event) (*Response, error) {
	var dialer net.Dialer
	dialCtx, cancel := context.WithTimeout(ctx, udpDialTimeout)
	defer cancel()

	conn, err := dialer.DialContext(dialCtx, "udp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("udp dial %s: %w", u.Host, err)
	}
	defer conn.Close()

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		return nil, fmt.Errorf("expected a UDP connection")
	}

	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	connID, err := udpConnect(ctx, logger, udpConn)
	if err != nil {
		return nil, fmt.Errorf("udp connect: %w", err)
	}

	resp, err := udpAnnounce(ctx, logger, udpConn, connID, tor, peerID, port, event)
	if err != nil {
		return nil, fmt.Errorf("udp announce: %w", err)
	}

	return resp, nil
}

func randUint32() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("failed to generate random value: %w", err)
	}
	return binary.BigEndian.Uint32(b[:]), nil
}

func udpConnect(ctx context.Context, logger *slog.Logger, conn *net.UDPConn) (uint64, error) {
	transactionID, err := randUint32()
	if err != nil {
		return 0, err
	}

	req := make([]byte, 16)
	binary.BigEndian.PutUint64(req[0:8], udpProtocolMagic)
	binary.BigEndian.PutUint32(req[8:12], udpActionConnect)
	binary.BigEndian.PutUint32(req[12:16], transactionID)

	resp, err := udpRoundTrip(ctx, logger, conn, req, 16)
	if err != nil {
		return 0, err
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	gotTxID := binary.BigEndian.Uint32(resp[4:8])

	if gotTxID != transactionID {
		return 0, fmt.Errorf("transaction id mismatch")
	}
	if action == udpActionError {
		return 0, fmt.Errorf("tracker error: %s", string(resp[8:]))
	}
	if action != udpActionConnect {
		return 0, fmt.Errorf("unexpected action %d", action)
	}

	return binary.BigEndian.Uint64(resp[8:16]), nil
}

func udpAnnounce(ctx context.Context, logger *slog.Logger, conn *net.UDPConn, connID uint64, tor *torrent.Torrent, peerID [20]byte, port uint16, event Event) (*Response, error) {
	transactionID, err := randUint32()
	if err != nil {
		return nil, err
	}
	key, err := randUint32()
	if err != nil {
		return nil, err
	}

	req := make([]byte, 98)
	binary.BigEndian.PutUint64(req[0:8], connID)
	binary.BigEndian.PutUint32(req[8:12], udpActionAnnounce)
	binary.BigEndian.PutUint32(req[12:16], transactionID)
	copy(req[16:36], tor.InfoHash[:])
	copy(req[36:56], peerID[:])
	binary.BigEndian.PutUint64(req[56:64], 0)                  // downloaded
	binary.BigEndian.PutUint64(req[64:72], uint64(tor.Length)) // left
	binary.BigEndian.PutUint64(req[72:80], 0)                  // uploaded
	binary.BigEndian.PutUint32(req[80:84], event.udpValue())
	binary.BigEndian.PutUint32(req[84:88], 0) // IP - let the tracker use the packet's source IP
	binary.BigEndian.PutUint32(req[88:92], key)
	binary.BigEndian.PutUint32(req[92:96], 0xFFFFFFFF) // num_want: default
	binary.BigEndian.PutUint16(req[96:98], port)

	resp, err := udpRoundTrip(ctx, logger, conn, req, 20)
	if err != nil {
		return nil, err
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	gotTxID := binary.BigEndian.Uint32(resp[4:8])

	if gotTxID != transactionID {
		return nil, fmt.Errorf("transaction id mismatch")
	}
	if action == udpActionError {
		return nil, fmt.Errorf("tracker error: %s", string(resp[8:]))
	}
	if action != udpActionAnnounce {
		return nil, fmt.Errorf("unexpected action %d", action)
	}

	interval := binary.BigEndian.Uint32(resp[8:12])
	// leechers := binary.BigEndian.Uint32(resp[12:16])
	// seeders  := binary.BigEndian.Uint32(resp[16:20])

	peers, err := parseCompactPeers(string(resp[20:]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse peers: %w", err)
	}

	return &Response{Interval: int64(interval), Peers: peers}, nil
}

func udpRoundTrip(ctx context.Context, logger *slog.Logger, conn *net.UDPConn, req []byte, minRespLen int) ([]byte, error) {
	buf := make([]byte, 2048)

	var lastErr error
	for attempt := 0; attempt <= udpMaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if _, err := conn.Write(req); err != nil {
			return nil, fmt.Errorf("write: %w", err)
		}

		timeout := udpBaseTimeout * time.Duration(1<<attempt)
		conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := conn.Read(buf)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			lastErr = err
			logger.Debug("udp round trip attempt failed", "attempt", attempt, "timeout", timeout, "err", err)
			continue
		}

		if n < minRespLen {
			lastErr = fmt.Errorf("response too short: %d bytes", n)
			logger.Debug("udp round trip attempt failed", "attempt", attempt, "timeout", timeout, "err", lastErr)
			continue
		}

		result := make([]byte, n)
		copy(result, buf[:n])
		return result, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", udpMaxRetries, lastErr)
}
