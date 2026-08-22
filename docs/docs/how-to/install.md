---
title: How to install incusos-builder
description: Install incusos-builder with a package manager, container image, release archive, or ghd
---

# How to install incusos-builder

Choose an installation channel for your operating system and architecture.

## Install the Homebrew cask

The Homebrew cask supports macOS on Intel (`amd64`) and Apple silicon
(`arm64`):

```bash
brew install --cask componere/tap/incusos-builder
incusos-builder --version
```

## Install with Scoop

The Scoop manifest supports Windows on `amd64` and `arm64`. Add the Componere
bucket, then install the root manifest:

```powershell
scoop bucket add scoop-bucket https://github.com/componere/scoop-bucket
scoop install scoop-bucket/incusos-builder
incusos-builder --version
```

## Install from the APT repository

The stable APT repository supports Linux on `amd64` and `arm64`. Install its
HTTPS prerequisites and aggregate OpenPGP key, then add the repository:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -d -m 0755 /etc/apt/keyrings
curl --fail --silent --show-error --location \
  https://pkgs.componere.dev/keys/apt-repository-001.asc \
  | sudo tee /etc/apt/keyrings/componere-packages.asc >/dev/null
sudo chmod 0644 /etc/apt/keyrings/componere-packages.asc
printf '%s\n' \
  'deb [signed-by=/etc/apt/keyrings/componere-packages.asc] https://pkgs.componere.dev/apt stable main' \
  | sudo tee /etc/apt/sources.list.d/componere-packages.list >/dev/null
sudo apt-get update
sudo apt-get install -y incusos-builder
incusos-builder --version
```

APT reads the stable roots under `https://pkgs.componere.dev/apt/dists/stable/`.

## Install from the DNF/RPM repository

The stable RPM repository supports Linux on `x86_64` (`amd64`) and `aarch64`
(`arm64`). Two keys sign different things: the aggregate key signs the
repository metadata, and the producer key signs each package. Verifying both
requires importing both.

```bash
sudo tee /etc/yum.repos.d/componere-packages.repo >/dev/null <<'EOF'
[componere-packages]
name=Componere packages
baseurl=https://pkgs.componere.dev/rpm/stable/$basearch
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://pkgs.componere.dev/keys/rpm-repository-001.asc
       https://pkgs.componere.dev/keys/incusos-builder-rpm-001.asc
EOF
sudo dnf install -y incusos-builder
incusos-builder --version
```

`repo_gpgcheck=1` verifies `repomd.xml` against
`rpm-repository-001.asc`. `gpgcheck=1` verifies each package against
`incusos-builder-rpm-001.asc`. Dropping the second key from `gpgkey` makes
`dnf install` fail on an untrusted package signature.

The repository paths are
`https://pkgs.componere.dev/rpm/stable/x86_64` and
`https://pkgs.componere.dev/rpm/stable/aarch64`.

## Install from the APK repository

The stable APK repository supports Linux on `x86_64` (`amd64`) and `aarch64`
(`arm64`). As with RPM, two keys sign different things: the aggregate key
signs `APKINDEX.tar.gz` and the producer key signs each package. Install both
under their published filenames, because `apk` matches a package's embedded
signature name against the filenames in `/etc/apk/keys`:

```bash
sudo wget -q \
  https://pkgs.componere.dev/keys/apk-index-001.rsa.pub \
  -O /etc/apk/keys/apk-index-001.rsa.pub
sudo wget -q \
  https://pkgs.componere.dev/keys/incusos-builder-apk-001.rsa.pub \
  -O /etc/apk/keys/incusos-builder-apk-001.rsa.pub
printf '%s\n' 'https://pkgs.componere.dev/apk/stable/main' \
  | sudo tee -a /etc/apk/repositories >/dev/null
sudo apk update
sudo apk add incusos-builder
incusos-builder --version
```

The APK client selects the matching index under
`https://pkgs.componere.dev/apk/stable/main/x86_64` or
`https://pkgs.componere.dev/apk/stable/main/aarch64`. Without
`incusos-builder-apk-001.rsa.pub`, `apk add` rejects the package as untrusted
even though the index verifies.

## Run the container image

The OCI image supports Linux containers on `amd64` and `arm64`. Set `DIGEST`
to the published manifest digest, then pull and run that exact image:

```bash
DIGEST='sha256:<published-manifest-digest>'
IMAGE="ghcr.io/componere/incusos-builder@${DIGEST}"
docker pull "$IMAGE"
docker run --rm "$IMAGE" --version
```

Consumers that need repeatable inputs must pin the digest. Do not replace the
digest with a moving `MAJOR.MINOR`, `MAJOR`, or `latest` tag.

## Install a direct archive download

Release archives cover all six targets:

- `incusos-builder_<version>_darwin_<amd64|arm64>.tar.gz`
- `incusos-builder_<version>_linux_<amd64|arm64>.tar.gz`
- `incusos-builder_<version>_windows_<amd64|arm64>.zip`

On macOS or Linux, set the release version, operating system, and architecture,
then download and verify the matching archive:

```bash
VERSION='x.y.z'
OS='linux'
ARCH='amd64'
ARCHIVE="incusos-builder_${VERSION}_${OS}_${ARCH}.tar.gz"
gh release download "v${VERSION}" \
  --repo componere/incusos-builder \
  --pattern "$ARCHIVE" \
  --pattern checksums.txt \
  --pattern checksums.txt.sigstore.json
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum --check --ignore-missing checksums.txt
else
  shasum -a 256 --check checksums.txt --ignore-missing
fi
tar -xzf "$ARCHIVE"
sudo install -m 0755 incusos-builder /usr/local/bin/incusos-builder
incusos-builder --version
```

Use `OS='darwin'` on macOS. Set `ARCH` to `amd64` or `arm64` as appropriate.

On Windows, download and verify the ZIP archive in PowerShell:

```powershell
$Version = 'x.y.z'
$Arch = 'amd64'
$Archive = "incusos-builder_${Version}_windows_${Arch}.zip"
gh release download "v$Version" `
  --repo componere/incusos-builder `
  --pattern $Archive `
  --pattern checksums.txt `
  --pattern checksums.txt.sigstore.json
$ChecksumLine = Get-Content checksums.txt |
  Where-Object { $_.EndsWith("  $Archive") }
$Expected = ($ChecksumLine -split '\s+')[0]
$Actual = (Get-FileHash -Algorithm SHA256 $Archive).Hash.ToLowerInvariant()
if ($Actual -ne $Expected) { throw 'checksum mismatch' }
Expand-Archive $Archive -DestinationPath incusos-builder
.\incusos-builder\incusos-builder.exe --version
```

Set `$Arch` to `amd64` or `arm64` as appropriate. Add the extracted directory
to `PATH` if you want to run `incusos-builder` outside that directory.

`checksums.txt.sigstore.json` is the keyless Cosign bundle for
`checksums.txt`. Consumers that verify release signatures can use that bundle
to verify the checksum file.

## Install with ghd

`ghd` supports the same macOS, Linux, and Windows `amd64` and `arm64` archive
targets. Install `ghd` by following its
[getting started guide](https://github.com/meigma/ghd/blob/master/docs/docs/getting-started.md),
then install the archive and verify its build provenance attestation:

```bash
ghd install componere/incusos-builder/incusos-builder \
  --store-dir "$HOME/.local/share/ghd/store" \
  --bin-dir "$HOME/.local/bin"
incusos-builder --version
```

Add `$HOME/.local/bin` to `PATH` before the final command if it is not already
present. Pin a release by appending `@x.y.z` to the ghd package identifier.
