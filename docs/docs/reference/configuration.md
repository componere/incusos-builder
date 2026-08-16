---
title: Configuration reference
description: Seed config document accepted by incusos-builder
---

# Configuration reference

incusos-builder reads one YAML **seed config**. After optional SOPS
decryption it strict-decodes the document, applies defaults, and
validates it into a build specification. Present `seeds` sections are
rendered into the image **seed-data partition** as one YAML file each.

This page is the lookup for that document. Command flags live in the
[CLI reference](cli.md). SOPS key setup lives in
[How to encrypt a seed config with SOPS](../how-to/sops-encryption.md).
Offline **rescue media** lives in
[How to build offline media](../how-to/build-offline-media.md).

## Schema pin

Nested seed types are the structs in
`github.com/lxc/incus-os/incus-osd/api/seed` at module version
`v0.0.0-20260815030500-0f5b8057f2fc`. The nine web-customizer keys
match `customizer.ImagesPostSeeds`. `seeds.kernel` and `seeds.security`
are CLI-only extensions; the web customizer type has no fields for them.

incusos-builder accepts only document `version: 1`. A different
`version` is rejected as an unsupported schema that requires a newer
CLI.

## Document

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
seeds: {}
```

| Key | Type | Required | Default |
|---|---|---|---|
| `version` | integer | yes | none |
| `image` | mapping | yes | empty mapping (fails validation) |
| `seeds` | mapping | no | all sections absent |
| `sops` | mapping or scalar | no | absent |

`sops` is not a schema field. A top-level `sops` key selects
in-memory decryption before decode. After a successful decrypt the
plaintext is the document above.

Any other top-level key is an unknown field.

## Load pipeline

1. Read the file, or stdin when the path is `-`. A read failure is an
   invalid config.
2. Decode the raw bytes as a YAML mapping. A parse failure here is an
   invalid config. Quoted literals in that diagnostic are replaced with
   `<value>`.
3. If the mapping has a top-level `sops` key, decrypt with
   `sops/v3/decrypt.Data` in YAML format. Presence alone selects this
   path; the `sops` block is not inspected. Every failure after
   selection is a decryption failure, including malformed metadata, no
   matching key, a MAC mismatch, and a stray `sops` key on otherwise
   plain YAML.
4. Strict-decode the plaintext into the document with
   `yaml.WithKnownFields()`.
5. Require `version: 1`.
6. Apply defaults.
7. Validate.

Decrypted bytes are not written to the filesystem.

## Strict decode

Unknown keys are rejected. The diagnostic is:

```text
<dotted.yaml.path>: unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this
```

Examples from the validation matrix:

| Rejected key | Path in the error |
|---|---|
| top-level `mystery` | `mystery` |
| `image.flavor` | `image.flavor` |

Quoted YAML literals are stripped from decode diagnostics so secret
values do not appear.

`seeds.kernel` decodes as `apiseed.Kernel`, which has only `console`
and `version`. Fields that exist on upstream `SystemKernelConfig` but
not on that seed type (`blacklist_modules`, `memory`, `network`,
`pci`) are unknown fields under `seeds.kernel`.

## Defaults

Applied after a successful decode, before validation.

| Condition | Effect |
|---|---|
| `image.channel` is empty | `image.channel` becomes `stable` |
| A present seed section omits `version` or sets it to `""` | that section's `version` becomes `"1"` |
| `image.offline` is `true` and `seeds.update` is absent | `seeds.update` is created with `version: "1"` |
| `image.offline` is `true` | `seeds.update.check_frequency` is set to `never` |

Absent seed sections stay absent. Their `version` is not created
except for the offline `update` seed above.

## Validation

Every validation error wraps invalid config and names a field path.

| Path | Rule |
|---|---|
| `version` | must be present |
| `version` | must equal `1` |
| `image.type` | must be `iso` or `raw` |
| `image.architecture` | must be `x86_64` or `aarch64` |
| `seeds.install.target.sort_order` | if `seeds.install.target` is present: empty, `smallest`, or `largest` (comparison is case-insensitive) |
| `seeds.security.encryption_recovery_keys` | must be absent or empty |
| `seeds.applications` | required when `image.offline` is `true`; the `applications` list must contain at least one entry |

`image.channel` and `image.release` are not enum-checked. Channel is
free text. An empty release selects the highest version in the channel
at resolve time.

`seeds.update.check_frequency` is not parsed as a duration here. The
only project rewrite is the offline force to `never`.

## Seed-data size

Render produces an uncompressed tar. At splice time the tar length
must be less than or equal to the acquired image's **seed-data
partition** length. A larger tar is invalid config:

```text
seed tar is <n> bytes, seed-data partition holds <m>
```

Published IncusOS images used with this pin have a seed-data
partition that starts at byte `2148532224` and is `104857600` bytes
(100 MiB). Drift of that start offset is an acquisition failure, not
a config error.

Nil seed sections are omitted from the tar. Empty `seeds` yields a
valid zero-entry tar.

## Errors

| Sentinel | Exit | When |
|---|---|---|
| invalid config | 3 | read failure, YAML parse failure before SOPS selection, strict-decode failure, validation failure, or seed tar larger than the seed-data partition |
| decryption failed | 4 | any failure after a top-level `sops` key is detected |

Decrypt-path failures do not fall through to invalid config.

## `version`

Document schema version.

**Type:** integer
**Required:** yes
**Accepted value:** `1`

Omitted: `version: required`.
Any other integer: `version: unsupported schema version; a newer CLI is required`.

## `image`

Build artifact selector.

| Key | Type | Required | Default |
|---|---|---|---|
| `type` | string | yes | none |
| `architecture` | string | yes | none |
| `channel` | string | no | `stable` |
| `release` | string | no | empty (highest version in `channel`) |
| `offline` | boolean | no | `false` |

### `image.type`

`iso` is the iso9660 installer. `raw` is the raw disk image.

### `image.architecture`

Update-server architecture name: `x86_64` or `aarch64`.

### `image.channel`

Release channel filter. Free text. Omitted or empty becomes `stable`.

### `image.release`

Exact update version pin. Empty selects the highest version in
`image.channel`.

### `image.offline`

When `true`, the build also writes **rescue media** labeled
`RESCUE_DATA`, `seeds.applications` must list at least one
application, and `seeds.update.check_frequency` is forced to `never`.

## `seeds`

Optional mapping of seed sections. Each present (non-null) section is
one tar member. YAML names:

| YAML key | Origin | Tar member |
|---|---|---|
| `applications` | web customizer | `applications.yaml` |
| `incus` | web customizer | `incus.yaml` |
| `install` | web customizer | `install.yaml` |
| `migration-manager` | web customizer | `migration-manager.yaml` |
| `network` | web customizer | `network.yaml` |
| `operations-center` | web customizer | `operations-center.yaml` |
| `provider` | web customizer | `provider.yaml` |
| `services` | web customizer | `services.yaml` |
| `update` | web customizer | `update.yaml` |
| `kernel` | CLI extension | `kernel.yaml` |
| `security` | CLI extension | `security.yaml` |

Tar member order is the web-customizer `writeSeed` order, then the two
CLI extensions:

1. `applications.yaml`
2. `incus.yaml`
3. `operations-center.yaml`
4. `migration-manager.yaml`
5. `install.yaml`
6. `network.yaml`
7. `provider.yaml`
8. `services.yaml`
9. `update.yaml`
10. `kernel.yaml`
11. `security.yaml`

Tar headers set `Name`, mode `0600`, and `Size` only.

Every present section accepts `version` (string). Omitted `version` on
a present section becomes `"1"`.

### `seeds.applications`

Preinstalled applications.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `applications` | list of mappings | required when `image.offline` is `true`; otherwise optional |

Each list entry:

| Key | Type |
|---|---|
| `name` | string |

An offline document with `applications: []` is rejected at
`seeds.applications`.

### `seeds.incus`

Incus init preseed.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `apply_defaults` | boolean | no (`false`) |
| `preseed` | mapping | no |

`preseed` is `github.com/lxc/incus/v7/shared/api.InitPreseed`.
First-level keys:

| Key | Type |
|---|---|
| `config` | string-to-string map (Incus server config) |
| `networks` | list |
| `storage_pools` | list |
| `storage_volumes` | list |
| `profiles` | list |
| `projects` | list |
| `certificates` | list |
| `cluster_groups` | list |
| `cluster` | mapping |

`cluster` first-level keys from `InitClusterPreseed` / `ClusterPut`:
`server_name`, `enabled`, `member_config`, `cluster_address`,
`cluster_certificate`, `server_address`, `cluster_token`,
`cluster_certificate_path`.

Deeper Incus object shapes are the Incus API types, not additional
incusos-builder fields.

### `seeds.install`

Installer seed.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `force_install` | boolean | no (`false`) |
| `force_install_confirmation` | string | no |
| `force_reboot` | boolean | no (`false`) |
| `security` | mapping | no |
| `target` | mapping | no |

`force_install` ignores existing data on the target disk.
`force_install_confirmation` is an optional reinstall confirmation
string. `force_reboot` reboots on completion instead of waiting for
the install media to be removed.

When `target` is omitted, the installer expects a single drive.

#### `seeds.install.security`

Mutually exclusive degraded-security install flags. They apply only
when the named condition is already true.

| Key | Type | Default |
|---|---|---|
| `missing_tpm` | boolean | `false` |
| `missing_secure_boot` | boolean | `false` |

`missing_tpm` allows an swtpm fallback when no physical TPM is
present. `missing_secure_boot` allows boot without Secure Boot checks
when Secure Boot is already disabled.

#### `seeds.install.target`

Target disk selector. Listed filters are combined with logical AND.

| Key | Type | Notes |
|---|---|---|
| `bus` | string | bus type such as `NVME`, `SCSI`, or `USB` (case-insensitive) |
| `id` | string | case-sensitive substring of `/dev/disk/by-id/` |
| `min_size` | string | minimum disk size, such as `100GiB` |
| `max_size` | string | maximum disk size, such as `1TiB` |
| `sort_order` | string | empty, `smallest`, or `largest` (case-insensitive) |

When `sort_order` is set, matching targets are sorted by capacity and
the first is chosen.

### `seeds.migration-manager`

Migration Manager seed. The YAML key is kebab-case.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `trusted_client_certificates` | list of strings | no |
| `apply_defaults` | boolean | no (`false`) |
| `preseed` | mapping | no |

Each `trusted_client_certificates` entry is a PEM-encoded client
certificate. Its SHA-256 fingerprint is added to any fingerprints in
`preseed.system_security`.

`preseed` first-level keys:

| Key | Type |
|---|---|
| `system_certificate` | mapping |
| `system_network` | mapping |
| `system_security` | mapping |

Those mappings are
`github.com/FuturFusion/migration-manager/shared/api` types.

### `seeds.network`

Network seed. Fields of `api.SystemNetworkConfig` are inlined next to
`version`.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `confirmation_timeout` | string | no |
| `dns` | mapping | no |
| `time` | mapping | no |
| `proxy` | mapping | no |
| `interfaces` | list | no |
| `bonds` | list | no |
| `vlans` | list | no |
| `wireguard` | list | no |

`confirmation_timeout` is a duration string. When set, new network
changes roll back unless confirmed before it elapses.

#### `seeds.network.dns`

| Key | Type |
|---|---|
| `domain` | string |
| `hostname` | string |
| `nameservers` | list of strings |
| `search_domains` | list of strings |
| `dns_over_tls` | boolean |

#### `seeds.network.time`

| Key | Type |
|---|---|
| `ntp_servers` | list of strings |
| `timezone` | string |

#### `seeds.network.proxy`

| Key | Type |
|---|---|
| `rules` | list of `{destination, target}` |
| `servers` | map of name to server |

Each proxy server:

| Key | Type |
|---|---|
| `auth` | string |
| `host` | string |
| `password` | string |
| `realm` | string |
| `username` | string |
| `use_tls` | boolean |

#### `seeds.network.interfaces[]`

| Key | Type |
|---|---|
| `name` | string |
| `hwaddr` | string |
| `addresses` | list of strings |
| `mtu` | integer |
| `lldp` | boolean |
| `roles` | list of strings |
| `routes` | list of `{to, via}` |
| `vlan_tags` | list of integers |
| `required_for_online` | string |
| `strict_hwaddr` | boolean |
| `ethernet` | mapping |
| `firewall_rules` | list |

Documented interface roles: `management`, `cluster`, `instances`,
`storage`.

#### `seeds.network.bonds[]`

Same addressing, role, route, VLAN-tag, ethernet, and firewall fields
as an interface, plus:

| Key | Type |
|---|---|
| `mode` | string |
| `members` | list of strings |

`hwaddr` is optional on a bond.

#### `seeds.network.vlans[]`

| Key | Type |
|---|---|
| `name` | string |
| `id` | integer |
| `parent` | string |
| `addresses` | list of strings |
| `mtu` | integer |
| `roles` | list of strings |
| `routes` | list of `{to, via}` |
| `required_for_online` | string |
| `firewall_rules` | list |

#### `seeds.network.wireguard[]`

| Key | Type |
|---|---|
| `name` | string |
| `addresses` | list of strings |
| `mtu` | integer |
| `port` | integer |
| `private_key` | string |
| `roles` | list of strings |
| `routes` | list of `{to, via}` |
| `required_for_online` | string |
| `firewall_rules` | list |
| `peers` | list |

Each peer:

| Key | Type |
|---|---|
| `public_key` | string |
| `allowed_ips` | list of strings |
| `endpoint` | string |
| `persistent_keepalive` | integer |
| `preshared_key` | string |

#### Shared network nested objects

`ethernet`:

| Key | Type |
|---|---|
| `disable_energy_efficient` | boolean |
| `disable_gro` | boolean |
| `disable_gso` | boolean |
| `disable_ipv4_tso` | boolean |
| `disable_ipv6_tso` | boolean |
| `wakeonlan` | boolean |
| `wakeonlan_modes` | list of strings |
| `wakeonlan_password` | string |

`firewall_rules[]`:

| Key | Type |
|---|---|
| `action` | string |
| `source` | string |
| `protocol` | string |
| `port` | integer |

`routes[]`:

| Key | Type |
|---|---|
| `to` | string |
| `via` | string |

### `seeds.operations-center`

Operations Center seed. The YAML key is kebab-case.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `trusted_client_certificates` | list of strings | no |
| `apply_defaults` | boolean | no (`false`) |
| `preseed` | mapping | no |

PEM trusted-client certificates are handled as in
`seeds.migration-manager`.

`preseed` first-level keys:

| Key | Type |
|---|---|
| `system_certificate` | mapping |
| `system_network` | mapping |
| `system_security` | mapping |
| `system_updates` | mapping |
| `system_settings` | mapping |

Those mappings are
`github.com/FuturFusion/operations-center/shared/api/system` types.

### `seeds.provider`

Update and configuration provider. Fields of
`api.SystemProviderConfig` are inlined next to `version`.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `name` | string | no |
| `config` | string-to-string map | no |

### `seeds.services`

Auxiliary service toggles.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `iscsi` | mapping | no |
| `lvm` | mapping | no |
| `multipath` | mapping | no |
| `netbird` | mapping | no |
| `nvme` | mapping | no |
| `ovn` | mapping | no |
| `tailscale` | mapping | no |
| `usbip` | mapping | no |

#### `seeds.services.iscsi`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `targets` | list of `{target, address, port}` |

#### `seeds.services.lvm`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `system_id` | integer |

#### `seeds.services.multipath`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `wwns` | list of strings |

#### `seeds.services.netbird`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `setup_key` | string |
| `management_url` | string |
| `admin_url` | string |
| `anonymize` | boolean |
| `block_inbound` | boolean |
| `block_lan_access` | boolean |
| `disable_client_routes` | boolean |
| `disable_server_routes` | boolean |
| `disable_dns` | boolean |
| `disable_firewall` | boolean |
| `dns_resolver_address` | string |
| `external_ip_map` | list of strings |
| `extra_dns_labels` | list of strings |

#### `seeds.services.nvme`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `targets` | list |

Each NVMe target:

| Key | Type |
|---|---|
| `transport` | string |
| `address` | string |
| `host_address` | string |
| `port` | integer |
| `nqn` | string |

#### `seeds.services.ovn`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `ic_chassis` | boolean |
| `database` | string |
| `tls_client_certificate` | string |
| `tls_client_key` | string |
| `tls_ca_certificate` | string |
| `tunnel_address` | string |
| `tunnel_protocol` | string |

#### `seeds.services.tailscale`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `login_server` | string |
| `auth_key` | string |
| `accept_routes` | boolean |
| `accept_dns` | boolean |
| `advertised_routes` | list of strings |
| `advertise_exit_node` | boolean |
| `exit_node` | string |
| `exit_node_allow_lan_access` | boolean |
| `serve_enabled` | boolean |
| `serve_port` | integer |
| `serve_service` | string |

#### `seeds.services.usbip`

| Key | Type |
|---|---|
| `enabled` | boolean |
| `targets` | list of `{address, bus_id}` |

### `seeds.update`

Update daemon seed. Fields of `api.SystemUpdateConfig` are inlined
next to `version`.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `auto_reboot` | boolean | no (`false`) |
| `channel` | string | no |
| `check_frequency` | string | no |
| `maintenance_windows` | list | no |

When `image.offline` is `true`, `check_frequency` is `never` after
defaults, even if the document set another value.

Each maintenance window:

| Key | Type |
|---|---|
| `start_day_of_week` | string |
| `start_hour` | integer |
| `start_minute` | integer |
| `end_day_of_week` | string |
| `end_hour` | integer |
| `end_minute` | integer |

Accepted weekday strings on the pin: `Sunday`, `Monday`, `Tuesday`,
`Wednesday`, `Thursday`, `Friday`, `Saturday`. Omitted is empty.

incusos-builder does not run upstream `SystemUpdateConfig.Validate()`
at parse time.

### `seeds.kernel`

CLI extension. Rendered as `kernel.yaml` after the nine customizer
members.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `console` | list | no |

Each console entry:

| Key | Type |
|---|---|
| `device` | string |
| `baud_rate` | integer |

### `seeds.security`

CLI extension. Rendered as `security.yaml` last. Fields of
`api.SystemSecurityConfig` are inlined next to `version`.

| Key | Type | Required |
|---|---|---|
| `version` | string | no (default `"1"`) |
| `custom_ca_certs` | list of strings | no |
| `encryption_recovery_keys` | list of strings | no; must be empty if present |

`custom_ca_certs` entries are PEM certificates.

## `seeds.security.encryption_recovery_keys`

incus-osd rejects a non-empty recovery-key list from the security
seed at boot. incusos-builder refuses the same list at parse time so
the image is not built.

A non-empty list is invalid config at
`seeds.security.encryption_recovery_keys`:

```text
it is not possible to set encryption recovery key(s) via the security seed; see https://linuxcontainers.org/incus-os/docs/main/reference/system/security/
```

The rejected key values are not copied into the error. An omitted
list or an empty list is accepted. Rendered `security.yaml` from the
golden fixture includes `encryption_recovery_keys: []` because the
upstream field is not `omitempty`.

## SOPS

Age keys are supplied through the SOPS process environment, including
`SOPS_AGE_KEY`. An empty `SOPS_AGE_KEY_FILE` is not a usable key source
(it opens path `""`).

| Input | Result |
|---|---|
| no top-level `sops` | plaintext decode |
| valid SOPS document and a matching key | decrypt, then decode the plaintext |
| top-level `sops`, any decrypt failure | decryption failed |
| path `-` | same rules on stdin bytes |

`incusos-builder validate -f` and `incusos-builder build -f` share
this loader. `-f -` reads stdin.

## Examples

Minimal accepted document (`internal/config` iso/`x86_64` case).
Omitted `channel` becomes `stable`.

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
```

