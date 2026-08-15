# incusos-builder Architecture

## 1. Goals / non-goals

**Goals**

- A Go CLI that turns one YAML config file into seeded IncusOS install media, locally, with zero dependency on the web customizer service.
- Full setting coverage: everything the web UI exposes, plus everything the seed API supports that the UI hides — including the `kernel` and `security` seeds the web service cannot inject at all.
- **Offline installs are in scope for v1.** `image.offline: true` produces the same two artifacts the web service does: the seeded OS image *and* the `RESCUE_DATA` resources media carrying the selected applications plus the signed release metadata the booted recovery path requires.
- Native SOPS decryption of the config; plaintext secrets never touch disk.
- First-class automation (stable exit codes, one JSON envelope, no prompts without a TTY) and first-class interactive UX (Lip Gloss / Huh / Log), per the `cli` and `charmbracelet` skills.
- Conformance with AGENTS.md rules A/T/R/P/D/I/E/L.

**Non-goals**

- No HTTP server, no web UI, no OIDC relay (`oidc.go` upstream is service plumbing, not build logic).
- No re-implementation of the update *server*; we are a client of `https://images.linuxcontainers.org/os`.
- No update-tarball artifact (`sendUpdateTarball`) in v1 — a distinct artifact type absent from the brief's UI surface; follow-up in §10.
- No Bubble Tea TUI.

## 2. Upstream findings

All citations are into `reference/incus-os/`.

### End-to-end flow of the web customizer

1. **Request intake** — `recordFileRequest` (`incus-osd/cmd/image-customizer/main.go:265`) strict-decodes an `apicustomizer.ImagesPost` (`DisallowUnknownFields`, recursive over all seed structs), stores the raw request in a 10-minute TTL map keyed by UUID, and returns download URLs. **If `offline: true`, a second entry of type `rescue` is stored and a `resources` download URL is returned alongside `image`** (`main.go:317–321`). Offline is therefore a two-artifact operation, not a seed tweak.
2. **Base image acquisition** — the service serves from a local mirror of the update server. `parseIndex` (`main.go:872`) reads `index.json` into `apiupdate.Index`; `filterAssets` (`main.go:823`) picks the update whose `Channels` contains the requested channel and whose `Version` is an exact match or the highest, filtering `Files` by architecture/type/component and returning **filenames**. `sendOSImage` (`main.go:399`) requires exactly one matching `image-iso`/`image-raw` asset; channel defaults to `"stable"` (`main.go:434–437`).
   - The real update server (what our CLI talks to): `internal/providers/provider_images.go` — default `https://images.linuxcontainers.org/os` (`:319–320`); index at `/index.sjson`, S/MIME-verified against an update CA (`:396–425`), **or `/index.json` permitted only over HTTPS** (`:428–434`); every asset at `serverURL + "/" + version + "/" + file.Filename` (`:549`, `:614`, `:640`, `:684`), each with `Sha256` and `Size` (`api/images/update_file.go`).
   - The S/MIME path is not directly reusable: `util.VerifySMIME` lives in `internal/util` (`ssl.go:108`), shells out to `openssl smime`, and anchors trust in `certs.GetEmbeddedCertificates()` — whose CA PEMs are injected at upstream build time and **absent from this checkout** (`incus-osd/certs/files/` contains only `production-cert-subject-key-ids.txt`).
   - **Index metadata is untrusted input.** `UpdateFull.Version` and `UpdateFile.Filename` are unconstrained JSON strings (`api/images/update.go`, `api/images/update_file.go`); nothing upstream sanitizes them before path use (the customizer joins them straight into mirror paths, `main.go:934`). Our client validates them before any URL or filesystem use (§6).
3. **Seed construction** — `writeSeed` (`main.go:1000`) serializes each non-nil member of `ImagesPostSeeds` to YAML (`yaml.Dump(..., yaml.WithV2Defaults())`, `go.yaml.in/yaml/v4`) into a plain uncompressed tar with **nine** entry names `applications.yaml`, `incus.yaml`, `operations-center.yaml`, `migration-manager.yaml`, `install.yaml`, `network.yaml`, `provider.yaml`, `services.yaml`, `update.yaml` (mode `0600`), returning the tar's byte size. (`ImagesPostSeeds`, `api/customizer/images.go:20–30`, has only these nine.)
4. **Injection mechanics** — `sendOSImage` streams exactly **2,148,532,224 bytes** of decompressed base image, then the seed tar, then skips the source forward by the tar size and streams the remainder (`main.go:501–545`): the tar overwrites the pre-provisioned seed partition in place. That offset is not arbitrary: both layouts place a **fixed 100 MiB `seed-data` partition** directly after the 2 GiB ESP — `SizeMinBytes=100M`/`SizeMaxBytes=100M` in `mkosi.repart/01-seed-data.conf:2–5` for raw images, and `sgdisk -n 2::+100MiB … -c 2:seed-data` after `-n 1::+2GiB` in `scripts/convert-img-to-iso.sh:37–38` for ISOs (2 GiB ESP + 1 MiB GPT/alignment = 2,148,532,224). Nothing upstream bounds the tar against that partition; an oversized tar would silently overwrite the next partition. Our splice makes both facts checked invariants (§7). On boot, `incus-osd` reads that partition (partlabel `seed-data` or a user `SEED_DATA` volume, `internal/seed/seed.go`) as a tar, extracting sections by filename with `.json`/`.yaml`/`.yml` accepted and **strict decoding** — unknown fields fail the boot-time parse.
5. **Offline forcing** — `offline: true` forces `Seeds.Update.CheckFrequency = "never"`, creating `&apiseed.Update{Version: "1"}` if nil (`main.go:438–445`).
6. **Rescue/resources media** — `sendRescueImage` (`main.go:562`) requires a non-empty `applications` seed (`:604–616`), matches each application by **filename** `<name>.raw.gz` against the update's asset list (`:626–636`), then **appends the literal names `update.json` and `update.sjson`** to the asset list (`:645`). These two files are *not* `UpdateFile` entries — they have no index-provided digest; `buildImage` (`main.go:912`) reads every asset from `<mirror>/<version>/<name>` verbatim, stages everything under `update/`, then builds either an ISO (`mkisofs -V RESCUE_DATA -joliet-long -rock`) or raw media (`truncate` + `sgdisk -n 1 -c 1:RESCUE_DATA` + `mkfs.vfat -S 512 --offset=2048` + `mcopy` at the 1 MiB partition offset).
   - **`update.sjson` is the trusted document — the only one.** The booted recovery path (`internal/recovery/recovery.go:31–48`) mounts any vfat or iso9660 volume labeled `RESCUE_DATA`; its `applyUpdate` **returns immediately doing nothing if `update/update.sjson` is absent** (`recovery.go:178–182`), and otherwise S/MIME-verifies it against the embedded update CA, decodes the verified payload as `apiupdate.Update`, refuses downgrades, and then admits application files **exclusively by the filename/sha256 list inside that verified payload** (`recovery.go:184–263`). `update.json`, the unsigned twin, is never read by recovery. The document itself is a `multipart/signed` S/MIME message (`openssl smime -verify`, `ssl.go:161`; format asserted in `recovery.go:RunSignedScript`), so its clear-text JSON payload is extractable locally without any key material — which is what our build-time structural validation exploits (§6).

