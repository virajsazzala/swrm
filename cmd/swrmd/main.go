package main

import (
	// "fmt"
	"fmt"
	"log"
	"time"

	"github.com/virajsazzala/swrm/internal/peer"
	"github.com/virajsazzala/swrm/internal/torrent"
	"github.com/virajsazzala/swrm/internal/tracker"
)

func main() {
	// test
	t, err := torrent.Open("./assets/torrent-files/debian-13.5.0-amd64-netinst.iso.torrent")
	if err != nil {
		log.Fatal(err)
	}

	i, err := peer.New()
	if err != nil {
		log.Fatal(err)
	}

	r, err := tracker.Announce(t, i, 6881)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", r)

	for _, p := range r.Peers {
		c, err := peer.Connect(p, 3*time.Second)
		if err != nil {
			continue
		}

		fmt.Printf("Successfully connected to: %s\n", c.Conn.RemoteAddr().String())
		defer c.Conn.Close()
		break
	}
}
