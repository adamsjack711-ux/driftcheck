package parse

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
	"gopkg.in/yaml.v3"
)

// parseYAML decodes a YAML document. yaml.v3 already gives typed scalars
// (int, float64, bool, string, time.Time), which is exactly what the
// normalizer wants. Multi-document files are rejected with a clear error
// rather than silently comparing only the first document.
func parseYAML(data []byte) (model.Tree, []string, error) {
	var docs []any
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("invalid YAML: %w", err)
		}
		docs = append(docs, doc)
	}
	switch len(docs) {
	case 0:
		return model.Tree{}, nil, nil
	case 1:
		if docs[0] == nil {
			return model.Tree{}, nil, nil
		}
		return model.Flatten(docs[0]), nil, nil
	default:
		return nil, nil, fmt.Errorf("multi-document YAML (%d documents) is not supported; split the documents into separate files", len(docs))
	}
}
