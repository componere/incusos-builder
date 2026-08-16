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

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `version` | integer | yes | none | Schema version of this document, so an older CLI refuses a newer layout. |
| `image` | mapping | yes | empty mapping (fails validation) | Which published IncusOS image to acquire and in what form. |
| `seeds` | mapping | no | all sections absent | Seed sections rendered into the image seed-data partition. |
| `sops` | mapping or scalar | no | absent | SOPS metadata left in place by `sops`; its presence selects decryption. |

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

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `type` | string | yes | none | Artifact format: the `iso` installer or a `raw` disk image. |
| `architecture` | string | yes | none | CPU architecture of the image to acquire. |
| `channel` | string | no | `stable` | Update channel the release is resolved from. |
| `release` | string | no | empty (highest version in `channel`) | Exact update version to pin instead of the newest one. |
| `offline` | boolean | no | `false` | Build media that installs and runs without network access. |

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

| YAML key | Origin | Tar member | Configures |
|---|---|---|---|
| `applications` | web customizer | `applications.yaml` | Applications installed once IncusOS is running. |
| `incus` | web customizer | `incus.yaml` | Incus preseed and whether to apply Incus defaults. |
| `install` | web customizer | `install.yaml` | Installer behavior and target-disk selection. |
| `migration-manager` | web customizer | `migration-manager.yaml` | Migration Manager preseed and trusted client certificates. |
| `network` | web customizer | `network.yaml` | Interfaces, bonds, VLANs, WireGuard, DNS, NTP, and proxy. |
| `operations-center` | web customizer | `operations-center.yaml` | Operations Center preseed and trusted client certificates. |
| `provider` | web customizer | `provider.yaml` | Which provider supplies updates and applications. |
| `services` | web customizer | `services.yaml` | Storage and VPN services enabled on first boot. |
| `update` | web customizer | `update.yaml` | Update channel, check frequency, and maintenance windows. |
| `kernel` | CLI extension | `kernel.yaml` | Kernel settings needed before the IncusOS API is reachable. |
| `security` | CLI extension | `security.yaml` | Extra certificate authorities in the system trust store. |

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

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `applications` | list of mappings | required when `image.offline` is `true`; otherwise optional | Applications IncusOS installs after it first boots. |

Each list entry:

| Key | Type | Description |
|---|---|---|
| `name` | string | Application to install, such as `incus` or `operations-center`. |

IncusOS accepts at most one primary application. When the list names no
primary application, upstream appends `incus` to it.

An offline document with `applications: []` is rejected at
`seeds.applications`.

### `seeds.incus`

Incus init preseed.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `apply_defaults` | boolean | no (`false`) | Apply upstream's reasonable Incus defaults while installing Incus. |
| `preseed` | mapping | no | Preseed handed to Incus during install. |

`preseed` is `github.com/lxc/incus/v7/shared/api.InitPreseed`.
First-level keys:

| Key | Type | Description |
|---|---|---|
| `config` | string-to-string map | Incus server configuration keys. |
| `networks` | list | Networks to create, grouped by project. |
| `storage_pools` | list | Storage pools to create. |
| `storage_volumes` | list | Storage volumes to create. |
| `profiles` | list | Profiles to create. |
| `projects` | list | Projects to create. |
| `certificates` | list | Client certificates to trust. |
| `cluster_groups` | list | Cluster groups to create. |
| `cluster` | mapping | Cluster bootstrap or join details. |

`cluster` first-level keys from `InitClusterPreseed` / `ClusterPut`:

| Key | Type | Description |
|---|---|---|
| `server_name` | string | Name this cluster member is known by. |
| `enabled` | boolean | Whether clustering is enabled. |
| `member_config` | list | Member-specific configuration applied while joining. |
| `cluster_address` | string | Address of an existing cluster to join. |
| `cluster_certificate` | string | Expected PEM-encoded certificate of that cluster. |
| `cluster_certificate_path` | string | Path to a file holding that cluster certificate. |
| `cluster_token` | string | Join token issued by the cluster. |
| `server_address` | string | Local address this member uses for cluster traffic. |

Deeper Incus object shapes are the Incus API types, not additional
incusos-builder fields.

### `seeds.install`

Installer seed.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `force_install` | boolean | no (`false`) | Install over existing data on the target disk. This can destroy data. |
| `force_install_confirmation` | string | no | Confirmation value required to overwrite an existing IncusOS install; the installer reports the expected value. |
| `force_reboot` | boolean | no (`false`) | Reboot when the install finishes instead of waiting for the media to be removed. |
| `security` | mapping | no | Opt-ins that let IncusOS install in a degraded security state. |
| `target` | mapping | no | Selector for the install disk when several drives are present. |

