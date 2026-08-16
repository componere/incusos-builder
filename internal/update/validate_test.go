package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

func TestValidateVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "observed datetime", version: "202608102114"},
		{name: "observed IncusOS stamp", version: "IncusOS_202508141200"},
		{name: "dot in middle", version: "2026.08"},
		{name: "empty", version: "", wantErr: true},
		{name: "dot", version: ".", wantErr: true},
		{name: "dotdot", version: "..", wantErr: true},
		{name: "question", version: "v?1", wantErr: true},
		{name: "hash", version: "v#1", wantErr: true},
		{name: "percent", version: "v%61", wantErr: true},
		{name: "slash", version: "a/b", wantErr: true},
		{name: "backslash", version: `a\b`, wantErr: true},
		{name: "space", version: "a b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateVersion(tt.version)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrFetch)
				assert.Contains(t, err.Error(), tamperSuffix)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{name: "nested arch path", filename: "aarch64/IncusOS_202608102114.img.gz"},
		{name: "single segment", filename: "changelog.yaml.gz"},
		{name: "empty", filename: "", wantErr: true},
		{name: "absolute", filename: "/etc/passwd", wantErr: true},
		{name: "dotdot segment", filename: "aarch64/../passwd", wantErr: true},
		{name: "dot segment", filename: "aarch64/./x", wantErr: true},
		{name: "empty segment", filename: "aarch64//x", wantErr: true},
		{name: "question", filename: "a?b", wantErr: true},
		{name: "hash", filename: "a#b", wantErr: true},
		{name: "percent", filename: "a%2e%2e", wantErr: true},
		{name: "backslash", filename: `aarch64\x`, wantErr: true},
		{name: "trailing slash", filename: "aarch64/", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFilename(tt.filename)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrFetch)
				assert.Contains(t, err.Error(), tamperSuffix)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateSHA256AndSize(t *testing.T) {
	t.Parallel()

	good := strings.Repeat("ab", 32)
	require.NoError(t, ValidateSHA256(good))
	require.ErrorIs(t, ValidateSHA256(strings.Repeat("AB", 32)), ErrFetch)
	require.ErrorIs(t, ValidateSHA256(good[:63]), ErrFetch)
	require.ErrorIs(t, ValidateSHA256(good+"aa"), ErrFetch)
	require.ErrorIs(t, ValidateSHA256("not-hex-"+strings.Repeat("0", 56)), ErrFetch)

	require.NoError(t, validateSize(1))
	require.NoError(t, validateSize(maxAssetSize))
	require.ErrorIs(t, validateSize(0), ErrFetch)
	require.ErrorIs(t, validateSize(-1), ErrFetch)
	require.ErrorIs(t, validateSize(maxAssetSize+1), ErrFetch)
}

func TestValidateAssetOrder(t *testing.T) {
	t.Parallel()

	file := apiimages.UpdateFile{
		Filename: "ok.bin",
		Sha256:   strings.Repeat("ab", 32),
		Size:     1,
	}
	require.NoError(t, validateAsset(testVersion, file))

	bad := file
	bad.Size = 0
	err := validateAsset("v?bad", bad)
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "version")
}
