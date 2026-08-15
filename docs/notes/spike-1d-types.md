# Spike 1.D — upstream type-import feasibility

Scratch module: `spikes/typeimport/` (`module spike-typeimport`). Root `go.mod` / `go.sum` were not touched. `go list ./...` from the worktree root still reports only `cmd/incusos-builder` and `internal/cli`.

Toolchain: Go 1.26.4 darwin/arm64, `GOTOOLCHAIN=local`. Date of run: 2026-08-15.

## Questions

### Does upstream tag releases? What should Phase 2 pin?

`git ls-remote --tags https://github.com/lxc/incus-os` returned **312** tags. They are date stamps `YYYYMMDDHHMM`, not semver. Newest:

| git tag | commit | notes |
|---|---|---|
| `202608102114` | `7b3c147b4d0df26dd5b894e7498578e6d29cdb5f` | latest tag; subject `Update go.mod`; committer `2026-08-10T17:04:36-04:00` |
| *(no tag)* | `0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12` | `origin/HEAD` as of this spike; merge of `#1289`; committer `2026-08-14T21:05:00-06:00` |

Date tags are usable as git refs. Go rewrites them to pseudo-versions:

```
$ go list -m github.com/lxc/incus-os/incus-osd@202608102114
github.com/lxc/incus-os/incus-osd v0.0.0-20260810210436-7b3c147b4d0d

$ go get github.com/lxc/incus-os/incus-osd@latest
go: added github.com/lxc/incus-os/incus-osd v0.0.0-20260815030500-0f5b8057f2fc
```

proxy.golang.org `@latest`:

```json
{"Version":"v0.0.0-20260815030500-0f5b8057f2fc","Time":"2026-08-15T03:05:00Z","Origin":{"VCS":"git","URL":"https://github.com/lxc/incus-os","Subdir":"incus-osd","Hash":"0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12"}}
```

The latest **tag** is missing types Phase 2 needs. `incus-osd/api/seed/services.go` and `ImagesPostSeeds.Services` exist at HEAD and are absent at `202608102114`:

```
$ git ls-tree --name-only 202608102114:incus-osd/api/seed
applications.go doc.go incus.go install.go kernel.go migration_manager.go
network.go operations_center.go provider.go security.go update.go
# no services.go

$ git diff 7b3c147b4d0df26dd5b894e7498578e6d29cdb5f HEAD -- incus-osd/api/seed/services.go incus-osd/api/customizer/images.go
# services.go is new; ImagesPostSeeds gains a Services field
```

**DECISION:** Phase 2 pins

```
github.com/lxc/incus-os/incus-osd v0.0.0-20260815030500-0f5b8057f2fc
```

- git commit: `0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12`
- module zip sum: `h1:rqaB8S8vjar+9kZDKLGt2Kz1rfVsiu5Sfm9cCF6spDY=`
- `--version` should report that exact module version string (optionally also short hash `0f5b805`). There is no semver tag to print.

Do not pin `202608102114` / `v0.0.0-20260810210436-7b3c147b4d0d`: it cannot represent the `services` seed the architecture includes. Revisit when upstream cuts a date tag after `0f5b805`.

### Is the compiled-in closure type-only?

`go build ./...` in the scratch module succeeded. The binary printed every requested type and the pin:

```
types:
  customizer.ImagesPost
  customizer.ImagesPostSeeds
  images.Index
  images.Update
  images.UpdateFull
  images.UpdateFile
  seed.Applications
  seed.Incus
  seed.Install
  seed.Kernel
  seed.MigrationManager
  seed.Network
  seed.OperationsCenter
  seed.Provider
  seed.Security
  seed.Services
  seed.Update
go: go1.26.4
path: spike-typeimport
incus-osd: version=v0.0.0-20260815030500-0f5b8057f2fc sum=h1:rqaB8S8vjar+9kZDKLGt2Kz1rfVsiu5Sfm9cCF6spDY=
```

Binary size: **5,697,906 bytes**. A same-toolchain `fmt.Println` hello-world is 2,492,466 bytes (~3.2 MiB extra for the type graph).

`go list -deps ./...`: **221** packages total — **194 stdlib**, **27 non-stdlib**.

**incus-osd `internal/` packages compiled in:** none.
**incus-osd `cmd/` / daemon packages compiled in:** none.
**tailscale / tview / tcell / umoci compiled in:** none.

Non-stdlib compiled packages, grouped by module:

| module | version | compiled packages |
|---|---|---|
| `spike-typeimport` | (main) | `spike-typeimport` |
| `github.com/lxc/incus-os/incus-osd` | `v0.0.0-20260815030500-0f5b8057f2fc` | `api`, `api/customizer`, `api/images`, `api/seed` |
| `github.com/lxc/incus/v7` | `v7.3.0` | `shared/api`, `shared/osinfo` |
| `github.com/FuturFusion/operations-center` | `v0.8.1` | `shared/api/system` |
| `github.com/FuturFusion/migration-manager` | `v0.6.15` | `shared/api` |
| `github.com/FuturFusion/openfga-sync` | `v0.0.0-20260802043841-c17052a24795` | `shared/config` |
| `github.com/zitadel/oidc/v3` | `v3.49.2` | `pkg/oidc`, `pkg/crypto` |
| `github.com/zitadel/schema` | `v1.3.2` | `github.com/zitadel/schema` |
| `github.com/go-jose/go-jose/v4` | `v4.1.4` | `.`, `cipher`, `json` |
| `github.com/google/uuid` | `v1.6.0` | `github.com/google/uuid` |
| `github.com/muhlemmer/gu` | `v0.3.1` | `github.com/muhlemmer/gu` |
| `go.yaml.in/yaml/v4` | `v4.0.0-rc.6` | `.`, `internal/libyaml`, `plugin/limit` |
| `golang.org/x/oauth2` | `v0.36.0` | `.`, `internal` |
| `golang.org/x/text` | `v0.40.0` | `language` + `internal/language`, `internal/language/compact`, `internal/tag` |

