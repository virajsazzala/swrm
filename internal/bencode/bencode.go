package bencode

import (
	"bytes"
	"errors"
	"strconv"
)

const maxParseDepth = 1000

func Unmarshal(b []byte) (any, error) {
	value, consumed, err := parseValueDepth(b, 0)
	if err != nil {
		return nil, err
	}

	if consumed < len(b) {
		return nil, errors.New("Invalid Bencoded data")
	}

	return value, nil
}

func parseInt(b []byte) (int64, int, error) {
	if len(b) == 0 || b[0] != 'i' {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}
	end := bytes.IndexByte(b, 'e')

	if end == -1 || end == 1 {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	digits := b[1:end]
	if err := validateBencodeInt(digits); err != nil {
		return 0, 0, err
	}

	value, err := strconv.ParseInt(string(digits), 10, 64)
	if err != nil {
		return 0, 0, errors.New("Invalid Bencoded integer")
	}

	return value, end + 1, nil
}

func validateBencodeInt(digits []byte) error {
	i := 0
	if digits[0] == '-' {
		i = 1
		if len(digits) == 1 || digits[1] == '0' {
			return errors.New("Invalid Bencoded integer")
		}
	} else if digits[0] == '0' && len(digits) != 1 {
		return errors.New("Invalid Bencoded integer")
	}

	for ; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return errors.New("Invalid Bencoded integer")
		}
	}

	return nil
}

func parseString(b []byte) (string, int, error) {
	colon := bytes.IndexByte(b, ':')
	if colon == -1 {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	lengthDigits := b[:colon]
	if err := validateBencodeStringLength(lengthDigits); err != nil {
		return "", 0, err
	}

	length, err := strconv.Atoi(string(lengthDigits))
	if err != nil || length < 0 {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	data := b[colon+1:]
	if len(data) < length {
		return "", 0, errors.New("Invalid Bencoded byte string")
	}

	return string(data[:length]), length + colon + 1, nil
}

func validateBencodeStringLength(digits []byte) error {
	if len(digits) == 0 {
		return errors.New("Invalid Bencoded byte string")
	}
	if digits[0] == '0' && len(digits) != 1 {
		return errors.New("Invalid Bencoded byte string")
	}
	for _, d := range digits {
		if d < '0' || d > '9' {
			return errors.New("Invalid Bencoded byte string")
		}
	}
	return nil
}

func parseList(b []byte, depth int) ([]any, int, error) {
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

		value, consumed, err := parseValueDepth(b[offset:], depth)
		if err != nil {
			return nil, 0, errors.New("Invalid Bencoded list")
		}

		list = append(list, value)
		offset = offset + consumed
	}

	return list, offset + 1, nil
}

func parseDict(b []byte, depth int) (map[string]any, int, error) {
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

		value, valConsumed, err := parseValueDepth(b[offset:], depth)
		if err != nil {
			return nil, 0, errors.New("Invalid Bencoded dictionary")
		}
		offset += valConsumed

		dict[key] = value
	}

	return dict, offset + 1, nil
}

func parseValueDepth(b []byte, depth int) (any, int, error) {
	if len(b) == 0 {
		return nil, 0, errors.New("Invalid Bencoded value")
	}

	switch {
	case b[0] == 'i':
		return parseInt(b)
	case b[0] >= '0' && b[0] <= '9':
		return parseString(b)
	case b[0] == 'l':
		if depth >= maxParseDepth {
			return nil, 0, errors.New("Bencoded data nested too deeply")
		}
		return parseList(b, depth+1)
	case b[0] == 'd':
		if depth >= maxParseDepth {
			return nil, 0, errors.New("Bencoded data nested too deeply")
		}
		return parseDict(b, depth+1)
	default:
		return nil, 0, errors.New("Invalid Bencoded value")
	}
}

func parseValue(b []byte) (any, int, error) {
	return parseValueDepth(b, 0)
}

func ValueSize(b []byte) (int, error) {
	_, size, err := parseValueDepth(b, 0)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func ReadString(b []byte) (string, int, error) {
	return parseString(b)
}
