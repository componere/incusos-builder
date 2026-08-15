# Spike 1.C — live update-server layout + metadata sizes

Date: 2026-08-15.
Server: `https://images.linuxcontainers.org/os`.
Scratch: `spikes/server/` (`go run .` from that directory).
Non-goals honored: metadata only; no full-image downloads; no other spike directories touched.

Upstream types: `github.com/lxc/incus-os/incus-osd/api/images` (`UpdateFull.URL` is `json:"url,omitempty"` in `api/images/update.go`). Pin used by the scratch module: `v0.0.0-20260815030500-0f5b8057f2fc` (commit `0f5b8057`).

---

## 1. Index layout

`GET /index.json` → HTTP 200, `Content-Type: application/json`, **35672 bytes**.
Strict-decodes as `apiimages.Index` with `DisallowUnknownFields`.

**Top-level shape** (`apiimages.Index`):

| JSON field | type | live value |
|---|---|---|
| `format` | string | `"1.0"` |
| `updates` | array of `UpdateFull` | 3 entries, newest first |

**Per-update fields** (`apiimages.Update` + `UpdateFull.URL`):

| JSON field | type | notes |
|---|---|---|
| `format` | string | `"1.0"` on every live entry |
| `version` | string | datetime-like, e.g. `202608102114` |
| `origin` | string | `linuxcontainers.org` |
| `severity` | string | `high` / `none` (enum `UpdateSeverity`) |
| `published_at` | RFC3339 nano UTC | e.g. `2026-08-10T22:30:02.88333048Z` |
| `channels` | `[]string` | see §6 |
| `files` | `[]UpdateFile` | 55 files per current update |
| `url` | string | **present on all 3** — see §5 |

**Versions listed** (index order = newest first): `202608102114`, `202608072311`, `202608021451`.

**Per-file fields** (`apiimages.UpdateFile`) — union of keys on the live index is exactly:

`architecture`, `component`, `filename`, `sha256`, `size`, `type`

No other file keys. Hash field name is `sha256` (64 lowercase hex chars). `size` is a positive int64.

**Architectures** on every current update: `aarch64` (27 files), `x86_64` (27 files), empty/`""` (1 file). Empty architecture is `UpdateFileArchitectureUndefined` and is used for the arch-independent Secure Boot bundle `IncusOS_2026_R1.tar.gz` (`type=update-secureboot`, size 1638).

**Types observed:** `application`, `changelog`, `image-iso`, `image-manifest`, `image-raw`, `update-efi`, `update-secureboot`, `update-usr`, `update-usr-verity`, `update-usr-verity-signature`.

**Components observed:** `debug`, `gpu-support`, `incus`, `incus-ceph`, `incus-linstor`, `migration-manager`, `openfga`, `operations-center`, `os`.

**Filenames** are often two-segment (`aarch64/…`, `x86_64/…`). The charset allowlist in ARCHITECTURE.md is therefore correctly specified **per segment**. `url.JoinPath(serverURL, version, filename)` produces a working nested URL (confirmed below).

### Trimmed real excerpt (newest update, first two files)

```json
{
  "format": "1.0",
  "updates": [
    {
      "format": "1.0",
      "channels": ["testing", "stable"],
      "files": [
        {
          "architecture": "aarch64",
          "component": "os",
          "filename": "aarch64/IncusOS_202608102114.img.gz",
          "sha256": "4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75",
          "size": 435456672,
          "type": "image-raw"
        },
        {
          "architecture": "aarch64",
          "component": "os",
          "filename": "aarch64/IncusOS_202608102114.iso.gz",
          "sha256": "d949e6cd9e651deff85190dd91217cde747201d8330e866bc7d90f27906b4243",
          "size": 436472054,
          "type": "image-iso"
        }
      ],
      "origin": "linuxcontainers.org",
      "published_at": "2026-08-10T22:30:02.88333048Z",
      "severity": "high",
      "url": "/202608102114",
      "version": "202608102114"
    }
  ]
}
```

