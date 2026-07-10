package main

import (
	"fmt"
	"log"

	"github.com/virajsazzala/swrm/internal/bencode"
)

func main() {
	// test
	v, err := bencode.Unmarshal([]byte("d3:foo3:bar4:listli1ei2eee"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", v)
}