When `target` is omitted, the installer expects a single drive.

#### `seeds.install.security`

Mutually exclusive degraded-security install flags. They apply only
when the named condition is already true.

| Key | Type | Default | Description |
|---|---|---|---|
| `missing_tpm` | boolean | `false` | Allow an swtpm fallback, and only when no physical TPM is present. |
| `missing_secure_boot` | boolean | `false` | Allow boot without Secure Boot checks, and only when Secure Boot is already disabled. |

#### `seeds.install.target`

Target disk selector. Listed filters are combined with logical AND.

| Key | Type | Description |
|---|---|---|
| `bus` | string | Match only disks on this bus type, such as `NVME`, `SCSI`, or `USB` (case-insensitive). |
| `id` | string | Case-sensitive substring match against the disk ID in `/dev/disk/by-id/`. |
| `min_size` | string | Reject disks smaller than this size, such as `100GiB`. |
| `max_size` | string | Reject disks larger than this size, such as `1TiB`. |
| `sort_order` | string | Pick the `smallest` or `largest` matching disk (case-insensitive); empty picks the sole match. |

When `sort_order` is set, matching targets are sorted by capacity and
the first is chosen.

### `seeds.migration-manager`

Migration Manager seed. The YAML key is kebab-case.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `trusted_client_certificates` | list of strings | no | PEM client certificates Migration Manager should trust. |
| `apply_defaults` | boolean | no (`false`) | Apply upstream's reasonable Migration Manager defaults while installing it. |
| `preseed` | mapping | no | Preseed handed to Migration Manager during install. |

Each `trusted_client_certificates` entry is a PEM-encoded client
certificate. Its SHA-256 fingerprint is added to any fingerprints in
`preseed.system_security`.

`preseed` first-level keys:

| Key | Type | Description |
|---|---|---|
| `system_certificate` | mapping | Server certificate, private key, and CA. |
| `system_network` | mapping | REST API listen address and worker endpoint. |
| `system_security` | mapping | Trusted clients and proxies, OIDC, OpenFGA, and ACME. |

Those mappings are
`github.com/FuturFusion/migration-manager/shared/api` types.

### `seeds.network`

Network seed. Fields of `api.SystemNetworkConfig` are inlined next to
`version`.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `confirmation_timeout` | string | no | Duration such as `5m`; the configuration rolls back unless it is confirmed before the timeout elapses. |
| `dns` | mapping | no | Host name, domain, resolvers, and DNS over TLS. |
| `time` | mapping | no | NTP servers and system timezone. |
| `proxy` | mapping | no | System-wide HTTP(S) proxy servers and routing rules. |
| `interfaces` | list | no | Physical interfaces to configure. |
| `bonds` | list | no | Link aggregation groups built from interfaces. |
| `vlans` | list | no | VLAN devices layered on an interface or bond. |
| `wireguard` | list | no | WireGuard tunnels. |

#### `seeds.network.dns`

| Key | Type | Description |
|---|---|---|
| `domain` | string | Domain name of the system. |
| `hostname` | string | Host name of the system. |
| `nameservers` | list of strings | Resolvers to use instead of the ones learned from DHCP. |
| `search_domains` | list of strings | Domains appended when resolving short names. |
| `dns_over_tls` | boolean | Resolve over TLS; every listed nameserver must support it. |

#### `seeds.network.time`

| Key | Type | Description |
|---|---|---|
| `ntp_servers` | list of strings | NTP servers used to synchronize the clock. |
| `timezone` | string | System timezone, such as `America/New_York`. |

#### `seeds.network.proxy`

| Key | Type | Description |
|---|---|---|
| `rules` | list of `{destination, target}` | Which destinations go through which server, bypass the proxy, or are blocked. |
| `servers` | map of name to server | Named proxy servers that rules can target. |

Each rule matches `destination` against a `|`-separated glob list and
sends matching traffic to `target`: a server name, `direct` to bypass
the proxy, or `none` to block it.

Each proxy server:

| Key | Type | Description |
|---|---|---|
| `auth` | string | Authentication scheme: `anonymous`, `basic`, or `kerberos`. |
| `host` | string | Proxy host and port, such as `proxy.example.com:8080`. |
| `password` | string | Password for `basic` or `kerberos` authentication. |
| `realm` | string | Kerberos realm. |
| `username` | string | User name for `basic` or `kerberos` authentication. |
| `use_tls` | boolean | Reach the proxy over TLS. |

#### `seeds.network.interfaces[]`

