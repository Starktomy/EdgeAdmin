# Signed administrator upgrades

Automatic installation now requires an independently configured release public
key and a signed manifest. Existing unsigned ZIPs are rejected. There is no
insecure-install switch. A TLS certificate alone does not authenticate a release
publisher. The repository contains no official publisher key: obtain one through
a trusted administrative channel, or sign releases you build and publish.

## Prepare a release

From `EdgeAdmin/`, generate an Ed25519 signing key **outside the repository** on
an offline/release signing machine:

```sh
go run ./cmd/edge-sign-release -generate-key -key /secure/release-signing.pem
```

Store the private key only on that machine. The command prints the base64 public
key. Sign the final ZIP after building it; the ZIP must contain
`edge-admin/bin/edge-admin` and the usual distribution directories:

```sh
go run ./cmd/edge-sign-release \
  -key /secure/release-signing.pem -key-id release-2026 \
  -artifact /releases/edge-admin-linux-amd64-v1.5.0.zip \
  -version 1.5.0 -os linux -arch amd64 -component admin
```

Publish both the ZIP and the generated `ZIP.manifest.json` sidecar over HTTPS,
for example `/releases/edge-admin-linux-amd64-v1.5.0.zip.manifest.json`.
The sidecar contains an Ed25519 signature over the exact base64-decoded JSON
payload, binding component, version, OS, architecture, SHA-256 and ZIP size.
Do not sign a downloaded package until it has been independently verified.

## Configure an administrator installation

Create `configs/upgrade-trust.json` next to the running installation's other
configuration files, owned by the service administrator and not writable by
untrusted users:

```json
{
  "publicKeys": {
    "release-2026": "BASE64_ED25519_PUBLIC_KEY_FROM_THE_SIGNER"
  },
  "allowedHosts": ["releases.example.com", "goedge.cloud"]
}
```

Use exact HTTPS authorities (hostname, with `:port` if non-default); wildcards
and suffix matches are not supported. Redirects must remain HTTPS and target an
allowed authority, with at most four redirects. Include `goedge.cloud` only when
using its version discovery endpoint; include every actual artifact/sidecar
host. Each release still needs a signature from a configured key. Key rotation
can temporarily configure both old and new key IDs, then remove the old key.
Do not accept keys supplied by the downloaded manifest itself.

The UI and `edge-admin upgrade --url=...` accept the ZIP URL. Both fetch the
sidecar, verify its signer/platform/newer version, then check the ZIP's signed
size and digest **before extraction or overwrite**. Limits are 1 MiB metadata,
512 MiB ZIP, 2 GiB extracted data and 20,000 entries. Traversal, symlinks, device
entries and duplicate paths are rejected. Staging uses a private temporary
directory. Installation still uses the existing copy/restart process; staged
version activation and automatic rollback remain future work.