Plan expectation vs fact:

- `lxc/incus/v7 shared/api` — **confirmed** (plus `shared/osinfo`, 10,802 B of OS/distro enums and string helpers, imported by migration-manager API).
- `operations-center` — **confirmed** (`shared/api/system` only, 8,014 B).
- `migration-manager` — **confirmed** (`shared/api`).
- Extra, not named in the plan: **`openfga-sync/shared/config`**. Forced because `api/seed` imports the parent package `github.com/lxc/incus-os/incus-osd/api`, and Go compiles every file in that package, including `application_openfga.go` (`Sync *syncconfig.Config`). 13,180 B of config structs + YAML/JSON duration codecs; not a daemon.
- Extra chain from migration-manager API types: `zitadel/oidc` → `go-jose`, `x/oauth2`, `x/text/language`, `muhlemmer/gu`, `zitadel/schema`.
- `go.yaml.in/yaml/v4` is already in the type closure (`incus/v7/shared/api` and `openfga-sync/shared/config`). Same library `writeSeed` uses.

Largest compiled third-party sources (this-package `.go` bytes, tests excluded): yaml `internal/libyaml` 377 KiB, `x/text` language tables 258+91 KiB, `incus/v7/shared/api` 204 KiB / 51 files, `go-jose` 141 KiB, `zitadel/oidc/pkg/oidc` 81 KiB, `incus-osd/api` 67 KiB / 26 files.

Why `go list -m all` looks scary: it listed **332** modules (incus-osd's full requirement graph: tailscale, tview, umoci, aws-sdk, openfga server, testcontainers, …). Those are **not compiled** and **not in go.sum**. This spike's `go.sum` has **17** module paths; `go.mod` has **13** requires (1 direct + 12 indirect). Phase 2's type-only CI gate must use `go list -deps`, not `go list -m all`.

### replace directives?

Checked `go.mod` of every compiled-in module: **no `replace` directives** in `incus-osd`, `incus/v7`, `operations-center`, `migration-manager`, `openfga-sync`, or `zitadel/oidc/v3`. The scratch module built against the public proxy with no local replaces.

### Go version compatibility?

`incus-osd` `go.mod` line: `go 1.25.12` (same at tag `202608102114` and at HEAD). Our module is `go 1.26.4`. Build with `GOTOOLCHAIN=local` (1.26.4) succeeded. Compatible.

## Verdict

**PASS** — type-only / type-adjacent closure; safe to import in Phase 2.

Not ESCALATE: nothing from `incus-osd/internal` or `incus-osd/cmd` is compiled in. The extras beyond the three named type surfaces are small API/config packages and the OIDC/JOSE stack that migration-manager's API types already require.

Borderline, named so Phase 2 can write the gate precisely:

| package | why it compiles | acceptable? |
|---|---|---|
| `incus-osd/api` (whole parent package) | `api/seed` imports it; Go has no file-level import | yes — still user-facing structs (`debug_tui.go` uses stdlib `log/slog` only) |
| `openfga-sync/shared/config` | `api/application_openfga.go` | yes — config types, ~13 KiB |
| `incus/v7/shared/osinfo` | migration-manager `shared/api` | yes — enums + string matching, ~11 KiB |
| `zitadel/oidc/v3/pkg/{oidc,crypto}` + `go-jose` + `x/oauth2` | migration-manager OIDC claim types | yes — not a daemon; JOSE/oauth helpers come along with those types. Heaviest extra (jose 141 KiB + oidc 81 KiB). |
| `go.yaml.in/yaml/v4` | incus `shared/api`, openfga-sync config | yes — Phase 2 needs this anyway for seed YAML |

stdlib `net/http` and `crypto/tls` appear in the 194-package stdlib closure via jose/oauth2. That is compile-in, not a runtime client we invoke.

## Phase 2 must consume

1. Pin `github.com/lxc/incus-os/incus-osd v0.0.0-20260815030500-0f5b8057f2fc`. `--version` prints that string.
2. Import `api/seed`, `api/images`, `api/customizer` (and parent `api` as needed). Do not mirror types.
3. CI type-only gate: `go list -deps` of the production packages that import incus-osd, assert:
   - no `github.com/lxc/incus-os/incus-osd/internal/`
   - no `github.com/lxc/incus-os/incus-osd/cmd/`
   - allowlist the 27-ish non-stdlib set above (plus our own packages). Do **not** assert on `go list -m all`.
4. `go.yaml.in/yaml/v4 v4.0.0-rc.6` is already selected by this pin; seed rendering should use this version (with `WithV2Defaults()`) so it cannot drift from the type module.
5. Next pin bump: wait for a date tag **after** `0f5b805`, or deliberately move the pseudo-version. The latest date tag today is schema-behind HEAD.

## Commands

```
cd spikes/typeimport
export GOTOOLCHAIN=local
go get github.com/lxc/incus-os/incus-osd@latest
go mod tidy
go build ./...
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./...
go list -m all | wc -l          # 332 — unused MVS graph; ignore for the gate
go list -m github.com/lxc/incus-os/incus-osd@202608102114
```