| Key | Type | Description |
|---|---|---|
| `name` | string | Name of the bridge IncusOS creates for this interface. |
| `hwaddr` | string | MAC address of the physical device, or an interface name whose MAC is substituted at boot. |
| `addresses` | list of strings | `dhcp4`, `dhcp6`, `slaac`, or static addresses in CIDR form. |
| `mtu` | integer | MTU of the device. |
| `lldp` | boolean | Send and receive LLDP on the device. |
| `roles` | list of strings | How IncusOS uses the device: `management`, `cluster`, `instances`, or `storage`. |
| `routes` | list of `{to, via}` | Static routes bound to the device. |
| `vlan_tags` | list of integers | Extra VLAN tags the bridge accepts, on top of any `vlans` entry. |
| `required_for_online` | string | Address families that must come up before the system counts as online: `ipv4`, `ipv6`, `both`, `any`, or `no`. Defaults to `any`. |
| `strict_hwaddr` | boolean | Drop traffic leaving the device whose source MAC is not `hwaddr`. |
| `ethernet` | mapping | Offload, wake-on-LAN, and energy-efficiency settings. |
| `firewall_rules` | list | Ingress firewall rules for the device. |

Roles drive how IncusOS uses a device: `management` for management
traffic, `cluster` for internal cluster traffic, `instances` for use by
containers and virtual machines, and `storage` for network-attached
storage. With no roles set, IncusOS assigns `management` and `cluster`
to every interface.

#### `seeds.network.bonds[]`

Same addressing, role, route, VLAN-tag, ethernet, and firewall fields
as an interface, plus:

| Key | Type | Description |
|---|---|---|
| `mode` | string | Bonding mode: `balance-rr`, `active-backup`, `balance-xor`, `broadcast`, `802.3ad`, `balance-tlb`, or `balance-alb`. |
| `members` | list of strings | Interfaces enslaved to the bond, named by MAC or interface name. |

`hwaddr` is optional on a bond.

#### `seeds.network.vlans[]`

| Key | Type | Description |
|---|---|---|
| `name` | string | Name of the VLAN device. |
| `id` | integer | 802.1Q VLAN ID carried on the parent. |
| `parent` | string | Interface or bond that carries the VLAN. |
| `addresses` | list of strings | `dhcp4`, `dhcp6`, `slaac`, or static addresses in CIDR form. |
| `mtu` | integer | MTU of the device. |
| `roles` | list of strings | How IncusOS uses the device: `management`, `cluster`, `instances`, or `storage`. |
| `routes` | list of `{to, via}` | Static routes bound to the device. |
| `required_for_online` | string | Address families that must come up before the system counts as online. |
| `firewall_rules` | list | Ingress firewall rules for the device. |

#### `seeds.network.wireguard[]`

| Key | Type | Description |
|---|---|---|
| `name` | string | Name of the WireGuard device. |
| `addresses` | list of strings | Static addresses assigned to the tunnel, in CIDR form. |
| `mtu` | integer | MTU of the tunnel. |
| `port` | integer | UDP port WireGuard listens on. |
| `private_key` | string | Base64 private key of this endpoint; generated when empty. |
| `roles` | list of strings | How IncusOS uses the device: `management`, `cluster`, `instances`, or `storage`. |
| `routes` | list of `{to, via}` | Static routes bound to the tunnel. |
| `required_for_online` | string | Address families that must come up before the system counts as online. |
| `firewall_rules` | list | Ingress firewall rules for the tunnel. |
| `peers` | list | Remote endpoints of the tunnel. |

Each peer:

| Key | Type | Description |
|---|---|---|
| `public_key` | string | Base64 public key of the peer. |
| `allowed_ips` | list of strings | Networks routed to the peer and accepted from it. |
| `endpoint` | string | Host and port used to reach the peer. |
| `persistent_keepalive` | integer | Seconds between keepalive packets, for peers behind NAT. |
| `preshared_key` | string | Optional symmetric key mixed into the handshake. |

#### Shared network nested objects

`ethernet`:

| Key | Type | Description |
|---|---|---|
| `disable_energy_efficient` | boolean | Turn off Energy-Efficient Ethernet. |
| `disable_gro` | boolean | Turn off generic receive offload, in software and hardware. |
| `disable_gso` | boolean | Turn off generic segmentation offload. |
| `disable_ipv4_tso` | boolean | Turn off TCP segmentation offload for IPv4. |
| `disable_ipv6_tso` | boolean | Turn off TCP segmentation offload for IPv6. |
| `wakeonlan` | boolean | Enable wake-on-LAN, using the `magic` mode unless `wakeonlan_modes` is set. |
| `wakeonlan_modes` | list of strings | systemd wake-on-LAN modes to set instead of `magic`. |
| `wakeonlan_password` | string | SecureOn password, used only with the `secureon` mode. |

