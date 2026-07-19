package torrent

import (
	"fmt"
)

func getString(root map[string]any, key string, required bool) (string, error) {
	v, ok := root[key]
	if !ok {
		if required {
			return "", fmt.Errorf("missing required field: %s", key)
		}
		return "", nil
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %s must be a string", key)
	}

	return s, nil
}

func getInt(root map[string]any, key string, required bool) (int64, error) {
	v, ok := root[key]
	if !ok {
		if required {
			return 0, fmt.Errorf("missing required field: %s", key)
		}
		return 0, nil
	}

	i, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("field %s must be an integer", key)
	}

	return i, nil
}

func getDict(root map[string]any, key string, required bool) (map[string]any, error) {
	v, ok := root[key]
	if !ok {
		if required {
			return nil, fmt.Errorf("missing required field: %s", key)
		}
		return nil, nil
	}

	d, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("field %s must be a dictionary", key)
	}

	return d, nil
}