Checked-in valid fixture (`internal/config/testdata/valid.yaml`):

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
  channel: stable
seeds:
  applications:
    applications:
      - name: incus
```

Offline document that creates `seeds.update` with
`check_frequency: never`:

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
  offline: true
seeds:
  applications:
    applications:
      - name: incus
```

Every accepted seed section present. Omitted per-section `version`
values become `"1"`:

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  applications:
    applications:
      - name: incus
  install: {}
  incus: {}
  network: {}
  provider: {}
  services: {}
  update: {}
  kernel: {}
  security: {}
  migration-manager: {}
  operations-center: {}
```

CLI kernel extension matching `internal/seed/testdata/kernel.golden.yaml`:

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  kernel:
    console:
      - device: /dev/ttyS0
        baud_rate: 115200
```

CLI security extension with an allowed empty recovery-key list:

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  security:
    custom_ca_certs:
      - |
        -----BEGIN CERTIFICATE-----
        CA
        -----END CERTIFICATE-----
    encryption_recovery_keys: []
```

Populated eleven-section shape used by the live seed round-trip
fixture (architecture and release are placeholders):

```yaml
version: 1
image:
  type: raw
  architecture: aarch64
  channel: stable
  release: "202608102114"
seeds:
  applications:
    version: "1"
    applications:
      - name: incus
      - name: operations-center
  incus:
    version: "1"
    apply_defaults: true
    preseed:
      config:
        core.https_address: ":8443"
  operations-center:
    version: "1"
    apply_defaults: true
    trusted_client_certificates:
      - |
        -----BEGIN CERTIFICATE-----
        OC
        -----END CERTIFICATE-----
  migration-manager:
    version: "1"
    apply_defaults: true
    trusted_client_certificates:
      - |
        -----BEGIN CERTIFICATE-----
        MM
        -----END CERTIFICATE-----
  install:
    version: "1"
    force_install: true
    force_install_confirmation: reinstall-incusos
    force_reboot: true
    security:
      missing_tpm: true
      missing_secure_boot: true
    target:
      bus: NVME
      id: disk-by-id-example
      min_size: 100GiB
      max_size: 2TiB
      sort_order: largest
  network:
    version: "1"
    confirmation_timeout: 30s
    dns:
      domain: example.test
      hostname: incusos
      nameservers:
        - 192.0.2.53
      search_domains:
        - example.test
      dns_over_tls: true
    time:
      ntp_servers:
        - time.cloudflare.com
      timezone: UTC
    interfaces:
      - name: eth0
        hwaddr: 00:16:3e:aa:bb:cc
        addresses:
          - 192.0.2.10/24
        mtu: 1500
        lldp: true
        roles:
          - management
        routes:
          - to: 0.0.0.0/0
            via: 192.0.2.1
  provider:
    version: "1"
    name: images
    config:
      server: https://images.linuxcontainers.org/os
  services:
    version: "1"
    iscsi:
      enabled: true
      targets:
        - target: iqn.2026-08.test:disk
          address: 192.0.2.20
          port: 3260
    lvm:
      enabled: true
      system_id: 1
    multipath:
      enabled: true
      wwns:
        - naa.60060160
    netbird:
      enabled: true
      setup_key: nb-setup
      management_url: https://netbird.example.test
      admin_url: https://netbird.example.test
      external_ip_map:
        - 192.0.2.10
      extra_dns_labels:
        - incusos
    nvme:
      enabled: true
      targets:
        - transport: tcp
          address: 192.0.2.30
          port: 4420
    ovn:
      enabled: true
      database: tcp:192.0.2.40:6641
      tunnel_address: 192.0.2.41
      tunnel_protocol: geneve
    tailscale:
      enabled: true
      login_server: https://login.tailscale.com
      auth_key: tskey-auth
      advertised_routes:
        - 192.0.2.0/24
    usbip:
      enabled: true
      targets:
        - address: 192.0.2.50
          bus_id: "1-1"
  update:
    version: "1"
    auto_reboot: true
    channel: stable
    check_frequency: 6h
    maintenance_windows:
      - start_day_of_week: Saturday
        start_hour: 2
        start_minute: 0
        end_day_of_week: Saturday
        end_hour: 4
        end_minute: 0
  kernel:
    version: "1"
    console:
      - device: /dev/ttyS0
        baud_rate: 115200
  security:
    version: "1"
    custom_ca_certs:
      - |
        -----BEGIN CERTIFICATE-----
        CA
        -----END CERTIFICATE-----
```

Rejected recovery keys (secret does not appear in the error):

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  security:
    encryption_recovery_keys:
      - super-secret-recovery-key
```

## See also

- [CLI reference](cli.md) — `-f` / stdin, `validate`, `build`, exit codes
- [How to encrypt a seed config with SOPS](../how-to/sops-encryption.md)
- [How to build offline media](../how-to/build-offline-media.md)
- [Seed injection](../explanation/seed-injection.md)
- [Upstream version coupling](../explanation/upstream-version-coupling.md)