### Seed sections and where they're consumed

`api/seed/` defines **eleven** sections and `incus-osd` reads all eleven from the seed partition. `kernel` and `security` are absent from the web API and UI — a CLI writing the tar directly can inject them. One caveat discovered in the osd consumer: `internal/seed/security.go:GetSecurity` **rejects any non-empty `encryption_recovery_keys` list** ("it is not possible to set encryption recovery key(s) via the security seed") and daemon startup treats that as a fatal seed error (`cmd/incus-osd/main.go:211–215`). The field exists structurally (`api/system_security.go:28–31`) but is a forbidden input; our validator rejects it too.

### Complete setting surface

**Image-level (`ImagesPost`)** — *UI-visible*: `type` (iso/raw), `architecture` (x86_64/aarch64), `channel` (free text, default `stable`), `offline` (checkbox). *Hidden*: `version` (exact release pin; no UI field).

**Seeds** (UI evidence: `html/index.html` inputs + `html/js/local.js:50–228`):

| Section | UI-visible | Hidden (API/seed-only) |
|---|---|---|
| `applications` | single app radio (incus, incus-lts-7.0, operations-center, migration-manager) | arbitrary list of `{name}` (`api/seed/applications.go`) |
| `install` | usage toggle, `force_install`, `force_reboot`, `target{id,bus,min_size,max_size,sort_order}`, `security{missing_tpm,missing_secure_boot}` | `force_install_confirmation` (`api/seed/install.go`) |
| `incus` | `apply_defaults`, admin client certificate, OIDC issuer/client-id/scopes/claim | the entire `incusapi.InitPreseed` (storage pools, networks, profiles, projects, server config), arbitrary certificate list (`api/seed/incus.go`) |
| `network` | YAML textarea — client-side it is `jsyaml.load`ed into the request object (`local.js:147–152`) and **server-side strict-decoded** like every other seed | full typed surface: `confirmation_timeout`, `dns`, `time`, `proxy`, `interfaces[]`, `bonds[]`, `vlans[]`, `wireguard[]` incl. peers/routes/firewall rules (`api/system_network.go`) |
| `migration-manager` | trusted client cert, OIDC | `apply_defaults`, full `MigrationManagerPreseed` (`api/seed/migration_manager.go`) |
| `operations-center` | trusted client cert, OIDC | `apply_defaults`, full `OperationsCenterPreseed` (`api/seed/operations_center.go`) |
| `provider` | — | `name`, `config{}` map (`api/system_provider.go`) |
| `services` | — | `iscsi`, `lvm`, `multipath`, `netbird`, `nvme`, `ovn`, `tailscale`, `usbip` (`api/seed/services.go`) |
| `update` | — (offline forces `check_frequency: never`) | `auto_reboot`, `channel`, `check_frequency`, `maintenance_windows[]` (`api/system_update.go`) |
| `kernel` | **not injectable via web service** | `console[]{device,baud_rate}` (`api/seed/kernel.go`) |
| `security` | **not injectable via web service** | `custom_ca_certs[]` only — `encryption_recovery_keys` is structurally present but **rejected by osd at boot**; our validator refuses it (exit 3) with the upstream rationale |

## 3. CLI surface

Binary: `incusos-builder` (module `github.com/componere/incusos-builder`; the template rename lands with implementation).

```
incusos-builder build     -f <config.yaml|-> -o <out.iso|out.img|out.*.gz|->
                          [--resources-output PATH] [--force] [--server URL|DIR]
                          [--cache-dir DIR] [--json]
incusos-builder validate  -f <config.yaml|->  [--json]
incusos-builder versions  [--channel stable] [--architecture x86_64] [--server URL|DIR] [--json]
incusos-builder init      [-o config.yaml]   (Huh form when interactive; commented example otherwise)
incusos-builder completion|help|--version    (Cobra built-ins; --version prints BuildInfo
                                             including the pinned incus-os API version)

Global: --color auto|always|never · --progress auto|always|never · --no-input
        --verbose/-q · standard Cobra --help
```

**Interaction policy** (one global, three knobs — every interactive feature is covered):