`firewall_rules[]`:

| Key | Type | Description |
|---|---|---|
| `action` | string | What to do with matching traffic: `accept`, `drop`, or `reject`. |
| `source` | string | Source address or CIDR the rule matches; empty matches any source. |
| `protocol` | string | `tcp` or `udp`; required whenever `port` is set. |
| `port` | integer | Destination port the rule matches; required whenever `protocol` is set. |

On top of these rules, IncusOS always allows ICMP, ICMPv6, and
established connections.

`routes[]`:

| Key | Type | Description |
|---|---|---|
| `to` | string | Destination prefix in CIDR form. |
| `via` | string | Gateway used to reach that prefix. |

### `seeds.operations-center`

Operations Center seed. The YAML key is kebab-case.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `trusted_client_certificates` | list of strings | no | PEM client certificates Operations Center should trust. |
| `apply_defaults` | boolean | no (`false`) | Apply upstream's reasonable Operations Center defaults while installing it. |
| `preseed` | mapping | no | Preseed handed to Operations Center during install. |

PEM trusted-client certificates are handled as in
`seeds.migration-manager`.

`preseed` first-level keys:

| Key | Type | Description |
|---|---|---|
| `system_certificate` | mapping | Server certificate and private key. |
| `system_network` | mapping | Address managed servers use, and the REST API listen address. |
| `system_security` | mapping | Trusted clients and proxies, OIDC, OpenFGA, and ACME. |
| `system_updates` | mapping | Upstream update source, signature trust, filters, and default channels. |
| `system_settings` | mapping | Daemon log level and the server-registration scriptlet. |

Those mappings are
`github.com/FuturFusion/operations-center/shared/api/system` types.

### `seeds.provider`

Update and configuration provider. Fields of
`api.SystemProviderConfig` are inlined next to `version`.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `name` | string | no | Provider serving updates and applications: `images`, `operations-center`, or `debug`. |
| `config` | string-to-string map | no | Provider-specific settings, such as the server URL. |

`debug` exists for IncusOS development and is not meant for general
use.

### `seeds.services`

Auxiliary service toggles.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `iscsi` | mapping | no | iSCSI initiator for remote block storage over TCP. |
| `lvm` | mapping | no | Clustered LVM storage backend. |
| `multipath` | mapping | no | Multipath access to SAN devices. |
| `netbird` | mapping | no | NetBird VPN client. |
| `nvme` | mapping | no | NVMe-over-TCP or NVMe-over-Fibre-Channel initiator. |
| `ovn` | mapping | no | OVN software-defined networking chassis. |
| `tailscale` | mapping | no | Tailscale VPN client. |
| `usbip` | mapping | no | Access to remote USB devices over IP. |

The services seed is applied on first boot, before any service starts.

#### `seeds.services.iscsi`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the iSCSI service. |
| `targets` | list of `{target, address, port}` | iSCSI targets to connect to, each with its target name, address, and port. |

#### `seeds.services.lvm`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the LVM service. |
| `system_id` | integer | Cluster-unique host identifier for this system. |

#### `seeds.services.multipath`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the Multipath service. |
| `wwns` | list of strings | Device World Wide Names to configure for multipath, as lowercase hex without separators. |

#### `seeds.services.netbird`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the NetBird service. |
| `setup_key` | string | NetBird setup key used to enroll this peer. |
| `management_url` | string | NetBird management server. |
| `admin_url` | string | NetBird admin server. |
| `anonymize` | boolean | Anonymize IP addresses and non-`netbird.io` domains in logs and status output. |
| `block_inbound` | boolean | Refuse all inbound connections. |
| `block_lan_access` | boolean | Block access to local networks when this peer acts as a router or exit node. |
| `disable_client_routes` | boolean | Ignore routes received from the management service. |
| `disable_server_routes` | boolean | Do not act as a router for server routes received from the management service. |
| `disable_dns` | boolean | Leave system DNS settings alone. |
| `disable_firewall` | boolean | Leave firewall rules alone. |
| `dns_resolver_address` | string | Custom address for NetBird's local DNS resolver. |
| `external_ip_map` | list of strings | Maps between local addresses and interfaces. |
| `extra_dns_labels` | list of strings | Additional DNS labels for this peer. |

#### `seeds.services.nvme`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the NVMe service. |
| `targets` | list | NVMe targets to connect to. |

Each NVMe target:

