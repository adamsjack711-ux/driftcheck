package parse

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

// parseTOML decodes TOML into a generic map. BurntSushi/toml produces
// int64/float64/bool/string/time.Time scalars, so normalization is direct.
func parseTOML(data []byte) (model.Tree, []string, error) {
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("invalid TOML: %w", err)
	}
	return model.Flatten(doc), nil, nil
}
