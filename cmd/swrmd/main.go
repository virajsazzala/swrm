package main

import (
	// "fmt"
	"fmt"
	"log"

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
}
