---
title: About upstream version coupling
description: Why incusos-builder pins incus-osd for types only, what the closure gate actually denies, and why a pin bump is a revalidation
---

# About upstream version coupling

incusos-builder speaks IncusOS's languages: seed-config YAML, the update
index, and the GPT layout of the installer. Those languages are defined
by `github.com/lxc/incus-os/incus-osd`. Importing that module is how the
builder avoids a second, drifting copy of the same structs.

Importing it also creates the coupling this page is about. A pin chooses
a schema. A pin bump can change seed fields, YAML dump defaults, and —
if someone imports the wrong package — the linked daemon. The type-only
rule exists so the first two can be deliberate and the third cannot
happen quietly.

## Why a pseudo-version, not the newest date tag

Upstream tags are `YYYYMMDDHHMM` stamps, not semver. Go rewrites a tag
or a commit to a `v0.0.0-YYYYMMDDHHMMSS-<12-hex>` pseudo-version. There
is no release number for `--version` to print other than that string.

The builder is pinned to

```text
github.com/lxc/incus-os/incus-osd v0.0.0-20260815030500-0f5b8057f2fc
```

That is commit `0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12`. The module zip
sum is `h1:rqaB8S8vjar+9kZDKLGt2Kz1rfVsiu5Sfm9cCF6spDY=`.
`--version` reports that exact module version on the `incus-os API:`
line.

The newest *tag* at the time of the type spike was `202608102114`. That
tag has no `api/seed/services.go` and no `ImagesPostSeeds.Services`. The
architecture includes a `services` seed, so pinning the tag would have
made a first-class section unrepresentable. The pin is HEAD-of-that-day
on purpose, and it stays there until someone moves it. Waiting for a
date tag after `0f5b805` is the conservative next step; moving the
pseudo-version without a tag is the deliberate alternative.

`go.yaml.in/yaml/v4 v4.0.0-rc.6` is the YAML library that pin already
selects. Seed rendering uses `yaml.Dump(..., yaml.WithV2Defaults())` so
the dump cannot drift from the type module. V4 defaults change sequence
indent and quoting; they are the wrong compatibility target.

## What "type-only" actually means

The allowed incus-osd surface is `api`, `api/seed`, `api/images`, and
`api/customizer`. Those packages are struct declarations for seed
config, the update index, and the web-customizer request shape. The
builder does not import `incus-osd/internal/...` or `incus-osd/cmd/...`.
Mirroring the structs would duplicate the schema the pin is meant to
own.

"Type-only" describes that incus-osd surface. It does not mean the
binary contains only type declarations. Go compiles every file in an
imported package, and those types pull other modules with them. The
type-import spike's scratch binary compiled 27 non-stdlib packages:
the four incus-osd API packages, `incus/v7` `shared/api` and
`shared/osinfo`, thin API/config packages from operations-center,
migration-manager, and openfga-sync, plus the OIDC/JOSE stack that
migration-manager's API types require. No incus-osd `internal/` or
`cmd/` package was in that closure. No tailscale, tview, or umoci
either.

`go list -m all` listed 332 modules — the full MVS graph of incus-osd,
including daemons this tree never builds. Those modules are not in
`go.sum` and are not compiled. A gate that asserted on `go list -m all`
would be noise. The production gate uses `go list -deps`.

stdlib `net/http` and `crypto/tls` appear in that deps closure because
JOSE and OAuth2 do. They are compile-in, not a runtime update client
the type import invokes.

## The closure gate is a deny list, not a headcount

`moon run root:check-upstream` runs
`.github/scripts/check_upstream_closure.py` over `go list -deps ./...`.
It fails if any package is `github.com/lxc/incus-os/incus-osd/internal`
or `.../cmd`, or a subpackage of either. It prints the incus-osd
packages that *are* present. It does not assert that there are 26 or 27
of them.

An absolute count is the wrong invariant. The spike's 27 included the
scratch `main`. Production additionally compiles SOPS, go-diskfs, pgzip,
Cobra, Viper, and the Charm libraries. Pointing a 27-count at `./...`
fails the moment the builder's own dependencies move, which they should
be free to do.

The deny list is the load-bearing rule: if a future import reaches
recovery, image-customizer, or any other daemon package, the gate fails
before that code is shipped. Near-miss names such as `api/internals` are
not denied. The builder's own `internal/build` is a different module and
is irrelevant to the rule.

Config validation embeds the same pin string in unknown-field errors
(`unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc`). That is
schema coupling made visible, not a second pin.

## Why a pin bump is a revalidation

Moving `incus-osd` — or the yaml/v4 version it selects — changes more
than a `go.mod` line. The things that can break are exactly the things
the pin is there to stabilize:

- **Seed-config schema.** New fields appear; old fields change meaning;
  a section such as `services` can exist at HEAD and not at the previous
  tag. Strict YAML decode will start accepting or rejecting documents.
  The unknown-field hint must name the new pin.
- **Dump and tar layout.** `WithV2Defaults()` plus writeSeed's headers
  are the byte-compat oracle for the nine web sections. Goldens under
  `internal/seed/testdata/` are generated from a vendored writeSeed copy
  at this pin, not from an upstream-built customizer binary. Dump or tar
  churn after a bump is expected and must be inspected, not blindly
  refreshed. Byte-equality versus an upstream-built binary remains
  unproven.
- **Compiled closure.** A new API file that imports `incus-osd/internal`
  would pull the daemon graph into `go list -deps`. The deny gate is the
  check; `go list -m all` is not.
- **Installer layout.** Splice still requires `seed-data` at byte
  `2148532224`. A pin that accompanies a new image layout is not
  finished until a live image is probed. See
  [About seed injection](seed-injection.md).
- **Metadata shape.** Index and sjson decoding ignore unknown fields so
  a new server key does not force a rebuild. Selected-file binding and
  size caps remain the integrity story in
  [About the trust model](trust-model.md). A bump is still the moment to
  look at whether `UpdateFull.URL`, file keys, or the sjson envelope
  changed meaning.

Those are coupled invariants, not a release checklist. The conservative
trigger remains: wait for a date tag after `0f5b805`, or move the
pseudo-version on purpose, then treat seed goldens, the closure deny
list, `--version`, and a GPT probe of the matching installer as the
evidence that the new pin still describes the system the builder talks
to.