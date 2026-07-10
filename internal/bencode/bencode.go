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
	s, n, err := parseString(b)
	fmt.Println(s, n, err)

	nu, bn, errn := parseInt(b)
	fmt.Println(nu, bn, errn)
}

func parseInt(b []byte) (int, int, error) {
	/*
	note:
		spec compliance
		current impl also allows -42, -0, +42, 03, etc.
		they are not allowed according to the spec.
		must be fixed, to throw an error.
	*/

	if (len(b) == 0 || b[0] != 'i') {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}
	e := bytes.IndexByte(b, 'e')

	if e == -1 || e == 1 {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	n, err := strconv.Atoi(string(b[1:e]))
	if err != nil {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	return n, e + 1, nil
}

func parseString(b []byte) (string, int, error) {
	/*
	note:
		leading zeros spec compliance - not done yet
	*/
	i := bytes.IndexByte(b, ':')
	if i == -1 {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	l, err := strconv.Atoi(string(b[:i]))
	if err != nil || l < 0 {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	s := b[i+1:]
	if len(s) < l {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	return string(s[:l]), l + i + 1, nil
}
