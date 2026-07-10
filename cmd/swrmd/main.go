package main

import (
	"github.com/virajsazzala/swrm/internal/bencode"
)

func main() {
	// test
	bencode.Unmarshal([]byte("li42el3:abcee"))
	bencode.Unmarshal([]byte("le"))
	bencode.Unmarshal([]byte("i42e"))
	bencode.Unmarshal([]byte("3:cat"))
}