(Live update has 55 files; excerpt truncated.)

---

## 2. Asset URL shape — confirmed

Shape `serverURL/<version>/<filename>` is live, including filenames that contain `/`.

Probe file (smallest on newest update): `aarch64/changelog-stable.yaml.gz`, index `Size=306`.

Constructed URL:

`https://images.linuxcontainers.org/os/202608102114/aarch64/changelog-stable.yaml.gz`

| request | status | length evidence |
|---|---|---|
| `HEAD` | **200** | `Content-Length: 306` — **equals index `Size`** |
| `GET` `Range: bytes=0-15` | **206** | `Content-Length: 16`, `Content-Range: bytes 0-15/306`, body 16 bytes |

Server advertises `accept-ranges: bytes`. Only 16 bytes of the asset were transferred.

`update.json` / `update.sjson` are **not** in the `files` list. They sit next to the assets at `/<version>/update.json` and `/<version>/update.sjson`, matching `image-customizer` `buildImage` (`filepath.Join(mirror, version, asset)` with those two names appended literally).

---

## 3. Per-version metadata existence + sizes

Newest version: `202608102114`.

| URL | status | exact bytes |
|---|---|---|
| `/202608102114/update.json` | 200 `application/json` | **11859** |
| `/202608102114/update.sjson` | 200 `application/octet-stream` | **14268** |
| `/index.sjson` (supporting) | 200 `application/octet-stream` | **38081** |
| `/index.json` | 200 `application/json` | **35672** |

---

## 4. `update.sjson` parse

**Approach:** `net/mail.ReadMessage` for the RFC 822 header block, `mime.ParseMediaType` for `Content-Type` params, then `mime/multipart.NewReader` on the body with the `boundary` param. Each part is `io.ReadAll`'d immediately (`NextPart` invalidates the previous part's body). Signature part is base64-decoded to DER and passed to `github.com/smallstep/pkcs7.Parse`. Payload is strict-JSON-decoded into `apiimages.Update` (`DisallowUnknownFields`).

This is extraction + structural validation, not signature authentication (that remains a follow-up with a user-supplied CA).

**MIME envelope:**

- `Content-Type: multipart/signed`
- `protocol=application/x-pkcs7-signature`
- `micalg=sha-256` (digest algorithm)
- 2 parts:
  1. `text/plain` — JSON payload
  2. `application/x-pkcs7-signature; name="smime.p7s"`, `Content-Transfer-Encoding: base64`, detached PKCS#7 (`p7.Content` length 0)

**PKCS#7 signer info** (`pkcs7.Parse` + `GetOnlySigner`; openssl `pkcs7 -print` agrees):

| | |
|---|---|
| Certificates present | **2** |
| cert[0] | subject `CN=Incus OS - Update E1, O=Linux Containers`; issuer `CN=Incus OS - Root E1, O=Linux Containers`; ECDSA-SHA384; notAfter 2045-06-21 |
| cert[1] (signer) | subject `CN=Incus OS - Update 2025 E1, O=Linux Containers`; issuer `CN=Incus OS - Update E1, O=Linux Containers`; ECDSA-SHA384; notAfter 2027-06-26 |
| Signer serial | `57b38fa7fb43a675bc86129c317e86908866d3e2` |
| SignedData digest alg | **SHA-256** (`2.16.840.1.101.3.4.2.1`), matching `micalg` |
| Signature alg on certs | ecdsa-with-SHA384; signer key is P-256 |

**Parse / version-match / validation:**

- strict-decode sjson payload as `apiimages.Update`: **OK**
- sjson payload version `202608102114` **matches** `update.json` twin
- file-count 55 = 55
- every file: parseable sha256 hex (32 bytes) + positive size: **OK, 0 anomalies**
- one empty-architecture file (`IncusOS_2026_R1.tar.gz`) — expected, not an anomaly
- three-way `(Filename, Sha256, Size)` binding on newest release `202608102114` (index entry / `update.json` body / sjson payload; `UpdateFull` embeds `Update`, all three are `[]UpdateFile`): **AGREE**
  - counts: index=55, update.json=55, sjson=55
  - positional: 55/55 agree (index == update.json == sjson)
  - live program output:

