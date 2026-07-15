// Package parse turns config files of any supported format into a normalized
// model.Tree. It is the seam where non-file sources (Vault, AWS Secrets
// Manager, ...) would plug in later: implement Source and register it.
package parse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adamsjack711-ux/driftcheck/internal/model"
)

// Format identifies a supported config syntax.
type Format string

const (
	FormatEnv     Format = "env"
	FormatJSON    Format = "json"
	FormatYAML    Format = "yaml"
	FormatTOML    Format = "toml"
	FormatUnknown Format = ""
)

// Source loads a named config into a normalized tree. FileSource is the only
// implementation in v1; a secret-manager provider would be another.
type Source interface {
	// Load returns the normalized tree plus non-fatal warnings (e.g. skipped
	// malformed lines). A returned error means the source is unusable.
	Load(name string) (model.Tree, []string, error)
}

// DetectFormat infers the format from a file name.
// Returns FormatUnknown for unsupported extensions.
func DetectFormat(name string) Format {
	base := filepath.Base(name)
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env") {
		return FormatEnv
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".json":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".toml":
		return FormatTOML
	case ".env":
		return FormatEnv
	default:
		return FormatUnknown
	}
}

// FileSource loads config files from disk, dispatching on extension.
type FileSource struct{}

func (FileSource) Load(path string) (model.Tree, []string, error) {
	format := DetectFormat(path)
	if format == FormatUnknown {
		return nil, nil, fmt.Errorf("%s: unsupported config format (want .env, .json, .yaml/.yml, or .toml)", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	tree, warnings, err := Parse(format, data)
	if err != nil {
		return nil, warnings, fmt.Errorf("%s: %w", path, err)
	}
	return tree, warnings, nil
}

// Parse decodes raw bytes of a known format into a normalized tree.
func Parse(format Format, data []byte) (model.Tree, []string, error) {
	switch format {
	case FormatEnv:
		return parseEnv(data)
	case FormatJSON:
		return parseJSON(data)
	case FormatYAML:
		return parseYAML(data)
	case FormatTOML:
		return parseTOML(data)
	default:
		return nil, nil, fmt.Errorf("unsupported format %q", format)
	}
}
