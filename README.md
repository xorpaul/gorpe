# gorpe

A lightweight HTTPS daemon that executes whitelisted commands on remote hosts and returns their output and exit code. Designed for Nagios/Icinga-style monitoring checks, Puppet agent runs, and other ops tooling that needs a secure, audited execution channel without full SSH.

See also: [check_gorpe](https://github.com/xorpaul/check_gorpe) — the companion Nagios/Icinga plugin for calling gorpe from a monitoring system.

## Features

- **Allowlist-only execution** — only commands defined in `gorpe.yaml` can be run; unknown paths return an error
- **IP-based access control** — requests from IPs not in `allowed_ips` are rejected before any command is executed
- **No shell** — commands are split into words and executed directly; arguments are inserted per word, so they can never escape the quoting of the command template. A metacharacter blocklist adds defence in depth for commands that hand arguments to a shell themselves
- **Mutual TLS** — optional client certificate verification; auto-generates a self-signed CA and server cert on first run
- **HTTP/2** — low-latency transport with multiplexing
- **TCP keepalive** — configurable keepalive period prevents stateful firewalls/NAT from dropping long-running command connections
- **JSON response mode** — send `Accept: application/json` to get `{"exit_code": N, "output": "..."}` with the raw exit code (not clamped to 0–3), useful for programmatic consumers like gorpe-mcp that need Puppet's exit codes 4 and 6
- **Nagios-compatible text mode** — default plain-text response clamps exit codes to 0–3 and appends perfdata
- **Status endpoint** — `GET /` returns version, uptime, TCP keepalive period, TLS mode, and request counters as perfdata
- **Request logging** — every request is logged with a random ID for correlation

## Installation

```
go install github.com/xorpaul/gorpe@latest
```

Or build from source:

```
go build -o gorpe .
```

## Configuration

Copy `gorpe.yaml` to `/etc/gorpe/gorpe.yaml` and adjust:

```yaml
main:
  server_port: 5666
  server_address: 0.0.0.0      # bind address
  allowed_ips:
    - 192.168.1.10              # monitoring server
    - 10.0.0.5
  debug: 0                      # 1 = verbose request logging
  command_timeout: 60           # seconds before a command is killed (SIGTERM to its
                                # process group, SIGKILL 2s later); returns UNKNOWN
  connection_timeout: 30        # TCP keepalive period in seconds; set below
                                # the firewall/NAT idle timeout on your path
  certs_dir: /etc/gorpe/ssl/    # server cert/key location
  verify_client_cert: 0         # 1 = require a valid client certificate
  ca_file: /etc/gorpe/ssl/gorperootca.pem
  # Optional: restrict which client certs are accepted (requires verify_client_cert: 1)
  # client_auth_dns:
  #   - /C=DE/O=Example/CN=monitoring-agent
  # client_auth_issuer:
  #   - /C=DE/O=Example/CN=IssuingCA
  # client_auth_issuer_ca_files:
  #   - /etc/gorpe/ssl/issuing-ca.pem
  # Optional: commands whose arguments may contain quotes and | [] {} etc.
  # (only control characters are rejected). Only list commands that pass
  # their arguments to a program as an argv, never through a shell.
  # relaxed_arg_commands:
  #   - journalctl_wild

commands:
  puppet_agent: /usr/bin/puppet agent -t
  check_disk: /usr/lib/nagios/plugins/check_disk -w 20% -c 10% -p /
  # with arguments — each $ARG$ is filled by arg1, arg2, ... in order
  check_disk_path: /usr/lib/nagios/plugins/check_disk -w 20% -c 10% -p "$ARG$"
  echo_args: echo "$ARG$ and $ARG$"
  # an unquoted $ARG$ on its own is split into several words
  journalctl_wild: /usr/bin/journalctl $ARG$ --no-pager
```

## Running

```
gorpe -config /etc/gorpe/gorpe.yaml
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `-config` | `gorpe.yaml` | Path to config file |
| `-fg` | false | Stay in foreground (don't daemonise) |
| `-debug` | false | Enable debug output |

## Usage

### Plain-text (Nagios/Icinga compatible)

```
$ curl -k https://target:5666/check_disk
DISK OK - free space: / 42 GB (55%); | /=34GB;...
Result Code: 0

$ curl -k https://target:5666/check_disk_path/%2Fvar
DISK OK - free space: /var 10 GB (72%); | ...
Result Code: 0
```

Exit codes 0–3 map to OK / WARNING / CRITICAL / UNKNOWN, matching the Nagios plugin standard.

### With arguments

Arguments are sent as form fields `arg1`, `arg2`, … (GET query or POST body) and fill the `$ARG$` placeholders in that order. Empty arguments are skipped.

The command template is split into words with shell quoting rules *before* the arguments are inserted, and the result is executed directly — there is no shell:

- An unquoted `$ARG$` that forms a whole word is replaced by the argument split into words, honouring quotes inside the argument: `-u nginx --since "1 hour ago"` becomes `-u`, `nginx`, `--since`, `1 hour ago`.
- A quoted `$ARG$` (`'$ARG$'`, `"$ARG$"`) or one embedded in a larger word (`-w$ARG$%`) is replaced verbatim inside that single word. Quotes in the argument cannot end the template's quoting or add extra words.

```
$ curl -k https://target:5666/echo_args -d arg1=hello -d arg2=world
hello and world
Result Code: 0
```

### JSON mode

Send `Accept: application/json` to get a structured response with the raw (unclamped) exit code:

```
$ curl -k -H 'Accept: application/json' https://target:5666/puppet_agent
{"exit_code": 2, "output": "Notice: Finished catalog run in 12.34 seconds\n", "stdout": "Notice: Finished catalog run in 12.34 seconds\n", "stderr": ""}
```

This is especially useful for consumers that need Puppet's exit codes 4 (changes applied) and 6 (changes applied with failures), which the legacy text mode clamps to 3.

### Status endpoint

```
$ curl -k https://target:5666/
GORPE version v2.3.0 with TCP keepalive period 30s HTTP/2 SSL client certificate verification disabled Build time: ... GORPE uptime: 42.3s
|gorpe_uptime=42.3s requests=7 forbiddenrequests=0 failedrequests=0
Result Code: 0
```

## TLS / Certificate setup

On first run gorpe auto-generates a self-signed CA and server certificate under `certs_dir`. For production deployments, replace these with certificates from your internal PKI.

To require client certificates (mutual TLS), set `verify_client_cert: 1` and point `ca_file` at the CA that signed your client certs.

## Security notes

- Only commands explicitly listed under `commands:` can be executed — no arbitrary shell access
- Arguments are checked against a metacharacter blocklist (`|`, `` ` ``, `&`, `>`, `<`, `'`, `"`, `\`, `[`, `]`, `{`, `}`, `;`, newline) and rejected with exit code 3; the error names the offending character. gorpe itself never uses a shell, so the blocklist protects commands that pass arguments on to one (`eval`, `sh -c`, backticks) or to programs that can run code from them (awk, sed). Commands listed in `relaxed_arg_commands` only have control characters rejected
- Requests from IPs not listed in `allowed_ips` are logged and rejected
- `verify_client_cert: 1` adds a second authentication layer via mutual TLS
- When `client_auth_dns` is configured, both IP and certificate DN must match — the cert's Subject DN (in OpenSSL slash format) is checked against the list, the issuer DN is verified, and revocation status is checked via OCSP (preferred) or CRL with per-cert caching until the CA's `NextUpdate`

## License

See [LICENSE](LICENSE).
