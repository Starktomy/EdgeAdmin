# Administrator security changes

- Public `/files/file` returns only public files, served as attachments. Private
  downloads use `/files/private` and require an active super administrator with
  settings access. Public image URLs accept passive raster image types; SVG and
  HTML uploads are not served inline from the administrator origin.
- Form tokens are random, browser-session-bound, valid for 30 minutes, and
  consumed atomically. Sensitive account, key, password and upgrade writes
  require a token. `security.js` obtains a fresh token before same-origin AJAX
  writes; normal forms may send `csrfToken`, other clients `X-CSRF-Token`.
- Login and OTP success rotate the authenticated session ID. OTP challenges are
  bound to the requesting browser, expire in five minutes, and allow five
  attempts. Ten failed verifications lock that account's challenges for fifteen
  minutes. These challenge/rate/replay records are in memory: a process restart
  invalidates pending challenges. Multi-instance administrators need sticky
  login routing; cluster-wide limits/replay prevention require shared atomic
  challenge storage in a later change.
- Login redirect parameters accept only local paths. External URLs, authority
  paths, control characters and backslash forms are rejected.
- Forwarded client IP and HTTPS scheme headers are ignored unless the direct
  peer matches `EDGE_TRUSTED_PROXY_CIDRS`, a comma-separated list of CIDRs,
  for example `127.0.0.1/32,192.0.2.0/24`. Configure the reverse proxy to overwrite
  incoming forwarding headers. Leave this unset for direct connections. When
  enabling the existing client-IP-header setting, configure trusted proxies at
  the same time; otherwise the socket peer remains the client address. Cookies
  explicitly use HttpOnly and SameSite=Strict, and Secure for direct TLS or
  trusted HTTPS termination.
- New secret/configuration/key writes use private `0600` files and private
  `0700` backup directories, replacing old permissive file modes atomically.
  Review ownership and modes of previously installed files and external backups.
- Online installation requires release signing configuration; see
  [signed-upgrades.md](signed-upgrades.md). Unsigned legacy updates are blocked.

Administrator login submits the exact password over TLS to Admin, which marks
the raw-v1 protocol for EdgeAPI. Updated API nodes verify and migrate legacy
administrator MD5 records to salted Argon2id; plaintext passwords are not hashed
in the browser. Refresh any login pages opened before deployment. Account
creation and password changes continue to send their original raw values. User
portal password migration is separate. Administrator HTTP controls do not
replace API object-level authorization.

## RPC migration

Administrator RPC requests now use `rpcauth` v2 signed, short-lived claims;
upgrade API and administrator binaries together. The API can explicitly allow
legacy callers only for a controlled migration; the administrator always signs
v2 claims. RPC endpoints should use HTTPS with a certificate valid for the
configured endpoint hostname. Private CA deployments set `EDGE_RPC_CA_FILE`
to a PEM CA bundle (system roots are retained). Local loopback dialing still
verifies the original configured hostname. Normal connections and discovery
probes share this policy.

Plaintext HTTP RPC is rejected by default. An installation intentionally using
an API in the same host/container may set `EDGE_ALLOW_LOOPBACK_RPC=true`; it
permits only literal loopback IP endpoints (`127.0.0.1` or `::1`), never DNS names.
External APIs require verified HTTPS. The broad `EDGE_ALLOW_PLAINTEXT_RPC=true`
flag remains an explicit, unsafe migration override; the source Docker image
sets only the loopback flag. Invalid/missing CA files fail closed instead of
silently disabling verification.
