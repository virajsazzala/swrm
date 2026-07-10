package bencode

import (
	"bytes"
	"errors"
	"strconv"
)

func Unmarshal(b []byte) (any, error) {
	v, n, err := parseValue(b)
	if err != nil {
		return nil, err
	}

	if n < len(b) {
		return nil, errors.New("Invalid Bencoded data")
	}

	return v, nil
}

func parseInt(b []byte) (int64, int, error) {
	/*
		note:
			spec compliance
			current impl also allows -42, -0, +42, 03, etc.
			they are not allowed according to the spec.
			must be fixed, to throw an error.
	*/

	if len(b) == 0 || b[0] != 'i' {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}
	e := bytes.IndexByte(b, 'e')

	if e == -1 || e == 1 {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	n, err := strconv.ParseInt(string(b[1:e]), 10, 64)
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

func parseList(b []byte) ([]any, int, error) {
	if len(b) == 0 || b[0] != 'l' {
		return nil, 0, errors.New("Invalid Bencoded list")
	}

	list := []any{}
	i := 1
	for {
		if i >= len(b) {
			return nil, 0, errors.New("Unterminated Bencoded list")
		}

		if b[i] == 'e' {
			break
		}

		v, n, err := parseValue(b[i:])
		if err != nil {
			return nil, 0, errors.New("Invalid Bencoded list")
		}

		list = append(list, v)
		i = i + n
	}

	return list, i + 1, nil
}

func parseDict(b []byte) (map[string]any, int, error) {
	if len(b) == 0 || b[0] != 'd' {
		return nil, 0, errors.New("Invalid Bencoded dictionary")
	}

	dict := map[string]any{}
	i := 1
	for {
		if i >= len(b) {
			return nil, 0, errors.New("Unterminated Bencoded dictionary")
		}

		if b[i] == 'e' {
			break
		}

		vs, ns, errs := parseString(b[i:])
		if errs != nil {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}
		i = i + ns

		if i >= len(b) || b[i] == 'e' {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}

		ve, ne, erre := parseValue(b[i:])
		if erre != nil {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}
		i = i + ne

		dict[vs] = ve
	}

	return dict, i + 1, nil
}

func parseValue(b []byte) (any, int, error) {
	if len(b) == 0 {
		return nil, 0, errors.New("Invalid Bencoded value")
	}

	switch {
	case b[0] == 'i':
		return parseInt(b)
	case b[0] >= '0' && b[0] <= '9':
		return parseString(b)
	case b[0] == 'l':
		return parseList(b)
	case b[0] == 'd':
		return parseDict(b)
	default:
		return nil, 0, errors.New("Invalid Bencoded value")
	}
}
