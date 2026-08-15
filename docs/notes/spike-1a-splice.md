# Spike 1.A — seed splice mechanics + local verification

Date: 2026-08-15. Revised after review (`FindingsReview`): GPT probe 512/2048/4096; untouched-region digests; yaml verdict scoped; go-diskfs dep-weight struck; `sendOSImage` citation.
Scratch: `spikes/splice/` (`module spike-splice`; `go run .` from that directory).
Worktree: `.wt/feat-phase-1-spikes` (`feat/phase-1-spikes`).
Non-goals honored: no VM boot; no root `go.mod` / `internal/` / `cmd/` / other `spikes/*` changes.

Upstream pin: `github.com/lxc/incus-os/incus-osd@v0.0.0-20260815030500-0f5b8057f2fc`.
Reference: `reference/incus-os/incus-osd/internal/seed/seed.go`, `reference/incus-os/incus-osd/cmd/image-customizer/main.go` (`writeSeed`).

Local artifacts (gitignored under `spikes/splice/out/`): `seeded.img`, `MANIFEST.txt`.

---

## Image used

Newest index update `202608102114`. Smallest arm64 `image-raw` (only one aarch64 raw):

| field | value |
|---|---|
| version | `202608102114` |
| arch | `aarch64` |
| type | `image-raw` |
| filename | `aarch64/IncusOS_202608102114.img.gz` |
| URL | `https://images.linuxcontainers.org/os/202608102114/aarch64/IncusOS_202608102114.img.gz` |
| sha256 (index + verified) | `4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75` |
| size gz | 435456672 |
| size raw | 3432026112 |

```
$ shasum -a 256 out/IncusOS_202608102114.img.gz
4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75  out/IncusOS_202608102114.img.gz
```

---

## Splice arithmetic (end-to-end)

Customizer `sendOSImage` (`incus-osd/cmd/image-customizer/main.go:399`) hardcodes the seed-data start (`remainder := int64(2148532224)` at :507), overwrites `len(tar)` bytes there (`writeSeed` at :529), then `rc.Seek(int64(seedSize), 1)` at :535 and copies the remainder (:540–:549). Sibling is `sendRescueImage` (:554).

GPT on this release:

- sector size 512
- `seed-data` first LBA 4196352 → start byte **2148532224**, length **104857600** (100 MiB)

**OFFSET MATCH: no drift vs 2,148,532,224.**

Splice: stream `[0, offset)` → tar bytes → skip `len(tar)` from the source → remainder. Single 4 MiB buffer. Output size equals input size (3432026112). Verify: bytes at `offset..offset+len(tar)` are exactly the input tar; sha256 of `[0, offset)` and `[offset+len(tar), EOF)` match source vs spliced (streamed, reused 4 MiB buffer); tar entries strict-decode with `yaml.WithKnownFields()` into upstream seed types.

```
######## splice ########
splice: in=out/IncusOS_202608102114.img out=out/seeded.img offset=2148532224 tar=3072 out_size=3432026112 in_size=3432026112 wall=1.728s
TIMING splice: 1.728s

######## verify ########
verify: read 3072 bytes at offset 2148532224 in 0s
TAR BYTES: spliced region prefix == input tar (exact)
untar + strict yaml.WithKnownFields():
  install.yaml mode=0600 size=65
    decoded {"version":"1","force_install":true,"force_reboot":true,"target":null}
    strict-decode OK into upstream type
  network.yaml mode=0600 size=13
    decoded {"version":"1"}
    strict-decode OK into upstream type
untouched regions (sha256, streamed, reused 4194304-byte buffer):
  [0, 2148532224) len=2148532224
    src=ce355d34b335ae3d26affadd701abfcd10ed4c9998f643cdc0e54f97d7e55c57
    out=ce355d34b335ae3d26affadd701abfcd10ed4c9998f643cdc0e54f97d7e55c57
    PREFIX: identical
  [2148535296, EOF) len=1283490816
    src=7276168192d692f5ff75bf4271dc34d004eb6b556abf84ed398ae52142aa0df7
    out=7276168192d692f5ff75bf4271dc34d004eb6b556abf84ed398ae52142aa0df7
    SUFFIX: identical
untouched-region digests: 2.725s
round-trip: PASS
```

Re-run of `verify` against the existing `out/` artifacts (source `IncusOS_202608102114.img` already decompressed; not re-downloaded). Both untouched regions are byte-identical source vs spliced. Equal total size alone would not have caught a compensated skip.

