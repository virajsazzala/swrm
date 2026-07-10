package main

import (
	"fmt"
	"log"

	"github.com/virajsazzala/swrm/internal/torrent"
)

func main() {
	// test
	t, err := torrent.Open("./assets/torrent-files/debian-13.5.0-amd64-netinst.iso.torrent")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", t)
}
