package main

import (
	"fmt"
	"log"

	"github.com/virajsazzala/swrm/internal/downloader"
	"github.com/virajsazzala/swrm/internal/torrent"
)

func main() {
	// test - obviously, not the actual thing

	/* fetch torrent file */
	// t, err := torrent.Open("./assets/torrent-files/debian-13.6.0-amd64-netinst.iso.torrent")
	t, err := torrent.Open("./assets/torrent-files/sample.torrent")
	if err != nil {
		log.Fatal(err)
	}

	d, err := downloader.New(t)
	if err != nil {
		log.Fatal(err)
	}

	err = d.Announce()
	if err != nil {
		log.Fatal(err)
	}

	err = d.Download()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Download completed successfully!")
}