Spike 1.E booted this same `seeded.img` through UEFI → systemd-boot → UKI → kernel → systemd PID 1, which exercises the ESP (before the offset) and the dm-verity root partitions (after it). That is corroboration, not a substitute for the region digests.

Second splice+verify with CLI-exclusive `kernel.yaml` + `security.yaml`:

```
######## seed+splice+verify kernel+security ########
wrote out/seed-ks.tar (5120 bytes) ks=true
  tar entry name="install.yaml" mode=0600 size=65 format=USTAR typeflag="0"
  tar entry name="network.yaml" mode=0600 size=13 format=USTAR typeflag="0"
  tar entry name="kernel.yaml" mode=0600 size=67 format=USTAR typeflag="0"
  tar entry name="security.yaml" mode=0600 size=72 format=USTAR typeflag="0"
kernel/security: CLI-exclusive filenames (writeSeed does not emit these)
splice: in=out/IncusOS_202608102114.img out=out/seeded-ks.img offset=2148532224 tar=5120 out_size=3432026112 in_size=3432026112 wall=1.318s
verify: read 5120 bytes at offset 2148532224 in 0s
TAR BYTES: spliced region prefix == input tar (exact)
untar + strict yaml.WithKnownFields():
  install.yaml mode=0600 size=65
    decoded {"version":"1","force_install":true,"force_reboot":true,"target":null}
    strict-decode OK into upstream type
  network.yaml mode=0600 size=13
    decoded {"version":"1"}
    strict-decode OK into upstream type
  kernel.yaml mode=0600 size=67
    decoded {"console":[{"device":"/dev/ttyS0","baud_rate":115200}],"version":"1"}
    strict-decode OK into upstream type
  security.yaml mode=0600 size=72
    decoded {"custom_ca_certs":["spike-ca"],"encryption_recovery_keys":[],"version":"1"}
    strict-decode OK into upstream type
untouched regions (sha256, streamed, reused 4194304-byte buffer):
  [0, 2148532224) len=2148532224
    src=ce355d34b335ae3d26affadd701abfcd10ed4c9998f643cdc0e54f97d7e55c57
    out=ce355d34b335ae3d26affadd701abfcd10ed4c9998f643cdc0e54f97d7e55c57
    PREFIX: identical
  [2148537344, EOF) len=1283488768
    src=d3e44a7cf42b34537386e66d4efffa8960b5a8016d8a9979b29c2f597f30f534
    out=d3e44a7cf42b34537386e66d4efffa8960b5a8016d8a9979b29c2f597f30f534
    SUFFIX: identical
untouched-region digests: 2.612s
round-trip: PASS
```

Clean round-trip including kernel+security.

---

## DECISION: go-diskfs GPT reader vs hand parse (Phase 2)

Both parsers **agree** on every populated entry (name, first/last LBA, start byte, length). Parse cost is noise (61µs hand / 49µs diskfs) next to pgzip decompress (2.4s).

```
=== hand GPT (sector 512, parse 61µs) ===
hand: 5 populated partitions
#   name                                  first_lba     last_lba     start_byte         length
0   esp                                        2048      4196351        1048576     2147483648
1   seed-data                               4196352      4401151     2148532224      104857600
2   IncusOS_202608102114_verity_sig         4401152      4401183     2253389824          16384
3   IncusOS_202608102114_verity             4401184      4605983     2253406208      104857600
4   IncusOS_202608102114                    4605984      6703135     2358263808     1073741824

=== go-diskfs GPT (logical sector 512, parse 49µs) ===
diskfs: 5 populated partitions
#   name                                  first_lba     last_lba     start_byte         length
0   esp                                        2048      4196351        1048576     2147483648
1   seed-data                               4196352      4401151     2148532224      104857600
2   IncusOS_202608102114_verity_sig         4401152      4401183     2253389824          16384
3   IncusOS_202608102114_verity             4401184      4605983     2253406208      104857600
4   IncusOS_202608102114                    4605984      6703135     2358263808     1073741824

=== GPT parser diff ===
AGREE: both parsers report the same name/start-byte/length for every populated entry

seed-data start_byte=2148532224 length=104857600 (100 MiB)
customizer hardcoded offset: 2148532224
OFFSET MATCH: seed-data start == 2148532224 (no drift)
```

API fit: `diskfs.Open(path, diskfs.WithOpenMode(diskfs.ReadOnly))` + `GetPartitionTable()` → `*gpt.Table` with `Partitions []*gpt.Partition` (`Start`/`End` sectors, `Size` bytes, `Name`). Usable. Default open mode is `ReadWriteExclusive` (`O_RDWR|O_EXCL`) — must pass `ReadOnly` for a shared image file.

