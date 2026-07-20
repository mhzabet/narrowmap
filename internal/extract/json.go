package extract

import (
	"bytes"
	"encoding/json"
)

func JSON(data []byte, baseURL string, add func(string)) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	walkJSON(value, baseURL, add)
	return nil
}

func walkJSON(value any, baseURL string, add func(string)) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			add(key)
			walkJSON(child, baseURL, add)
		}
	case []any:
		for _, child := range typed {
			walkJSON(child, baseURL, add)
		}
	case string:
		URLsInText(typed, baseURL, add)
	case nil, bool, json.Number:
		return
	}
}
