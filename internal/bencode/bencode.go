package bencode

import (
	"bytes"
	"errors"
	"strconv"
)

func Unmarshal(b []byte) (any, error) {
	value, consumed, err := parseValue(b)
	if err != nil {
		return nil, err
	}

	if consumed < len(b) {
		return nil, errors.New("Invalid Bencoded data")
	}

	return value, nil
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
	end := bytes.IndexByte(b, 'e')

	if end == -1 || end == 1 {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	value, err := strconv.ParseInt(string(b[1:end]), 10, 64)
	if err != nil {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	return value, end + 1, nil
}

func parseString(b []byte) (string, int, error) {
	/*
		note:
			leading zeros spec compliance - not done yet
	*/
	colon := bytes.IndexByte(b, ':')
	if colon == -1 {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	length, err := strconv.Atoi(string(b[:colon]))
	if err != nil || length < 0 {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	data := b[colon+1:]
	if len(data) < length {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	return string(data[:length]), length + colon + 1, nil
}

func parseList(b []byte) ([]any, int, error) {
	if len(b) == 0 || b[0] != 'l' {
		return nil, 0, errors.New("Invalid Bencoded list")
	}

	list := []any{}
	offset := 1
	for {
		if offset >= len(b) {
			return nil, 0, errors.New("Unterminated Bencoded list")
		}

		if b[offset] == 'e' {
			break
		}

		value, consumed, err := parseValue(b[offset:])
		if err != nil {
			return nil, 0, errors.New("Invalid Bencoded list")
		}

		list = append(list, value)
		offset = offset + consumed
	}

	return list, offset + 1, nil
}

func parseDict(b []byte) (map[string]any, int, error) {
	if len(b) == 0 || b[0] != 'd' {
		return nil, 0, errors.New("Invalid Bencoded dictionary")
	}

	dict := map[string]any{}
	offset := 1
	for {
		if offset >= len(b) {
			return nil, 0, errors.New("Unterminated Bencoded dictionary")
		}

		if b[offset] == 'e' {
			break
		}

		key, consumed, err := parseString(b[offset:])
		if err != nil {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}
		offset += consumed

		if offset >= len(b) || b[offset] == 'e' {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}

		value, valConsumed, err := parseValue(b[offset:])
		if err != nil {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}
		offset += valConsumed

		dict[key] = value
	}

	return dict, offset + 1, nil
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

func ValueSize(b []byte) (int, error) {
	_, size, err := parseValue(b)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func ReadString(b []byte) (string, int, error) {
	return parseString(b)
}
