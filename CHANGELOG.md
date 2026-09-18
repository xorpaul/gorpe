# Changelog

All notable changes to this project will be documented in this file.

## [v2.4.0] - 2026-09-18

### 🔒 Security Fixes

- **OCSP response serial binding**: `ocsp.ParseResponse` was replaced with `ocsp.ParseResponseForCert(body, cert, issuer)` — without the cert argument the library accepted any CA-signed "Good" response regardless of serial number, allowing a response minted for one certificate to be replayed to authenticate a different one
- **Confirmed revocations now hard-fail**: A `*revokedError` sentinel distinguishes definitive OCSP/CRL revocation results from transient infrastructure errors; `checkCertAuth` hard-fails on the former and soft-fails only on the latter
- **CRL signature verification**: `crl.CheckSignatureFrom(issuer)` is called after parsing every CRL; a CRL with an invalid or unverifiable signature is skipped rather than trusted
- **Unverifiable CRL skipped**: When the issuer CA cert is not loaded, the CRL distribution point is skipped entirely instead of being used without signature verification

### 🔧 Correctness Fixes

- **HTTP timeout on revocation fetches**: OCSP and CRL HTTP calls now use a client with a 10 s timeout; previously the default client (no timeout) could block handler goroutines indefinitely on a slow PKI server
- **DN formatting for non-UTF8 ASN.1 string types**: `subjectToSlashDN` used `fmt.Sprintf("%v")` on raw `interface{}` values, producing decimal byte-slice notation for `[]byte`-typed attributes and breaking DN matching; a type switch now handles `string` and `[]byte` correctly
- **`issuerCerts` keyed by raw DER bytes**: The CA cert map is now keyed by `string(cert.RawSubject)` and looked up by `string(cert.RawIssuer)`, avoiding silent mismatches caused by ASN.1 string-type encoding differences (PrintableString vs UTF8String)
- **Revocation cache pruning**: A background goroutine sweeps expired entries from the revocation cache once per hour, preventing unbounded growth in long-running deployments with cert renewals

### 🧪 Tests

- Added `certauth_test.go` with 10 automated tests covering the fixes above, including a live OCSP responder, CRL server, tampered-CRL rejection, OCSP replay detection, and soft-fail behaviour

---

## [v2.3.0] - 2026-08-14

### ✨ New Features

- **JSON Response Mode**: Clients can now request JSON-formatted output by sending `Accept: application/json` — useful for programmatic consumers of gorpe responses
- **TCP Keepalive Wiring**: `connection_timeout` config value is now applied as the TCP keepalive period for accepted connections, ensuring stale connections are detected and cleaned up
- **Client cert DN authorization**: When `verify_client_cert: 1` and `client_auth_dns` are configured, gorpe now validates the client certificate's Subject DN against the allow-list (OpenSSL slash format), checks the Issuer DN against `client_auth_issuer`, and verifies revocation via OCSP (preferred) or CRL fallback with per-cert caching until the CA's `NextUpdate`

### ⚠️ Breaking Changes

- Config key `allowed_hosts` renamed to `allowed_ips` — update `gorpe.yaml` (and the Puppet template that renders it) before deploying v2.3.0

### 🔧 Maintenance

- Updated to Go 1.26.5
- Updated vendor dependencies for 2026 (added `golang.org/x/crypto` for OCSP support)

---

## [v2.2.1] - 2025-10-09

### ✨ New Features

- **Build Version Tracking**: Binary now embeds build version information
- **Service Uptime**: The `/` status endpoint now reports service uptime alongside other metrics
- **Windows Support**: Skip syslog initialisation on Windows builds so the binary compiles and runs cross-platform

### 🔧 Maintenance

- Added `build_release` as a submodule for consistent release tooling
- Updated release script
- Updated vendor dependencies

---

## [v2.2] - 2025-03-13

### 🔒 Security Improvements

- **Stronger SSL Ciphers**: Switched TLS configuration to a more restrictive and modern cipher suite, dropping weak/legacy ciphers

### 🔧 Technical Improvements

- **Modernised I/O**: Replaced deprecated `ioutil.ReadFile` calls with `os.ReadFile`
- Removed `--verbose` flag (debug logging is controlled via config)
- Migrated to updated `build_release.sh` tooling
- Updated vendor dependencies for 2025

---

## [v0.0.1] - 2022-03-10

### Initial Tagged Release

This tag captures the project after a series of major rewrites and feature additions spanning 2015–2022.

#### Core Features

- **Remote command execution**: HTTPS-based daemon that executes whitelisted commands (Nagios checks, Puppet agent runs, etc.) on behalf of remote callers
- **Client certificate authentication**: Mutual TLS — server verifies the caller's client certificate before executing any command
- **HTTP/2**: Transport upgraded to HTTP/2 for multiplexing and lower latency
- **YAML config**: Configuration switched from GCFG to YAML format with a cleaner structure and code split into multiple files
- **Nasty metacharacter detection**: Command arguments are validated against a shell metacharacter blocklist to prevent injection; invalid requests return an error with perfdata
- **Perfdata output**: Both the `/` status endpoint and failed-request counters emit Nagios-compatible performance data
- **Request logging**: All incoming requests are logged with method, path, and client identity
- **`check_gorpe` integration**: Client-side `check_gorpe` helper documented and linked for triggering gorpe from Nagios/Icinga

#### Historical Milestones (pre-tag)

- 2015 — initial implementation over SPDY/HTTPS
- 2015 — complete rework with `shellquote.Split` for safe argument parsing
- 2015 — added upload-file feature (later removed), syslog and debug logging
- 2016 — SSL/TLS client certificate verification and HTTP/2 upgrade
- 2020 — switched to YAML config, removed upload feature, refactored into separate files
- 2021 — improved `ExecuteCommand` return code handling and debug output reliability
- 2022 — fixed `gencerts.GenerateCert` call, added output log, removed upload section and debug print statements