go-diskfs v1.9.4 is already a required product dependency: spike 1.B adopted it as the rescue-media library, ARCHITECTURE §6 has `internal/media` depend on it, and Phase 3b writes GPT with it. The transitive surface is already in the graph; there is no marginal-dependency argument against using it to *read* a partition table. That earlier rationale is struck.

**Scope:** only the 512-byte raw layout was measured live (this `image-raw` artifact). The CLI's other first-class type is `iso`. That was **not** downloaded or probed here.

**ISO layout (from source, not measured):** upstream `scripts/convert-img-to-iso.sh` does `losetup --sector-size 2048` then `sgdisk -n 1::+2GiB` / `-n 2::+100MiB -c 2:seed-data`. On that image the GPT header lives at byte 2048 (LBA1). The seed-data *byte* offset is unchanged: 1 MiB alignment at 2048 B/sector puts ESP at LBA 512 and seed-data at LBA 1049088 = 2,148,532,224. Only the sector arithmetic differs.

The hand parser now tries logical sector sizes **512, 2048, 4096** (locate `EFI PART` at those LBA1 offsets) and derives `secsz` from the match. Synthetic headers at each offset reach the post-signature parse (so the probe finds them); the live raw still reports sector 512.

**Phase 2: still hand-parse GPT**, on the surviving grounds:

1. **Explicit multi-sector-size probe control.** We need 512 (raw) and 2048 (ISO) before asserting `seed-data`. A 20-line loop over three offsets is the whole policy.
2. **go-diskfs read path defaults to 512 for regular files** unless `WithSectorSize` is passed (`initDisk` in `diskfs.go`: default `defaultBlocksize`; `getSectorSizes` runs only for block devices). Exported constants stop at `SectorSize512` / `SectorSize4k` — 2048 is not a named size. Using go-diskfs for this read would still mean wrapping it in the same 512/2048/4096 probe plus `WithSectorSize(SectorSize(n))`. That is not simpler than reading 92 bytes at three offsets.
3. **Parse cost is noise either way** (61 µs hand / 49 µs diskfs against 2.4 s of decompression). Cost does not pick a winner.

Those are enough to keep the hand parse for *locating* `seed-data` on a regular file. They would not be enough if we were writing GPT — 1.B already does that with go-diskfs, and this spike should not pretend otherwise. Do not hardcode `2148532224` without the GPT assert — it matches this release, but the customizer's hardcoded offset is a footgun if the layout moves.

---

## yaml/v4 `WithV2Defaults` vs writeSeed

`writeSeed` (`image-customizer/main.go`):

- `yaml.Dump(v, yaml.WithV2Defaults())`
- tar header `{Name, Mode: 0o600, Size}` only (USTAR, typeflag `'0'`)
- `tar.NewWriter` + `Close()` (end-of-archive blocks included in the returned size)
- order, nil-skipped: applications, incus, operations-center, migration-manager, install, network, provider, services, update

Spike renderer uses the same dump + header fields. The `cmp` below is the spike renderer vs the spike's own `writeSeed` copy (`spikes/splice/main.go`), not vs an upstream-built customizer binary — tautological with respect to the yaml library. Only `install` + `network` of the nine `writeSeed` sections were compared (65 and 13 bytes).

```
wrote out/seed-install-network.tar (3072 bytes) ks=false
  tar entry name="install.yaml" mode=0600 size=65 format=USTAR typeflag="0"
  tar entry name="network.yaml" mode=0600 size=13 format=USTAR typeflag="0"
writeSeed clone size=3072 (counter=3072)
cmp paths: out/cmp/writeseed.tar out/cmp/ours.tar
CMP: ours == writeSeed clone (byte-identical)
```

```
$ cmp -l out/cmp/writeseed.tar out/cmp/ours.tar && echo 'cmp: identical (exit 0)'
cmp: identical (exit 0)
```

**Verdict:** same `yaml.Dump` call + tar header fields as upstream source (`cmd/image-customizer/main.go:1000-1120`). Byte-equality vs an upstream-built binary is unproven. Phase 2 should dump with `yaml.Dump(..., yaml.WithV2Defaults())` and the same tar headers. Do not switch to v4 dump defaults (`WithV2Defaults` = indent 2 + `CompactSeqIndent(false)` + `QuoteLegacy`; `WithV4Defaults` = indent 2 + `CompactSeqIndent(true)` + `QuoteSingle` — `go.yaml.in/yaml/v4@v4.0.0-rc.6` `yaml.go:38-73`).