- `--no-input`: no prompts anywhere. Auto-enabled when stdin or stdout is not a TTY or `CI` is set. Under `--no-input`: `build` requires `--force` to overwrite (missing → exit 2, never a hang); `init` deterministically writes the commented example config, even in a TTY.
- `--color`: `auto` (default) styles only when the target stream is a TTY and `NO_COLOR`/`TERM=dumb` are unset; `always`/`never` override.
- `--progress`: `auto` (default) renders the progress line on stderr only when **both stdout and stderr are TTYs** (the brief's stdout-TTY degradation rule); `always`/`never` override. `--json` and `-o -` never affect correctness of piped data because progress lives on stderr.

**Stdout contract** (streams never mix):

- Normal mode: artifacts go to their `-o`/`--resources-output` files via the publisher lifecycle below; stdout carries the human summary; stderr carries logs/progress.
- `--json`: stdout carries exactly one JSON envelope and nothing else — success `{"result": {output, resources_output?, type, architecture, version, channel, seed_bytes, sha256, resources_sha256?}}` or failure `{"error": {code, message}}` with the matching exit code. `validate`/`versions` use the same envelope shape.
- `-o -`: stdout is reserved **exclusively** for artifact bytes; summary suppressed entirely; diagnostics on stderr; consumers must check the exit code because a mid-stream failure truncates stdout. `--json` with `-o -` is a usage error (exit 2). `offline: true` with `-o -` is a usage error (two artifacts cannot share one stream).
- Reported `sha256` values cover **the exact bytes stored at the named output** (post any `.gz` recompression), one digest per artifact.
- `--` ends option parsing; `-f -` reads the config from stdin (SOPS-encrypted stdin included).

**Output publication lifecycle** (owned by `internal/cli`; one policy for one or two artifacts):

- **Path validation first**: with `offline: true`, `--resources-output` defaults to `<out-stem>.resources.<iso|img>` beside `-o`. After cleaning, the two final paths must be distinct and neither may be `-`; violation is a usage error (exit 2).
- **Overwrite policy, checked twice**: a pre-work existence check on both final paths fails fast — before any multi-GB download — unless `--force` (or an interactive confirm covering both, when input is allowed) authorizes replacement. This check is *UX*, not the enforcement: publication itself is no-clobber (below), so a file appearing between check and publish is refused, never overwritten.
- **Unique temporaries**: each artifact is produced into an `os.CreateTemp(dir(dest), ".<base>-*.tmp")` file in its destination directory — exclusive by construction, so concurrent builds targeting the same destination never share a temp. The image temp is written through a hashing writer during the splice; the finished resources temp is fsynced and hashed by a sequential re-read. Digests are a *publication* concern: the domain never does filesystem I/O to compute them.
- **No-clobber publication (default, race-free)**: nothing touches a final path until every artifact is complete, fsynced, and hashed. Each artifact is then published by **claim-then-rename**: `os.OpenFile(final, O_CREAT|O_EXCL, 0644)` atomically claims the name (EEXIST → `ErrOutput`, exit 6, "output appeared during the build; re-run with --force"), then `os.Rename(tmp, final)` replaces our own zero-byte claim with the complete artifact. Two concurrent non-force builds can never overwrite each other: exactly one wins each claim. Order is **resources first, image last** — the image's appearance is the commit marker; a visible image implies a complete build. *Rejected alternatives*: `os.Link(tmp, final)` (atomic in one call but fails on FAT-family targets — USB media directories are plausible destinations) and `renameat2(RENAME_NOREPLACE)` (Linux-only).
- **Forced replacement (`--force`), both-or-nothing on handled errors**: the old pair is preserved until the new pair is fully published.
  1. Both temps complete, fsynced, hashed.
  2. `rename(image → image + ".incusos-builder.bak")` — the commit marker disappears *first*.
  3. `rename(resources → resources + ".incusos-builder.bak")` (if present).
  4. Publish new resources (claim-then-rename).
  5. Publish new image (claim-then-rename) — commit.
  6. Best-effort remove both `.bak` files; leftovers are reported and harmless.
  On any handled failure in steps 2–5, the publisher undoes in reverse: removes any new finals it published, restores `resources.bak`, restores `image.bak` **last** — the marker reappears only once its paired resources are back — reports every step taken, and exits 6. Backup names are deterministic so manual recovery after a crash is a documented rename; under `--force`, replacing a stale same-named backup is accepted behavior.
  **Crash scope, stated honestly**: there is no transaction journal in v1; the both-or-nothing guarantee covers handled errors. What the *ordering alone* guarantees even for a kill -9: an old image final can never be paired with new resources (the old image is moved aside in step 2 before resources is touched, and the new image appears only after new resources in step 5). At every crash point the image path is either absent — unmistakably incomplete, with `.bak` recovery documented — or refers to a complete artifact consistent with the visible resources file. Online (single-artifact) builds are the degenerate case with steps 3–4 skipped.
- **Cleanup**: on any handled failure, all temps and claims are removed and backups restored; a complete-but-unpublished temp never masquerades as an output. Nothing partial is ever left at a final path: temps live under dot-prefixed unique names and only complete artifacts are ever renamed in.

**Config precedence** (tool behavior only — the YAML file is the build *spec*, not tool config): flags > `INCUSOS_BUILDER_*` env > defaults, via the template's Viper wiring in `internal/cli`. Applies to `--server`, `--cache-dir`, `--json`, `--color`, `--progress`, `--no-input`.

**Exit codes** (mapped from E1 sentinels in `internal/cli`): `0` success · `1` internal/unexpected · `2` usage error · `3` invalid config (`ErrConfig`) · `4` decryption failure (`ErrDecrypt`) · `5` acquisition failure (`ErrFetch`) · `6` output write failure (`ErrOutput`).

**Rejected alternative**: a `--set key=value` override surface for seed fields. The seed tree is deep and typed; a flat override DSL would be a second, stringly config language (violates I1's spirit). One YAML file is the contract; automation edits YAML.

## 4. Config schema strategy

```yaml
version: 1                    # config schema version, required
image:
  type: iso                   # iso | raw
  architecture: x86_64        # x86_64 | aarch64
  channel: stable             # optional; default stable
  release: ""                 # optional exact version pin (upstream ImagesPost.Version)
  offline: false              # true ⇒ also build RESCUE_DATA resources media
seeds:                        # every key optional; shapes are upstream seed types verbatim
  applications: { ... }       # api/seed.Applications
  install:      { ... }       # api/seed.Install
  incus:        { ... }       # api/seed.Incus (full InitPreseed available)
  network:      { ... }       # api/seed.Network
  provider:     { ... }
  services:     { ... }
  update:       { ... }
  kernel:       { ... }       # CLI extension — not available in the web customizer
  security:     { ... }       # CLI extension — custom_ca_certs only (see validation)
  migration-manager: { ... }
  operations-center: { ... }
```

**Mirror vs. import — decision: import upstream types.** `internal/config` and `internal/seed` use `github.com/lxc/incus-os/incus-osd/api/seed` directly. The evidence, not just the assertion:

- *What it pulls in*: `api/seed` imports exactly three external type surfaces — `github.com/lxc/incus/v7/shared/api` (`incus.go`), `github.com/FuturFusion/operations-center/shared/api/system` (`operations_center.go`), and `github.com/FuturFusion/migration-manager/shared/api` (`migration_manager.go`). The upstream module's *build list* is much larger (tailscale, tview, tcell, umoci… per `incus-osd/go.mod`), but only reachable packages compile in; a `go list -deps` gate in CI asserts the compiled-in set stays type-only (no `internal/`, no daemon packages), turning "only types compile in" from a claim into a checked invariant.
- *Why accept it*: the booted `incus-osd` strict-decodes these exact structs; unknown fields fail the boot-time seed parse. Hand-mirroring ~50 structs across three upstream modules would silently drift and produce images that fail on first boot. Fidelity is the product.
- *Schema coupling, made explicit*: the YAML our CLI accepts is defined by the pinned upstream version. `--version` prints that pin; validation errors on unknown seed fields say "unknown to incus-os <pinned-version>; a newer incusos-builder may accept this". Bumps are deliberate, and the nine-section golden tar test (§9) is the drift alarm.
- *YAML fidelity*: the seed writer uses the same `go.yaml.in/yaml/v4` with `WithV2Defaults()` as upstream `writeSeed`, so serialization is byte-compatible, not merely semantically equivalent.

**Versioning**: `version: 1` required and checked first; unknown versions → exit 3 with a "newer CLI required" message. Per-seed `version` fields default to `"1"` when omitted (matching the UI, `local.js:110`, `:174`).

**Validation** (all in `internal/config`, pure, table-tested): strict YAML decode (unknown fields rejected — users learn at build time, not boot time; this matches the server, which strict-decodes every seed including the UI's network textarea); enum checks (architecture ∈ {x86_64, aarch64} per `main.go:425`, type ∈ {iso, raw}); cross-field rules lifted from upstream handlers:

- `offline: true` ⇒ force `update.check_frequency: never`, creating the update seed if absent (replicates `main.go:438–445`).
- `offline: true` ⇒ `seeds.applications` non-empty — required because the resources media build has nothing to carry otherwise (`sendRescueImage`, `main.go:604–616`). Applied only when offline, matching where upstream enforces it.
- `seeds.security.encryption_recovery_keys` non-empty ⇒ **error** (exit 3), because osd fatally rejects it at boot (`internal/seed/security.go`). The message quotes the upstream rationale and points at the disk-encryption docs.
- install `sort_order` ∈ {"", smallest, largest}; install security options per upstream semantics.

The seed tar's *size* is additionally bounded against the acquired image's actual seed partition at build time (§7) — a config-shape check cannot know the capacity, so the invariant lives in the splice.

## 5. SOPS integration

Lives entirely in `internal/config`; the domain never knows encryption existed.

- **Detection**: after reading the raw bytes (file or stdin), a cheap YAML probe checks for a **top-level `sops` key** — its mere presence selects the encrypted path (no filename heuristics, no inspection of the block's internals). `sops` is not a config key, so there is no ambiguity with plain configs; a plain config containing a stray `sops` key was never valid anyway.
- **Decryption**: `decrypt.Data(raw, "yaml")` from `github.com/getsops/sops/v3/decrypt` (brief-mandated). Key discovery is the library's standard chain: age (`SOPS_AGE_KEY`, `SOPS_AGE_KEY_FILE`, default keys file), PGP via gpg agent, cloud KMS via ambient credentials. No key-management surface of our own.
- **Key material handling**: decrypted bytes exist only in memory; nothing hits temp files, logs, or error messages (errors reference field *paths*, never values). Seed YAML containing secrets (wireguard keys, proxy passwords) is written only into the output image stream.
- **Failure modes — everything after detection is `ErrDecrypt` (exit 4)**: malformed or incomplete `sops` metadata (missing `version`/`mac` included), no matching key (named key sources tried), MAC mismatch (tamper warning). Detection and taxonomy agree by construction: once the `sops` key is seen, no failure on that path can fall through to a config-decode error. Encrypted stdin works identically.
- **L1 note**: sops/v3 drags cloud SDKs and inflates the binary. Accepted as a hard requirement; isolated behind `internal/config` so future slimming touches one package.

## 6. Hexagonal decomposition

```
cmd/incusos-builder/        main.go — wiring only
internal/
  cli/                      Cobra tree, flag/env precedence, TTY detection,
                            interaction policy, sentinel→exit-code mapping,
                            output publisher (temps → fsync → hash → claim →
                            ordered rename → rollback), JSON envelope
  config/                   load (file/stdin) → SOPS detect/decrypt → strict
                            decode → validate → build.Spec.  Not a domain port:
                            it produces a value before the domain runs.
  build/                    DOMAIN CORE. Spec, Plan, Result types; version
                            resolution (filterAssets logic); GPT probe, splice
                            + rescue orchestration. Defines all ports. No I/O
                            beyond injected streams and handles.
  seed/                     pure: Seeds → tar bytes + size
                            (writeSeed equivalent, + kernel/security entries)
  update/                   adapter: ImageSource — HTTPS update-server client
                            (index, verified content-addressed asset cache,
                            release metadata) AND a local-directory source
                            (mirror/testdata)
    mocks/                  mockery output for build.ImageSource + build.VerifiedAsset
  media/                    adapter: RescueWriter — pure-Go RESCUE_DATA media
                            (iso9660 / GPT+FAT32) via go-diskfs
    mocks/                  mockery output for build.RescueWriter
  ux/                       adapter: Reporter — fancy (Lip Gloss + Log) and
                            plain renderers behind one constructor
    mocks/                  mockery output for build.Reporter
```

Names are short and obvious (A4); each adapter has one purpose (A2); package cores stay unexported behind thin APIs (A3); every package gets `doc.go` (D4).

**Ports** (defined in `internal/build`; I2 — accept interfaces, return concrete types; `VerifiedAsset` is a deliberate, documented exception: the capability is adapter-defined by nature, exactly like `io.ReadCloser`):

```go
// ImageSource acquires update metadata and verified assets.
type ImageSource interface {
    Index(ctx context.Context) (apiimages.Index, error)

    // Asset downloads (or reuses from cache), verifies, and locally
    // retains one asset, returning a reusable handle. Verification —
    // exact Size byte count and Sha256 — happens here, exactly once per
    // call; opening the returned handle is cheap. Download progress is
    // reported through the Reporter the adapter was constructed with —
    // progress originates where the network I/O happens, not in the domain.
    Asset(ctx context.Context, version string, file apiimages.UpdateFile) (VerifiedAsset, error)

    // ReleaseMetadata fetches the release's update.json and update.sjson —
    // upstream appends these literal names to every rescue asset list
    // (main.go:645) and serves them from the same per-version location as
    // every other asset. They are not UpdateFile entries and carry no index
    // digest, so the adapter validates them structurally instead:
    //  (a) each read is size-capped (8 MiB default until measured, §10);
    //  (b) update.sjson — the ONLY document recovery trusts
    //      (recovery.go:184–263) — must be a multipart/signed S/MIME
    //      message whose clear-text payload decodes as apiimages.Update
    //      with Version == version, and whose Files list contains every
    //      entry of selected with an equal Filename and Sha256. This is
    //      structural consistency validation, not signature authentication
    //      (the production CA is not in this checkout); the booted system
    //      remains the trust boundary. It catches the failure that matters:
    //      a stale, truncated, or HTML-error sjson that would otherwise
    //      yield a build that only fails at boot.
    //  (c) update.json, the unsigned twin, must decode as apiimages.Update
    //      with the same Version.
    // Bytes are returned VERBATIM — recovery verifies the signature over
    // the exact bytes; we never re-serialize.
    ReleaseMetadata(ctx context.Context, version string, selected []apiimages.UpdateFile) (ReleaseMetadata, error)
}

// VerifiedAsset is a handle to one verified asset retained by the source.
// Open may be called any number of times over the handle's lifetime; each
// call returns a fresh reader over the same verified bytes (compressed,
// exactly as served), and the caller that opened a reader closes it. The
// build opens the OS image twice — a short read for the GPT probe, then
// the full splice — and the rescue writer opens each application once.
// Handles stay valid until the process exits: the v1 cache has no
// eviction, so nothing invalidates a handle mid-build.
type VerifiedAsset interface {
    Open(ctx context.Context) (io.ReadCloser, error)
    Size() int64 // exact verified byte count == UpdateFile.Size
}

// ReleaseMetadata carries the verbatim release documents for rescue media.
type ReleaseMetadata struct {
    UpdateJSON  []byte // parsed + version-checked, stored verbatim
    UpdateSJSON []byte // payload structurally validated, stored verbatim
}

// Reporter receives phase and progress events. The update adapter and the
// domain both hold one (same interface, injected at wiring time in main).
type Reporter interface {
    Step(name string)
    Progress(done, total int64)
    Done(name string)
}

// RescueWriter builds RESCUE_DATA media into tmpPath — an exclusive temporary
// file created and owned by the caller (the CLI's output publisher), which
// fsyncs, hashes, and publishes it afterwards. The adapter never chooses
// paths and never learns cache layout: it stages every asset by streaming
// from its VerifiedAsset handle. It refuses an input with empty UpdateSJSON:
// media without update.sjson is silently non-functional on the booted system
// (recovery.go:178–182).
type RescueWriter interface {
    WriteRescue(ctx context.Context, typ ImageType, in RescueInput, tmpPath string) error
}

// RescueInput is everything staged under the media's update/ tree. The two
// metadata documents are explicit typed fields, not generic assets — their
// presence is a compile-visible requirement, not a list convention.
type RescueInput struct {
    Assets      []RescueAsset  // update/<name>.raw.gz entries
    UpdateJSON  []byte         // → update/update.json, verbatim
    UpdateSJSON []byte         // → update/update.sjson, verbatim
}

// RescueAsset names one file inside the media's update/ tree and the
// verified handle its bytes stream from.
type RescueAsset struct {
    RelPath string        // e.g. "update/incus.raw.gz" — validated, see below
    Asset   VerifiedAsset // opened once by the writer, streamed into the media
}
```

*Ownership model, spelled out (this closes the stream-vs-path seam)*: the source owns storage and verification and exposes only reopenable handles; the domain owns orchestration and passes handles around without ever touching the filesystem; the media adapter consumes streams. No port exposes a cache path, so no component can recompute adapter-private paths or smuggle undeclared I/O — and go-diskfs takes readers anyway, so a path would have bought nothing. *Rejected alternative*: putting a typed `DiskPath` in the handle contract — it leaks cache layout into two other packages and forces the media adapter to open foreign paths; streams keep the seam mockable and uniform across the HTTPS and local-directory sources.

**Untrusted-metadata validation** (in `internal/update`, before any URL or filesystem use — the index is fetched data, so violations are `ErrFetch` with tamper-suspicion wording, never a panic or a traversal):

- **Charset allowlist, not blocklist**: `UpdateFull.Version` must be non-empty and match `[A-Za-z0-9._-]+` with `.`/`..` rejected (observed upstream: `IncusOS_202508141200`). `UpdateFile.Filename` must be non-empty and relative; it may contain `/`-separated segments (upstream `buildImage` handles subdirectories), but **each segment** must independently pass the same allowlist with `.`/`..` and empty segments rejected. The allowlist excludes `?`, `#`, `%`, backslashes, and control bytes by construction — no URL-delimiter or percent-encoding ambiguity survives to URL or path use.
- **URLs are built with `net/url`**, never string concatenation: `url.JoinPath(server, version, filename)` escapes and normalizes each segment — belt and braces on top of the allowlist.
- `UpdateFile.Sha256`: exactly 64 lowercase hex characters — checked *before* it is ever used as a cache path component.
- **Caps and size sanity**: the index body is read through an `io.LimitReader` at 64 MiB (anything larger is corrupt or hostile — update/file counts are bounded transitively). `UpdateFile.Size` must satisfy `0 < Size ≤ 8 GiB`.
- **Exact byte-count enforcement**: every download streams through a counting, hashing reader; admission requires *exactly* `Size` bytes followed by EOF **and** a digest equal to `Sha256`. Short reads, trailing bytes, and hash mismatches all fail admission identically.
- The same allowlist rules gate the local-directory source before joining `<dir>/<version>/<filename>`, and rescue `RelPath` construction.
- **Not trusted for anything else**: free-space and decompressed-size expectations are operational concerns (a best-effort preflight statfs warning), never derived from unsigned metadata.

**Download cache — content-addressed** (v1, boring on purpose): verified assets live at `<cache>/sha256/<digest>`, keyed by the validated `UpdateFile.Sha256` — *never* by server, version, or filename. Admission: download to an `os.CreateTemp` file in the cache root, count and hash during the copy, rename to `sha256/<digest>` only when both the byte count and the hash match. Reuse: an existing entry is re-hashed and size-checked before a handle is issued; `VerifiedAsset.Open` then simply opens the immutable entry. Content addressing makes cross-server races structurally impossible: two builds against different mirrors can only collide on a name when they hold byte-identical content, so replace-between-check-and-open is harmless by definition. The local-directory source verifies the mirror file the same way at `Asset()` time and issues a handle onto it directly — verification at handle creation is the guarantee for both sources. Version/filename are logical metadata only (URL construction, display, rescue RelPath). Release metadata (`update.json`/`update.sjson`) is **not cached at all** — the files are small, offline builds are rare, and per-build fetch removes any need to namespace unhashed data by source identity. Bounded retries (3 attempts, short fixed backoff, ctx-cancellable) on transient failures (E3); a checksum mismatch after a clean re-download fails fast with tamper-suspicion wording. No HTTP Range resume, no cross-process locking, no eviction: unique temp names + atomic rename mean concurrent builds can duplicate work but never corrupt, and no eviction is what lets handles promise process-lifetime validity. Resume is a follow-up if multi-GB retries hurt in practice.

**Error ownership** (deterministic exit-code attribution, every artifact path covered):

- `internal/update` wraps every acquisition failure — index fetch, metadata validation, download, cache read, checksum/size mismatch, handle open, release-metadata fetch/structural validation — in `update.ErrFetch` (exit 5).
- The splice loop in `internal/build` does **not** use bare `io.Copy` between the two ports: it reads and writes explicitly (single reused buffer, P2), wrapping read-side errors as-is (they already carry `ErrFetch`) and write-side errors in `build.ErrOutput`. Attribution is structural, never guessed after the fact. The GPT-probe failures (unreadable/absent table, seed partition not at the splice offset) are read-side: `ErrFetch` — the acquired image is not what we expect. An oversized seed tar is the *user's* input: `config.ErrConfig` (exit 3), reporting actual tar size vs. partition capacity.
- `internal/media` wraps **all** of its failures — handle streams, filesystem/image construction, writes to `tmpPath` — in `build.ErrOutput` (exit 6). The second artifact's failure mapping is as deterministic as the first's.
- `internal/cli` owns the publisher (temps, fsync, hash, claim, ordered rename, rollback, backup restore) and wraps its own failures in `build.ErrOutput`; it maps sentinels to exit codes and nothing else.
- Sentinels (E1): `config.ErrConfig`, `config.ErrDecrypt`, `update.ErrFetch`, `build.ErrOutput`, `build.ErrVersionNotFound` (maps to 5, with nearby available versions listed from the index).

**Acquisition trust model** (v1, deliberately simple):

- `--server` accepts an `https://` URL (default `https://images.linuxcontainers.org/os`) or a local mirror directory (the customizer's own deployment model). Plain `http://` is rejected — mirroring upstream, which forbids unsigned metadata over HTTP (`provider_images.go:428–434`).
- We fetch `/index.json` over HTTPS and verify every asset against `UpdateFile.Sha256` and `Size`. This is exactly upstream's sanctioned fallback path, not an invention.
- We cannot do better unilaterally: upstream's S/MIME verification uses an unimportable `internal/util` helper that shells out to `openssl smime`, anchored to CA PEMs that are injected during upstream CI and absent from the source tree. Signed-index support (`--update-ca <pem>` + `smallstep/pkcs7`, already an upstream dependency) is a follow-up (§10), and requires the user to supply the CA.
- Defense in depth for offline media: `update.sjson` is copied verbatim into the rescue media, and the booted system independently re-verifies its signature before applying anything (`recovery.go:184–207`). A compromised index cannot smuggle unsigned applications onto an installed system — and our structural payload validation (version + selected filename/hash binding) plus size caps bound what a compromised server can even place on the media, while catching honest mistakes (stale mirror, error pages) at build time.

## 7. Key flows

**Build happy path (offline shown; online skips the rescue lane)**

```mermaid
sequenceDiagram
    participant CLI as cli (publisher)
    participant CFG as config
    participant B as build (core)
    participant SRC as update (ImageSource)
    participant SEED as seed
    participant MEDIA as media (RescueWriter)

    CLI->>CFG: Load(path) → SOPS detect/decrypt → validate
    CFG-->>CLI: build.Spec
    CLI->>CLI: validate distinct outputs; create unique temps (hashing writer on image temp)
    CLI->>B: Build(ctx, spec, src, rescue, reporter, out io.Writer, resourcesTmp string)
    B->>SRC: Index(ctx)
    B->>B: Resolve(spec, index) → Plan{version, image asset, app assets}
    B->>SRC: Asset(image) → VerifiedAsset      %% verify + cache; adapter reports progress
    B->>B: handle.Open → gunzip head → parse GPT → assert seed-data
    B->>SEED: Render(spec.Seeds) → tar bytes   %% reject tar > partition length
    B->>B: handle.Open → gunzip → copy 2,148,532,224 bytes → out
    B->>B: write tar → skip len(tar) in source → copy remainder → out
    B->>SRC: Asset(app raw.gz) ×N → handles          %% offline only
    B->>SRC: ReleaseMetadata(version, app files)     %% offline only; structural sjson check
    B->>MEDIA: WriteRescue(type, RescueInput{handles, update.json, update.sjson}, resourcesTmp)
    B-->>CLI: Result{version, seedSize}
    CLI->>CLI: fsync temps; hash resources temp by re-read
    CLI->>CLI: publish: [move old pair aside if --force] → resources → image; restore on failure
    CLI-->>CLI: print summary or JSON envelope (digests from publisher)
```

**Seed-partition invariant (pre-splice, always)**: before any output is written, the build opens the image handle for a short read — enough decompressed bytes to cover the GPT header and 128 partition entries — and locates the partition named `seed-data`. Two checks, both from the *actual* acquired image rather than hard-coded constants: (1) the partition's first byte must equal the upstream splice offset 2,148,532,224 — layout drift in a future release fails loudly (`ErrFetch`) instead of silently corrupting the neighboring partition; (2) the rendered seed tar must fit within the partition's length (100 MiB in every current layout, `mkosi.repart/01-seed-data.conf`, `convert-img-to-iso.sh:37–38`) — an oversized tar (e.g. a huge `incus` preseed) is `ErrConfig` with actual size vs. capacity. The tar is rendered once, up front, into a buffer capped at the partition length (config-derived, tiny in practice; the cap bounds the pathological case), then reused verbatim by the splice. Implementation nuance: raw images use 512-byte logical sectors and ISOs 2048 (`convert-img-to-iso.sh` rebuilds the GPT under `losetup --sector-size 2048`), so the probe tries the header at both offsets and validates the `EFI PART` signature; go-diskfs (already a dependency for §media) does the parsing.

Splice details: decompress with `klauspost/pgzip` (P1 — parallel inflate on the multi-GB critical path); no `gzran` needed since we only move forward, and the probe's second `Open` restarts the stream cleanly. `-o file.iso` writes decompressed; `-o file.iso.gz` recompresses via pgzip; `-o -` streams to stdout (online builds only). Version resolution replicates `filterAssets`: channel membership, exact `release` pin or highest version, exactly-one-image invariant.

Rescue lane details (replicating `sendRescueImage`/`buildImage`): application assets are matched by filename `<name>.raw.gz` against the resolved update's file list; a missing application is exit 5 naming the applications the update *does* carry. Verified assets stream from their handles verbatim (compressed, as upstream does) into the media's `update/` tree, alongside `update/update.json` and `update/update.sjson` from `ReleaseMetadata` — required, not optional, because recovery does nothing without the signed metadata (`recovery.go:178–182`). `internal/media` then builds either iso9660 (volume label `RESCUE_DATA`, Rock Ridge + Joliet) or a GPT image with one `RESCUE_DATA` partition holding FAT32 at the 1 MiB offset — pure Go via `go-diskfs`, no `mkisofs`/`sgdisk`/`mcopy` host dependency. Acceptance is behavioral, not tool-identical: osd mounts by label and filesystem type only (`recovery.go:31–48`), which is what the §10 boot spike proves.

## 8. UX layer

All three Charm packages share one Lip Gloss palette defined in `internal/ux` (per the charmbracelet skill's integration map).

- **Log** (`charm.land/log/v2`): the only logger, on stderr; level via `--verbose`/`-q`; styling follows `--color`.
- **Lip Gloss** (`charm.land/lipgloss/v2`): styled step headers, the single-line progress redraw on stderr (no Bubble Tea), the final summary block, and the `versions` table.
- **Huh** (`charm.land/huh/v2`): `init`'s guided form and `build`'s overwrite confirm (covering both outputs) — both exist only when input is allowed under §3's interaction policy. Huh's accessible mode is enabled when `ACCESSIBLE` is set.
- Renderer selection is pure wiring: `ux.New(colorMode, progressMode, stderr)` returns the fancy or plain `Reporter`; `--json` and `-o -` get the plain reporter with the summary suppressed by `cli`. The `Reporter` port keeps all of this out of the domain and out of the update adapter (which sees only the interface).

## 9. Testing strategy

**Port/adapter/mock inventory** (T2/T3 — every mock mockery-generated, living under its adapter's primary package):

| Port (in `internal/build`) | Adapter | Mockery output | Adapter contract test |
|---|---|---|---|
| `ImageSource` + `VerifiedAsset` | `internal/update` | `internal/update/mocks` | against `httptest` server: index parse, https-only enforcement, retry-on-5xx, local-dir source; **metadata validation** (allowlist rejections incl. `?`/`#`/`%`/traversal-shaped names as `ErrFetch` *before any request*, URL building via `url.JoinPath`, index 64 MiB cap, `Size` range check); **admission** (exact byte count + sha256 both required; short stream, trailing bytes, and hash mismatch each rejected); **content-addressed cache** (rename only on full match; concurrent admission of different content under different digests; reuse re-hash + size check); **handle contract** (Open twice yields identical bytes; Size matches); **`ReleaseMetadata`** (both files fetched from `/<version>/`, size cap enforced; sjson must be multipart/signed with payload Version == version and Files covering every selected filename+sha256 — stale/HTML/mismatched payloads rejected; `update.json` version mismatch rejected; missing/empty `update.sjson` rejected) |
| `Reporter` | `internal/ux` | `internal/ux/mocks` | renderers against a capture buffer: plain output shape, color/progress mode matrix |
| `RescueWriter` | `internal/media` | `internal/media/mocks` | write media into a caller-created temp from fake `VerifiedAsset` handles, read it back with go-diskfs: label, fs type, `update/` tree contents **including `update.json` and `update.sjson` bytes verbatim**; empty `UpdateSJSON` refused |

`internal/config` and `internal/seed` are not domain ports — config produces a `build.Spec` value before the domain runs and seed is a pure function — so they carry no mocks (an empty `mocks/` package would be noise, not compliance). They are covered by unit tests and by the CLI `testscript` suite end-to-end.

- **T1 — unit (pure)**: `internal/seed` golden test — a fully-populated **nine-section** `ImagesPostSeeds` must produce a tar byte-identical to upstream `writeSeed` (same yaml/v4 `WithV2Defaults` serialization, entry names, ordering, mode 0600). `kernel`/`security` entries are goldened separately against **osd-reader semantics** (filename + extension conventions from `internal/seed/seed.go`, strict-decode round-trip into `apiseed` types) since upstream `writeSeed` never emits them. Config validation tables (SOPS probe incl. `sops`-key-present-but-malformed → exit 4, strict-decode rejections, recovery-key rejection, offline cross-rules); `build.Resolve` against synthetic indexes (channel filtering, pinning, highest-version, exactly-one-image, application-asset matching); GPT probe against synthetic 512- and 2048-sector tables (seed-data found, wrong-offset rejected, absent-partition rejected); tar-size-vs-partition-capacity rejection; splice arithmetic with a test-injected offset.
- **T2 — integration (mock adapters)**: `build.Build` wired with `mocks.ImageSource` + `mocks.RescueWriter` + `mocks.Reporter` over a small synthetic gzip image with a real GPT: assert output = original bytes up to offset, exact seed tar, original bytes after offset+seedSize; **the test asserts the image handle is opened twice (probe, then splice) and each rescue handle once — the reopen contract is exercised, not assumed**; the offline run asserts the exact `RescueInput` handed to the writer — the test fails if `UpdateJSON`/`UpdateSJSON` are absent or belong to a different release than the resolved version. Publisher tests in `internal/cli`: distinct-path rejection, unique-temp concurrency, claim-based no-clobber (a file appearing after the pre-work check is refused, exit 6; two concurrent publishers — exactly one wins), forced replacement (old pair moved aside before any publish; restore order resources-then-image on failed image publish; old pair intact after any single injected failure), temp/claim cleanup on every failure path. CLI-level `testscript` runs (repo convention) using the local-directory source: exit codes 2–6, JSON envelope stability (success and error), `-o -` exclusivity rules, `--no-input` behavior, stdin config, SOPS fixture with a checked-in throwaway age key.
- **T3 — e2e (live, opt-in)**: env-gated (`INCUSOS_BUILDER_E2E=1`) against the real update server: `versions` parses the live index; a full `build` of the smallest image completes; the harness dd-extracts the region at 2,148,532,224, untars it, and diffs each YAML section against the input spec. An offline build's media is loop-inspected for label, filesystem, and file set including both metadata files. Testcontainers doesn't apply; the live update server *is* the external service.

**Acceptance gate** for v1: boot a built ISO in an Incus VM and confirm `incus-osd` consumes the seed (install target + network applied); boot with the rescue media attached and confirm the recovery path detects `RESCUE_DATA` and accepts the signed update metadata.

## 10. Open questions / prototype targets

Things a spike answers faster than more design — in priority order:

1. **Pure-Go rescue media acceptance.** go-diskfs iso9660/FAT32 output differs byte-wise from `mkisofs`/`mkfs.vfat`. osd only needs a mountable labeled filesystem, so this should be fine — the boot spike (acceptance gate) proves it. Fallback if go-diskfs ISO support disappoints: FAT32 raw only + documented `mkisofs` passthrough, but only if the spike fails.
2. **Release-metadata size cap.** The cap on `update.json`/`update.sjson` reads needs a number; pick it from observed live sizes (expected well under 1 MiB; a generous 8 MiB default until measured). The structural validation itself (§6) is settled design, not part of this question.
3. **Signed index (`--update-ca`).** Reimplement S/MIME verification with `smallstep/pkcs7` (upstream's own dependency) against a user-supplied CA; the production CA is not redistributable from this checkout. Ship when someone needs more than HTTPS + sha256 + structural checks.
4. **Live server layout quirks.** `UpdateFull.URL` (`api/images/update.go`) suggests per-update base-URL overrides the mirror flow never exercises. *Spike*: pull the real index, confirm asset URL construction — including that `/<version>/update.json` and `/<version>/update.sjson` are served where `buildImage`'s mirror layout implies — and confirm the live sjson payload parses under our structural validator.
5. **First-boot validation loop.** How fast can CI boot a built image in an Incus VM? Determines whether the acceptance gate is CI-automatable or a release-checklist item.
6. **Upstream type churn.** Decide the pin-bump cadence; the golden tar test plus the `go list -deps` type-only gate are the drift alarms. Follow-ups also parked here: update-tarball artifact, HTTP Range resume, cache eviction (would require revisiting the handle-lifetime promise in §6).

(The former "seed partition capacity" question is closed: it is now the pre-splice GPT invariant in §7, checked against the acquired image's real partition table.)

## Revision notes

Dispositions for review-3's numbered issues; every upstream claim was re-verified in `reference/incus-os/` before acting.

1. **Fixed (blocker).** Verified the premise: upstream stages by path only because `buildImage` constructs both `filepath.Join(os.Args[1], version, asset)` and the staging path itself (`main.go:912–954`) — information our ports had abstracted away, leaving `RescueAsset.DiskPath` unproducible without undeclared I/O. Resolution per the reviewer's preferred option: `ImageSource.Asset` now returns a `VerifiedAsset` handle (reopenable `Open(ctx)` + `Size()`); `RescueAsset` carries the handle instead of a path, and `RescueWriter` stages by streaming. No port exposes a cache path at all (go-diskfs consumes readers, so a path bought nothing); open/close ownership and handle lifetime (process-lifetime, guaranteed by the no-eviction cache) are specified in the port docs, and the T2 integration test asserts the open counts — probe + splice on the image handle, one open per rescue asset (§§6, 7, 9).
2. **Fixed (major).** Publication is no longer check-then-rename. Default path: atomic **claim-then-rename** (`O_CREAT|O_EXCL` claim, then rename over our own claim) — no-clobber is enforced at publication time on every platform including FAT targets, so a file appearing after the pre-work check is refused (exit 6), and concurrent non-force builds cannot overwrite each other (`os.Link` and `renameat2(RENAME_NOREPLACE)` documented as rejected alternatives). Forced path: the old image (commit marker) is moved aside *first*, then old resources, both to deterministic `.incusos-builder.bak` names; the new pair publishes resources-then-image; backups are restored in reverse (image last) on any handled failure, so the old pair is never destroyed by a partial replacement. Crash guarantees are explicitly scoped to what the ordering provides: a crash can never leave an old image paired with new resources, and the image path is either absent (unmistakably incomplete, `.bak` recovery documented) or complete-and-consistent. No transaction journal — that would be machinery v1 doesn't need (§3, tested in §9).
3. **Fixed (folded in as an invariant).** Verified: `seed-data` is fixed at 100 MiB in both layouts (`mkosi.repart/01-seed-data.conf:2–5`; `convert-img-to-iso.sh:37–38` — 2 GiB ESP + 100 MiB seed under 2048-byte sectors), and the customizer splices at 2,148,532,224 with no size bound. The former open question is closed: §7 now specifies a pre-splice GPT probe (via the first handle open) that asserts the seed-data partition starts at the splice offset (`ErrFetch` on layout drift — catches future pinned-image changes better than a hard-coded 100 MiB) and rejects a seed tar larger than the partition's actual length (`ErrConfig`, actual vs. capacity) before any output is written. Findings recorded in §2, tests in §9, question removed from §10.
4. **Fixed (folded in).** Verified: recovery derives version and the admissible file/hash list exclusively from the S/MIME-verified `update.sjson` payload and never reads `update.json` (`recovery.go:176–263`); the document is `multipart/signed` (`ssl.go:161`, format asserted in `RunSignedScript`), so its clear-text payload is extractable without key material. `ReleaseMetadata` now takes the plan's selected files and performs structural validation of the *trusted* document: payload must decode as `apiimages.Update`, `Version` must equal the resolved release, and `Files` must cover every selected application's filename **and** sha256. Explicitly labeled structural consistency validation, not signature authentication — boot remains the trust boundary; a stale/HTML/mismatched sjson now fails at build time instead of producing media that dies at boot (§6, tested in §9).
5. **Fixed (folded in).** Verified `UpdateFile` supplies both `Size` and `Sha256` (`update_file.go:3–10`). The boundary is now closed end-to-end: charset **allowlist** (`[A-Za-z0-9._-]`, per-segment for filenames) that excludes `?`/`#`/`%`/backslashes/control bytes by construction; URLs built with `url.JoinPath`, never concatenation; index read capped at 64 MiB (bounding entry counts transitively); `0 < Size ≤ 8 GiB` sanity; cache admission requires **exactly** `Size` bytes and a matching digest, and reuse re-checks both. Free-space/decompressed-size handling is explicitly an operational preflight warning, never trusted from unsigned metadata (§6, tested in §9).