```
--- 4b. three-way (Filename, Sha256, Size) binding ---
entry-by-entry across index.Updates[0].Files / update.json / sjson payload
(UpdateFull embeds Update; all three are []UpdateFile)
counts: index=55 update.json=55 sjson=55
positional: 55/55 agree (index == update.json == sjson)
verdict: AGREE
```

Extracted payload after `bytes.TrimSpace` is 11858 bytes vs 11859-byte `update.json` (trailing newline on the unsigned twin). Structurally equal; Phase 3a must copy the **HTTP body of `update.sjson` verbatim**, not the extracted JSON.

---

## 5. `UpdateFull.URL` overrides

**Present on every live update** (3/3), always the relative path `/{version}`:

- `"/202608102114"`
- `"/202608072311"`
- `"/202608021451"`

Set by `image-publisher` `generateIndex` (`UpdateFull{URL: "/" + entry.Name(), Update: update}`). These are **not** absolute base-URL overrides.

Neither the images provider (`serverURL + "/" + version + "/" + filename`) nor the customizer (`Join(mirror, version, asset)`) reads `UpdateFull.URL`. Asset URLs constructed from `serverURL/<version>/<filename>` work without it.

**Phase 3a:** ignore `URL` the same way upstream consumers do. Do not treat a leading `/` as a new host. A future absolute `https://…` override is still possible in the type but is unused on the live server today.

---

## 6. Channels

Observed values: **`testing`** and **`stable`**.

Membership is a **per-update `channels []string`**, not a top-level map. Every current update lists both:

```json
"channels": ["testing", "stable"]
```

So channel filtering is `slices.Contains(update.Channels, requested)` (as `provider_images.checkRelease` already does). There is no per-file channel field; changelogs encode the channel in the filename (`changelog-stable.yaml.gz` / `changelog-testing.yaml.gz`) instead.

---

## 7. Size measurements + DECISION

| document | bytes | KiB |
|---|---|---|
| `update.json` | 11859 | 11.58 |
| `update.sjson` | 14268 | 13.93 |
| `index.json` | 35672 | 34.84 |
| `index.sjson` | 38081 | 37.19 |

Largest **ReleaseMetadata** document (`update.json` / `update.sjson`): **14268 bytes ≪ 1 MiB**.

Headroom vs proposed caps (against 14268):

- 1 MiB: **73.5×**
- 8 MiB (plan placeholder): **587.9×**

Index stays on the planned **64 MiB** `LimitReader` (ARCHITECTURE §6). Three updates already make the index ~2.5× a single `update.json`; the 64 MiB bound is still ~1700× today's index.

**DECISION for Phase 3a:** cap `update.json` and `update.sjson` reads at **1 MiB**.

Rationale: the plan's 8 MiB default was only for the unmeasurable case. Live size is ~14 KiB. 1 MiB matches the “expected ≪ 1 MiB” note, leaves ~70× room for file-list growth, and is a tighter DoS bound than the placeholder.

Checkable arithmetic (recheck the cap without re-running the spike):

