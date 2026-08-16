package build

import (
	"testing"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	index := sampleIndex()

	tests := []struct {
		name        string
		spec        Spec
		index       apiimages.Index
		wantVersion string
		wantImage   string
		wantApps    []string
		wantErr     error
		errContains string
	}{
		{
			name: "highest version in channel",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "stable",
			},
			index:       index,
			wantVersion: "202608102114",
			wantImage:   "aarch64/IncusOS_202608102114.img.gz",
		},
		{
			name: "default channel when empty",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
			},
			index:       index,
			wantVersion: "202608102114",
			wantImage:   "aarch64/IncusOS_202608102114.img.gz",
		},
		{
			name: "exact pin",
			spec: Spec{
				Type:         ImageTypeISO,
				Architecture: ArchX8664,
				Channel:      "stable",
				Release:      "202608072311",
			},
			index:       index,
			wantVersion: "202608072311",
			wantImage:   "x86_64/IncusOS_202608072311.iso.gz",
		},
		{
			name: "channel filtering ignores other channel",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "testing",
			},
			index: apiimages.Index{Updates: []apiimages.UpdateFull{
				{Update: apiimages.Update{
					Version:  "202608102114",
					Channels: []string{"stable"},
					Files: []apiimages.UpdateFile{
						file("aarch64/IncusOS_202608102114.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
					},
				}},
				{Update: apiimages.Update{
					Version:  "202608021451",
					Channels: []string{"testing"},
					Files: []apiimages.UpdateFile{
						file("aarch64/IncusOS_202608021451.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
					},
				}},
			}},
			wantVersion: "202608021451",
			wantImage:   "aarch64/IncusOS_202608021451.img.gz",
		},
		{
			name: "unknown pin lists available",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "stable",
				Release:      "199901010000",
			},
			index:       index,
			wantErr:     ErrVersionNotFound,
			errContains: "available: 202608072311, 202608102114",
		},
		{
			name: "empty channel lists none",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "nightly",
			},
			index:       index,
			wantErr:     ErrVersionNotFound,
			errContains: `no updates in channel "nightly"`,
		},
		{
			name: "exactly-one-image violation",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "stable",
			},
			index: apiimages.Index{Updates: []apiimages.UpdateFull{
				{
					Update: apiimages.Update{
						Version:  "202608102114",
						Channels: []string{"stable"},
						Files: []apiimages.UpdateFile{
							file("aarch64/a.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
							file("aarch64/b.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
						},
					},
				},
			}},
			wantErr:     ErrVersionNotFound,
			errContains: "expected exactly one image-raw",
		},
		{
			name: "application matching keeps arch prefix",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "stable",
				Seeds: Seeds{Applications: &apiseed.Applications{
					Applications: []apiseed.Application{{Name: "incus"}},
				}},
			},
			index:       index,
			wantVersion: "202608102114",
			wantImage:   "aarch64/IncusOS_202608102114.img.gz",
			wantApps:    []string{"aarch64/incus.raw.gz"},
		},
		{
			name: "missing application lists what update does carry",
			spec: Spec{
				Type:         ImageTypeRaw,
				Architecture: ArchAarch64,
				Channel:      "stable",
				Seeds: Seeds{Applications: &apiseed.Applications{
					Applications: []apiseed.Application{{Name: "no-such-app"}},
				}},
			},
			index:       index,
			wantErr:     ErrVersionNotFound,
			errContains: "update does carry: aarch64/incus.raw.gz, aarch64/debug.raw.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Resolve(tt.spec, tt.index)
			if tt.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.wantErr)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, plan.Version)
			assert.Equal(t, tt.wantImage, plan.Image.Filename)

			var gotApps []string
			for _, app := range plan.Apps {
				gotApps = append(gotApps, app.Filename)
			}
			assert.Equal(t, tt.wantApps, gotApps)
		})
	}
}

func TestResolveUnknownImageType(t *testing.T) {
	t.Parallel()

	_, err := Resolve(Spec{Type: "qcow2", Architecture: ArchAarch64, Channel: "stable"}, sampleIndex())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrVersionNotFound)
	assert.Contains(t, err.Error(), "unknown image type")
}

func TestResolveHighestIsStringCompare(t *testing.T) {
	t.Parallel()

	index := apiimages.Index{Updates: []apiimages.UpdateFull{
		{Update: apiimages.Update{
			Version:  "202608021451",
			Channels: []string{"stable"},
			Files:    []apiimages.UpdateFile{file("aarch64/old.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw)},
		}},
		{Update: apiimages.Update{
			Version:  "202608102114",
			Channels: []string{"stable"},
			Files:    []apiimages.UpdateFile{file("aarch64/new.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw)},
		}},
	}}

	plan, err := Resolve(Spec{Type: ImageTypeRaw, Architecture: ArchAarch64, Channel: "stable"}, index)
	require.NoError(t, err)
	assert.Equal(t, "202608102114", plan.Version)
}

func sampleIndex() apiimages.Index {
	return apiimages.Index{Updates: []apiimages.UpdateFull{
		{Update: apiimages.Update{
			Version:  "202608102114",
			Channels: []string{"testing", "stable"},
			Files: []apiimages.UpdateFile{
				file("aarch64/IncusOS_202608102114.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
				file("aarch64/IncusOS_202608102114.iso.gz", ArchAarch64, apiimages.UpdateFileTypeImageISO),
				file("x86_64/IncusOS_202608102114.img.gz", ArchX8664, apiimages.UpdateFileTypeImageRaw),
				file("x86_64/IncusOS_202608102114.iso.gz", ArchX8664, apiimages.UpdateFileTypeImageISO),
				file("aarch64/incus.raw.gz", ArchAarch64, apiimages.UpdateFileTypeApplication),
				file("aarch64/debug.raw.gz", ArchAarch64, apiimages.UpdateFileTypeApplication),
				file("x86_64/incus.raw.gz", ArchX8664, apiimages.UpdateFileTypeApplication),
			},
		}},
		{Update: apiimages.Update{
			Version:  "202608072311",
			Channels: []string{"stable"},
			Files: []apiimages.UpdateFile{
				file("aarch64/IncusOS_202608072311.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
				file("x86_64/IncusOS_202608072311.iso.gz", ArchX8664, apiimages.UpdateFileTypeImageISO),
				file("aarch64/incus.raw.gz", ArchAarch64, apiimages.UpdateFileTypeApplication),
			},
		}},
		{Update: apiimages.Update{
			Version:  "202608021451",
			Channels: []string{"testing"},
			Files: []apiimages.UpdateFile{
				file("aarch64/IncusOS_202608021451.img.gz", ArchAarch64, apiimages.UpdateFileTypeImageRaw),
			},
		}},
	}}
}

func file(name string, arch Architecture, typ apiimages.UpdateFileType) apiimages.UpdateFile {
	return apiimages.UpdateFile{
		Architecture: apiimages.UpdateFileArchitecture(arch),
		Filename:     name,
		Type:         typ,
		Size:         1,
		Sha256:       "00",
	}
}
