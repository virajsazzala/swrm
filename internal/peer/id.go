package peer

import (
	"crypto/rand"
)

func New() ([20]byte, error) {
	// set id
	var id [20]byte
	copy(id[:], "-SW0001-")

	// generate 12 random bytes
	if _, err := rand.Read(id[8:]); err != nil {
		return [20]byte{}, err
	}

	return id, nil
}