| Key | Type | Description |
|---|---|---|
| `transport` | string | Transport type: `tcp` or `fc`. |
| `address` | string | With `tcp`, the target IP address. With `fc`, the remote port World Wide Names as `nn-0x<WWNN>:pn-0x<WWPN>`. |
| `host_address` | string | With `fc`, the local Fibre Channel port to connect from; all local ports are used when unset. |
| `port` | integer | With `tcp`, the target port. Unused with `fc`. |
| `nqn` | string | Connect straight to this subsystem NQN instead of using the target's discovery controller. |

#### `seeds.services.ovn`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the OVN service. |
| `ic_chassis` | boolean | Use this chassis as an interconnection gateway. |
| `database` | string | OVN database this system fetches its configuration from. |
| `tls_client_certificate` | string | PEM client certificate for the database connection. |
| `tls_client_key` | string | PEM client key for the database connection. |
| `tls_ca_certificate` | string | PEM CA certificate for the database connection. |
| `tunnel_address` | string | Address other chassis use to reach this node; comma-separate several. |
| `tunnel_protocol` | string | Encapsulation other chassis use to reach this node; comma-separate several. |

#### `seeds.services.tailscale`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the Tailscale service. |
| `login_server` | string | Tailscale (or Headscale) login server. |
| `auth_key` | string | Tailscale authentication key used to enroll this node. |
| `accept_routes` | boolean | Accept subnet routes advertised by the tailnet. |
| `accept_dns` | boolean | Accept DNS configuration advertised by the tailnet. |
| `advertised_routes` | list of strings | Subnet routes this node advertises to the tailnet. |
| `advertise_exit_node` | boolean | Offer this node as an exit node for tailnet internet traffic. |
| `exit_node` | string | Exit node to route internet traffic through, by IP, base name, or `auto:any`. |
| `exit_node_allow_lan_access` | boolean | Keep direct local network access while using an exit node. |
| `serve_enabled` | boolean | Expose `localhost:8443`, normally the Incus API, through Tailscale Serve. |
| `serve_port` | integer | TCP port Tailscale Serve publishes the HTTPS server on. |
| `serve_service` | string | Tailscale Service to publish as; not passed when empty. |

#### `seeds.services.usbip`

| Key | Type | Description |
|---|---|---|
| `enabled` | boolean | Enable the USBIP service. |
| `targets` | list of `{address, bus_id}` | Remote USB devices to attach, each by host address and bus ID. |

### `seeds.update`

Update daemon seed. Fields of `api.SystemUpdateConfig` are inlined
next to `version`.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `auto_reboot` | boolean | no (`false`) | Reboot on its own after an OS update is applied, interrupting service. |
| `channel` | string | no | Update channel to follow, normally `stable` or `testing`. |
| `check_frequency` | string | no | How often to check for updates, as a Go duration such as `6h`, or `never` to disable checks. Upstream defaults to six hours. |
| `maintenance_windows` | list | no | Restrict when updates may be checked for and applied. |

When `image.offline` is `true`, `check_frequency` is `never` after
defaults, even if the document set another value.

Each maintenance window:

| Key | Type | Description |
|---|---|---|
| `start_day_of_week` | string | Day the window opens on; omitted makes the window repeat daily. |
| `start_hour` | integer | Hour the window opens, in the system timezone. |
| `start_minute` | integer | Minute the window opens. |
| `end_day_of_week` | string | Day the window closes on; required whenever `start_day_of_week` is set. |
| `end_hour` | integer | Hour the window closes, in the system timezone. |
| `end_minute` | integer | Minute the window closes. |

Accepted weekday strings on the pin: `Sunday`, `Monday`, `Tuesday`,
`Wednesday`, `Thursday`, `Friday`, `Saturday`. Omitted is empty.

incusos-builder does not run upstream `SystemUpdateConfig.Validate()`
at parse time.

### `seeds.kernel`

CLI extension. Rendered as `kernel.yaml` after the nine customizer
members.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `console` | list | no | Console devices the IncusOS terminal interface should use. |

Each console entry:

| Key | Type | Description |
|---|---|---|
| `device` | string | Console device path, such as `/dev/ttyS0`. |
| `baud_rate` | integer | Speed to configure that console device at. |

### `seeds.security`

CLI extension. Rendered as `security.yaml` last. Fields of
`api.SystemSecurityConfig` are inlined next to `version`.

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | string | no (default `"1"`) | Schema version of this seed file. |
| `custom_ca_certs` | list of strings | no | PEM X.509 certificates added to the system as extra certificate authorities. |
| `encryption_recovery_keys` | list of strings | no; must be empty if present | Recovery keys for the encrypted system drive. Not settable from a seed; see below. |

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