- ~216 B per file entry (11,859 B / 55 files)
- sjson fixed overhead ~2.4 KiB (14,268 − 11,859 = 2,409 B of MIME envelope + PKCS#7)
- 1 MiB ceiling ≈ (1,048,576 − 2,400) / 216 ≈ **4,800 file entries**, vs 55 today across two architectures
- Revisit trigger: a release's metadata approaching a few hundred KiB

No `spikes/server/.gitignore`: nothing persisted to disk, and both metadata documents are well under 1 MiB.

---

## Scratch program output

```
=== Spike 1.C — live update-server layout + metadata sizes ===
server: https://images.linuxcontainers.org/os

--- 1. index.json structure ---
bytes: 35672
format: "1.0"
updates: 3

update[0]:
  format="1.0" version="202608102114" origin="linuxcontainers.org" severity="high" published_at=2026-08-10T22:30:02.88333048Z
  channels=[testing stable]
  url (UpdateFull.URL json:"url,omitempty")="/202608102114" (present=true)
  files=55
  architectures: <empty>=1, aarch64=27, x86_64=27
  types: application=18, changelog=4, image-iso=2, image-manifest=20, image-raw=2, update-efi=2, update-secureboot=1, update-usr=2, update-usr-verity=2, update-usr-verity-signature=2
  components: debug=4, gpu-support=4, incus=8, incus-ceph=4, incus-linstor=4, migration-manager=4, openfga=4, operations-center=4, os=19
  smallest file: aarch64/changelog-stable.yaml.gz size=306 type=changelog sha256=f6ac06f37b5cde02935f15834a5fd5615e0f94025315596b8fbb73c89396a4c0

update[1]:
  format="1.0" version="202608072311" origin="linuxcontainers.org" severity="high" published_at=2026-08-08T00:30:03.00378509Z
  channels=[testing stable]
  url (UpdateFull.URL json:"url,omitempty")="/202608072311" (present=true)
  files=55
  architectures: <empty>=1, aarch64=27, x86_64=27
  types: application=18, changelog=4, image-iso=2, image-manifest=20, image-raw=2, update-efi=2, update-secureboot=1, update-usr=2, update-usr-verity=2, update-usr-verity-signature=2
  components: debug=4, gpu-support=4, incus=8, incus-ceph=4, incus-linstor=4, migration-manager=4, openfga=4, operations-center=4, os=19
  smallest file: aarch64/gpu-support.manifest.json.gz size=353 type=image-manifest sha256=8828b3d0facf5bbc3efc342d317acb384a9369acce909e9f6d11169f26e82c76

update[2]:
  format="1.0" version="202608021451" origin="linuxcontainers.org" severity="none" published_at=2026-08-04T18:04:19.35641104Z
  channels=[testing stable]
  url (UpdateFull.URL json:"url,omitempty")="/202608021451" (present=true)
  files=55
  architectures: <empty>=1, aarch64=27, x86_64=27
  types: application=18, changelog=4, image-iso=2, image-manifest=20, image-raw=2, update-efi=2, update-secureboot=1, update-usr=2, update-usr-verity=2, update-usr-verity-signature=2
  components: debug=4, gpu-support=4, incus=8, incus-ceph=4, incus-linstor=4, migration-manager=4, openfga=4, operations-center=4, os=19
  smallest file: aarch64/gpu-support.manifest.json.gz size=349 type=image-manifest sha256=34d254526bf08e05bd476202328037d5d7ba424f22a0a76a056c8d868dc83c50

per-file JSON field names: architecture, component, filename, sha256, size, type
updates carrying UpdateFull.URL: 3 / 3
channels observed (update membership count): stable=3, testing=3
versions listed (index order): 202608102114, 202608072311, 202608021451

newest version (first index entry): 202608102114

--- 2. asset URL shape ---
constructed: https://images.linuxcontainers.org/os/202608102114/aarch64/changelog-stable.yaml.gz
index Size: 306
HEAD status: 200  Content-Length: 306  matches index Size: true
Range GET bytes=0-15 status: 206  Content-Length: 16  Content-Range: bytes 0-15/306  body bytes: 16

--- 3. per-version metadata ---
GET https://images.linuxcontainers.org/os/202608102114/update.json  bytes=11859
GET https://images.linuxcontainers.org/os/202608102114/update.sjson  bytes=14268
GET https://images.linuxcontainers.org/os/index.sjson  bytes=38081 (supporting; not the ReleaseMetadata cap target)

--- 4. parse update.sjson as multipart/signed ---
approach: net/mail.ReadMessage for RFC 822 headers, mime.ParseMediaType,
then mime/multipart.NewReader on the body using the boundary param.
  part[0] Content-Type="text/plain" encoding="" disposition="" bytes=11860
  part[1] Content-Type="application/x-pkcs7-signature; name=\"smime.p7s\"" encoding="base64" disposition="attachment; filename=\"smime.p7s\"" bytes=1914
Content-Type: multipart/signed
protocol: application/x-pkcs7-signature
micalg (digest algorithm): sha-256
boundary: ----B48B16031C0971EAB7E41BAFB11260B3
parts: 2  payload bytes: 11858  pkcs7 DER bytes: 1412
PKCS#7 certificates present: 2
  cert[0] subject="CN=Incus OS - Update E1,O=Linux Containers" issuer="CN=Incus OS - Root E1,O=Linux Containers" sigalg=ECDSA-SHA384 notAfter=2045-06-21T08:10:54Z
  cert[1] subject="CN=Incus OS - Update 2025 E1,O=Linux Containers" issuer="CN=Incus OS - Update E1,O=Linux Containers" sigalg=ECDSA-SHA384 notAfter=2027-06-26T08:10:54Z
PKCS#7 GetOnlySigner: subject="CN=Incus OS - Update 2025 E1,O=Linux Containers" serial=57b38fa7fb43a675bc86129c317e86908866d3e2
PKCS#7 attached content bytes: 0 (detached signature => 0)
strict-decode sjson payload as apiimages.Update: OK
sjson payload version: 202608102114
update.json version:   202608102114
version match: true
file-count match: sjson=55 json=55 equal=true
sjson channels: [testing stable]
empty-architecture file: filename=IncusOS_2026_R1.tar.gz type=update-secureboot size=1638
empty-architecture files: 1

--- 4b. three-way (Filename, Sha256, Size) binding ---
entry-by-entry across index.Updates[0].Files / update.json / sjson payload
(UpdateFull embeds Update; all three are []UpdateFile)
counts: index=55 update.json=55 sjson=55
positional: 55/55 agree (index == update.json == sjson)
verdict: AGREE

--- 5. structural validation (every file: parseable sha256 hex + positive size) ---
validation: OK (55 files, no anomalies)

--- 6. metadata size cap ---
index.json:     35672 bytes (34.84 KiB)
index.sjson:    38081 bytes (37.19 KiB)
update.json:    11859 bytes (11.58 KiB)
update.sjson:   14268 bytes (13.93 KiB)
largest ReleaseMetadata document: 14268 bytes
vs 1 MiB:  73.5x headroom
vs 8 MiB:  587.9x headroom
DECISION: Phase 3a should cap update.json / update.sjson at 1 MiB.
Observed max is well under 1 MiB; 1 MiB leaves ~70x headroom and is tighter
than the 8 MiB placeholder. Index stays at the planned 64 MiB LimitReader.

--- trimmed index excerpt (newest update, first 2 files) ---
{
  "format": "1.0",
  "updates": [
    {
      "channels": [
        "testing",
        "stable"
      ],
      "files": [
        {
          "architecture": "aarch64",
          "component": "os",
          "filename": "aarch64/IncusOS_202608102114.img.gz",
          "sha256": "4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75",
          "size": 435456672,
          "type": "image-raw"
        },
        {
          "architecture": "aarch64",
          "component": "os",
          "filename": "aarch64/IncusOS_202608102114.iso.gz",
          "sha256": "d949e6cd9e651deff85190dd91217cde747201d8330e866bc7d90f27906b4243",
          "size": 436472054,
          "type": "image-iso"
        }
      ],
      "files_total": 55,
      "files_truncated": true,
      "format": "1.0",
      "origin": "linuxcontainers.org",
      "published_at": "2026-08-10T22:30:02.88333048Z",
      "severity": "high",
      "url": "/202608102114",
      "version": "202608102114"
    }
  ],
  "updates_total": 3
}
=== done ===
```
