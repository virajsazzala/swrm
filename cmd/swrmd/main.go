package main

import (
	"fmt"
	"log"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

func main() {
	// test - obviously, not the actual thing

	/* fetch torrent file */
	t, err := torrent.Open("./assets/torrent-files/debian-13.6.0-amd64-netinst.iso.torrent")
	if err != nil {
		log.Fatal(err)
	}

	/* this creates a peer */
	i, err := peer.New()
	if err != nil {
		log.Fatal(err)
	}

	/* announce to the tracker */
	r, err := tracker.Announce(t, i, 6881)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", r)

	/* reach out to peers */
	for _, p := range r.Peers {
		c, err := peer.Connect(p, 3*time.Second)
		if err != nil {
			continue
		}

		addr := c.Conn.RemoteAddr().String()
		fmt.Printf("Successfully connected to: %s\n", addr)

		defer c.Conn.Close()

		/* p2p handshake */
		err = c.Handshake(t.InfoHash, i)
		if err != nil {
			fmt.Printf("Skipping bad peer %s: %v\n", addr, err)
			continue
		}
		fmt.Printf("Successfully connected to: %s (%s)\n", string(c.PeerID[:]), addr)

		err = c.Interested()
		if err != nil {
			fmt.Printf("Failed to write interested message, skipping: %v\n", err)
			continue
		}

		err = c.WaitForUnchoke()
		if err != nil {
			fmt.Printf("failed while waiting for unchoke, skipping: %v\n", err)
			continue
		}

		fmt.Printf("---\nClient state: %+v\n---\n", c)

		/* count pieces */
		cp := 0
		tp := len(t.Pieces)
		for i := 0; i < tp; i++ {
			if c.Bitfield.HasPiece(i) {
				cp++
			}
		}
		fmt.Printf("Client State\nPeer ID: %v\nPieces Available: %v/%v\nChoked: %v\nInterest: %v\n", string(c.PeerID[:]), cp, tp, c.Choked, c.Interest)

		break
	}
}
