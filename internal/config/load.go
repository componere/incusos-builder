package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/getsops/sops/v3/decrypt"
	"go.yaml.in/yaml/v4"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	sopsFormat    = "yaml"
	sopsKey       = "sops"
	unknownMid    = " not found in type "
	unknownPrefix = "field "
)

// Load reads path and returns a validated [build.Spec]. Path "-" reads stdin,
// including SOPS-encrypted bytes.
func Load(path string) (build.Spec, error) {
	return load(path, os.Stdin)
}

// Parse decodes raw config bytes into a validated [build.Spec].
// A top-level sops key selects in-memory decryption via decrypt.Data;
// every subsequent failure wraps [ErrDecrypt].
func Parse(raw []byte) (build.Spec, error) {
	encrypted, err := hasTopLevelSOPS(raw)
	if err != nil {
		return build.Spec{}, fmt.Errorf("%w: %s", ErrConfig, sanitizeYAMLMessage(err.Error()))
	}
	plaintext := raw
	if encrypted {
		decrypted, decErr := decrypt.Data(raw, sopsFormat)
		if decErr != nil {
			return build.Spec{}, fmt.Errorf("%w: %w", ErrDecrypt, decErr)
		}
		plaintext = decrypted
	}
	var doc document
	if err := yaml.Load(plaintext, &doc, yaml.WithKnownFields()); err != nil {
		return build.Spec{}, wrapDecodeError(err)
	}
	if err := checkVersion(&doc); err != nil {
		return build.Spec{}, err
	}
	applyDefaults(&doc)
	if err := validate(&doc); err != nil {
		return build.Spec{}, err
	}
	return doc.spec(), nil
}

// load is [Load] with an injectable stdin reader.
func load(path string, stdin io.Reader) (build.Spec, error) {
	raw, err := readConfig(path, stdin)
	if err != nil {
		return build.Spec{}, err
	}
	return Parse(raw)
}

// readConfig reads path or stdin ("-").
func readConfig(path string, stdin io.Reader) ([]byte, error) {
	if path == stdinPath {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("%w: read stdin: %w", ErrConfig, err)
		}
		return raw, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // path is the caller-supplied config file
	if err != nil {
		return nil, fmt.Errorf("%w: read config: %w", ErrConfig, err)
	}
	return raw, nil
}

// hasTopLevelSOPS reports whether raw YAML contains a top-level sops key.
// Presence alone selects the encrypted path; the block is not inspected.
func hasTopLevelSOPS(raw []byte) (bool, error) {
	var top map[string]any
	if err := yaml.Load(raw, &top); err != nil {
		return false, err
	}
	_, ok := top[sopsKey]
	return ok, nil
}

// wrapDecodeError maps yaml strict-decode failures to [ErrConfig] with field paths.
func wrapDecodeError(err error) error {
	var loadErrs *yaml.LoadErrors
	if errors.As(err, &loadErrs) {
		parts := make([]string, 0, len(loadErrs.Errors))
		for _, le := range loadErrs.Errors {
			parts = append(parts, formatLoadError(le))
		}
		return fmt.Errorf("%w: %s", ErrConfig, strings.Join(parts, "; "))
	}
	var loadErr *yaml.LoadError
	if errors.As(err, &loadErr) {
		return fmt.Errorf("%w: %s", ErrConfig, formatLoadError(loadErr))
	}
	return fmt.Errorf("%w: %s", ErrConfig, sanitizeYAMLMessage(err.Error()))
}

// formatLoadError turns a yaml constructor error into a field-path message.
func formatLoadError(le *yaml.LoadError) string {
	field, typeName, ok := parseUnknownField(le.Message)
	if ok {
		return unknownFieldMessage(fieldPath(typeName, field))
	}
	return sanitizeYAMLMessage(le.Message)
}

// parseUnknownField extracts field and Go type from a KnownFields error message.
func parseUnknownField(msg string) (field, typeName string, ok bool) {
	rest, found := strings.CutPrefix(msg, unknownPrefix)
	if !found {
		return "", "", false
	}
	field, typeName, found = strings.Cut(rest, unknownMid)
	if !found || field == "" || typeName == "" {
		return "", "", false
	}
	return field, typeName, true
}

// fieldPath maps a yaml type name plus leaf field to a dotted YAML path.
func fieldPath(typeName, field string) string {
	prefix := yamlPathPrefix(typeName)
	if prefix == "" {
		return field
	}
	return prefix + "." + field
}

// yamlPathPrefix returns the YAML path of a decoded Go type.
func yamlPathPrefix(typeName string) string {
	switch {
	case strings.HasSuffix(typeName, ".document"):
		return ""
	case strings.HasSuffix(typeName, ".image"):
		return "image"
	case strings.HasSuffix(typeName, ".seeds"):
		return "seeds"
	case strings.HasSuffix(typeName, ".Applications"):
		return "seeds.applications"
	case strings.HasSuffix(typeName, ".Application"):
		return "seeds.applications"
	case strings.HasSuffix(typeName, ".InstallTarget"):
		return "seeds.install.target"
	case strings.HasSuffix(typeName, ".InstallSecurity"):
		return "seeds.install.security"
	case strings.HasSuffix(typeName, ".Install"):
		return "seeds.install"
	case strings.HasSuffix(typeName, ".MigrationManager"):
		return "seeds.migration-manager"
	case strings.HasSuffix(typeName, ".OperationsCenter"):
		return "seeds.operations-center"
	case strings.HasSuffix(typeName, ".Incus"):
		return "seeds.incus"
	case strings.HasSuffix(typeName, ".Network"):
		return "seeds.network"
	case strings.HasSuffix(typeName, ".Provider"):
		return "seeds.provider"
	case strings.HasSuffix(typeName, ".Services"):
		return "seeds.services"
	case strings.HasSuffix(typeName, ".Update"):
		return "seeds.update"
	case strings.HasSuffix(typeName, ".Kernel"):
		return "seeds.kernel"
	case strings.HasSuffix(typeName, ".Security"):
		return "seeds.security"
	default:
		return ""
	}
}

// unknownFieldMessage is the strict-decode wording required by ARCHITECTURE §4.
func unknownFieldMessage(path string) string {
	return path + ": " + unknownFieldHint
}

// sanitizeYAMLMessage strips quoted literals so secret values never appear in errors.
func sanitizeYAMLMessage(msg string) string {
	var b strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(msg); i++ {
		c := msg[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
				b.WriteString("<value>")
			}
			continue
		}
		if c == '"' || c == '`' {
			inQuote = c
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