ARCHITECTURE §9 T1's fully-populated nine-section golden remains **open for Phase 2**. Pin it by building the upstream customizer once, generating the tar, and committing its digest.

---

## Seed entry filename table

OSD reader (`parseFileContentsFromRawTar`): walk the tar, trim optional `./` prefix, match `{stem}.json` (encoding/json `DisallowUnknownFields`) or `{stem}.yaml` / `{stem}.yml` (`yaml.NewLoader(..., yaml.WithKnownFields())`). VFAT/ISO user partition uses the same three names as directory entries.

| stem (Get* argument) | writeSeed tar name | reader also accepts | notes |
|---|---|---|---|
| `applications` | `applications.yaml` | `.json`, `.yml` | |
| `incus` | `incus.yaml` | `.json`, `.yml` | |
| `operations-center` | `operations-center.yaml` | `.json`, `.yml` | |
| `migration-manager` | `migration-manager.yaml` | `.json`, `.yml` | |
| `install` | `install.yaml` | `.json`, `.yml` | empty file (`io.EOF`) still means "do install" |
| `network` | `network.yaml` | `.json`, `.yml` | missing → default DHCP/SLAAC |
| `provider` | `provider.yaml` | `.json`, `.yml` | |
| `services` | `services.yaml` | `.json`, `.yml` | |
| `update` | `update.yaml` | `.json`, `.yml` | |
| `kernel` | **not written** | `kernel.yaml` / `.yml` / `.json` | CLI-exclusive; `GetKernel` treats missing as empty |
| `security` | **not written** | `security.yaml` / `.yml` / `.json` | CLI-exclusive; `GetSecurity` rejects non-empty `encryption_recovery_keys` |

This spike wrote `kernel.yaml` + `security.yaml` with the same dump/header rules and they round-tripped. Phase 2 CLI should emit those stems; the web customizer will not.

---

## Full partition table (202608102114 aarch64 raw)

Logical/physical sector 512. Disk last LBA (GPT header `AlternateLBA`) 6703175. Protective MBR present (`55aa`).

| # | name | first LBA | last LBA | start byte | length (bytes) | length |
|---|---|---|---|---|---|---|
| 0 | `esp` | 2048 | 4196351 | 1048576 | 2147483648 | 2048 MiB |
| 1 | `seed-data` | 4196352 | 4401151 | **2148532224** | 104857600 | 100 MiB |
| 2 | `IncusOS_202608102114_verity_sig` | 4401152 | 4401183 | 2253389824 | 16384 | 16 KiB |
| 3 | `IncusOS_202608102114_verity` | 4401184 | 4605983 | 2253406208 | 104857600 | 100 MiB |
| 4 | `IncusOS_202608102114` | 4605984 | 6703135 | 2358263808 | 1073741824 | 1024 MiB |

First usable LBA 2048; last usable 6703142. `esp` starts at 1 MiB.

---

## Wall-clock timings (feeds spike 1.E)

Host: Darwin arm64, Apple M4 Max. Sequential `go run` / compiled `all` against local disk.

| step | wall | notes |
|---|---|---|
| download gz | **14 s** | curl, 415 MiB, ~31 MiB/s |
| decompress + GPT | **2.437 s** | pgzip 2.436 s; GPT parse 49–61 µs |
| seed tar | **< 1 ms** | 3072 bytes |
| splice | **1.728 s** | 3.2 GiB stream copy, 4 MiB buffer |
| verify (tar + yaml) | **< 1 ms** | `ReadAt` 3072 bytes + untar + YAML |
| verify (untouched-region sha256) | **2.725 s** | streamed sha256 of `[0, offset)` + `[offset+tarLen, EOF)` vs source; both identical |
| second splice (kernel+security) | **1.318 s** | same image, 5120-byte tar |

gzip CLI decompress of the same file was 2 s (pre-check); pgzip in-process is in the same band.

Phase 2 budget: image fetch dominates; splice of a 3.2 GiB raw is ~1–2 s local SSD; GPT is free.

---

## MANIFEST.txt

```
version: 202608102114
arch: aarch64
filename: aarch64/IncusOS_202608102114.img.gz
url: https://images.linuxcontainers.org/os/202608102114/aarch64/IncusOS_202608102114.img.gz
sha256_gz: 4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75
size_gz: 435456672
sha256_seeded: ecd75e4723ead89bfd123186a7f1889d8255a011d7fffe80b2f059f0d4b2d545
size_seeded: 3432026112
seed-data offset: 2148532224
```
