package main

import (
	"fmt"
	"log"

	"github.com/virajsazzala/swrm/internal/downloader"
	"github.com/virajsazzala/swrm/internal/torrent"
)

func main() {
	// test - obviously, not the actual thing

	tor, err := torrent.Open("./assets/torrent-files/sample.torrent")
	if err != nil {
		log.Fatal(err)
	}

	dl, err := downloader.New(tor)
	if err != nil {
		log.Fatal(err)
	}

	if err := dl.Announce(); err != nil {
		log.Fatal(err)
	}

	if err := dl.Download(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Download completed successfully!")
}
