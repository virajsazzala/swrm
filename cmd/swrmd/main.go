package main

import (
	"fmt"
	"github.com/virajsazzala/swrm/internal/bencode"
)

func main() {
	// test
	inp := "i5432e"
	bencode.Unmarshal([]byte(inp))
	fmt.Println([]byte("23"))
}