package bencode

import (
	"errors"
	"strconv"
	"fmt"
	"bytes"
)

func Unmarshal(b []byte) {
	/* 
	    d4:name5:Alice3:agei25e4:tagsl3:cat3:doge5:metad6:admini1eee

		int -> i<int>e
		byte str -> <len>:<content>
		list -> l<elements>e
		dict -> d<keyvaluekeyvalue>e
		d
			4:name 5:Alice
			3:age  i25e
			4:tags l
						3:cat
						3:dog
			       e
			5:meta d
						6:admin i1e
			       e
		e
		{
			"name" : "Alice",
			"age"  : 25,
			"tags" : ["cat", "dog"],
			"meta" : {"admin" : 1}
		}
	*/ 	
	
	// test
	n, err := parseString(b)
	fmt.Println(n, err)
}

func parseInt(b []byte) (int, error) {
	/*
	note:
		current impl also allows -42, -0, +42, 03, etc.
		they are not allowed according to the spec.
		must be fixed, to throw an error.
	*/

	l := len(b)

	if l < 3 || b[0] != 'i' || b[l-1] != 'e' {
		return 0, errors.New("Invalid Bencoded integer")
	}

	return strconv.Atoi(string(b[1 : (l-1)]))
}

func parseString(b []byte) (string, error) {
	i := bytes.IndexByte(b, ':')
	if i == -1 {
		return "", errors.New("Invalid Bencoded byte string")
	}

	l, err := strconv.Atoi(string(b[:i]))
	if err != nil {
		return "", errors.New("Invalid Bencoded byte string")
	}

	s := b[i+1:]
	if len(s) != l {
		return "", errors.New("Invalid Bencoded byte string")
	}

	return string(s), nil
}
