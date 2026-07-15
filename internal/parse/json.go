package parse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

// parseJSON decodes with json.Number so 8080 stays an int and 80.5 a float —
// the default float64 decoding would erase exactly the distinction the
// type-drift check exists to catch.
func parseJSON(data []byte) (model.Tree, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	// Reject trailing garbage after the first document.
	if dec.More() {
		return nil, nil, fmt.Errorf("invalid JSON: unexpected content after top-level value")
	}
	return model.Flatten(normalizeJSONNumbers(doc)), nil, nil
}

func normalizeJSONNumbers(x any) any {
	switch t := x.(type) {
	case map[string]any:
		for k, v := range t {
			t[k] = normalizeJSONNumbers(v)
		}
		return t
	case []any:
		for i, v := range t {
			t[i] = normalizeJSONNumbers(v)
		}
		return t
	case json.Number:
		if i, err := strconv.ParseInt(t.String(), 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(t.String(), 64); err == nil && !math.IsInf(f, 0) {
			return f
		}
		return t.String()
	default:
		return x
	}
}
