# f2pool-cli

Go CLI for the [F2Pool V1](https://www.f2pool.com/api_doc) read-only API and the 34 POST endpoints in the [V2 API documentation](https://api.f2pool.com/v2/doc/en.html). The [original API entry page](docs/f2pool-api-entry.html), [V1 English HTML](docs/f2pool-api-v1-en.html), [V2 English HTML](docs/f2pool-api-v2-en.html), and [V2 Chinese HTML](docs/f2pool-api-v2-zh.html) were downloaded on 2026-11-06. The machine-readable [V2 endpoint catalog](internal/catalog/endpoints.json) is extracted from the English V2 HTML. Run `python3 docs/generate_catalog.py` after refreshing the HTML to update it.

## Build

```sh
go build -o f2pool ./cmd/f2pool
```

Prebuilt archives for Linux, macOS, and Windows are available on [GitHub Releases](https://github.com/lihongjie0209/f2pool-cli/releases). Extract the archive for your OS and architecture, then run `f2pool --version` to check the installed version.

## Authentication

Create an API token through F2Pool, then set it in the environment:

```sh
export F2POOL_API_TOKEN='your-token'
```

The CLI sends this value as `F2P-API-SECRET`. V2 requests use POST with JSON bodies; V1 requests use GET and require the token's `legacy/api_v1` permission. No token is needed for `--dry-run`, `endpoints`, or help.

## Commands

V2 command names follow the route after `/v2/`, with underscores changed to hyphens. For example:

```sh
./f2pool endpoints
./f2pool endpoints --json
./f2pool mining-user get --mining-user-name alice
./f2pool assets balance --currency bitcoin --mining-user-name alice
./f2pool hash-rate worker history --currency bitcoin --mining-user-name alice --worker-name rig1 --interval 600 --duration 86400
./f2pool blocks paging --currency bitcoin --page 1 --pagesize 20
```

Each endpoint has flags for every request field shown in the saved docs. Simple requests need only command-line flags; no JSON input is needed. Run `./f2pool <command> --help` for types and descriptions. Numeric flags accept decimal values; boolean flags can be written alone (or with `=false`). Repeat array flags for multiple values:

```sh
./f2pool assets balance --currency bitcoin --mining-user-name alice --calculate-estimated-income
./f2pool mining-user wallet history --mining-user-name alice --currencies bitcoin --currencies litecoin
```

The few endpoints with nested objects accept JSON for those object values:

```sh
./f2pool hash-rate info-list \
  --reqs '{"currency":"bitcoin","mining_user_name":"alice"}' \
  --reqs '{"currency":"bitcoin","mining_user_name":"bob"}'
```

Those nested-object endpoints also offer `--data` for a full JSON object, `--data @request.json` for a file, or `--data @-` for stdin. Individual field flags override keys in `--data`:

```sh
./f2pool mining-user wallet update --data @wallet-update.json
```

`call` accepts a V2 path for newly added routes:

```sh
./f2pool call /v2/mining_user/list --data '{}'
```

V1 is a separate command group for its legacy read-only API:

```sh
./f2pool v1 account --currency bitcoin --user alice
./f2pool v1 account --currency bitcoin --user alice --history 60
./f2pool v1 worker --currency bitcoin --user alice --worker-name rig1
./f2pool v1 multi-account --currency bitcoin --user alice --user bob
```

V1 offers a combined account response, supports some currencies absent from the V2 documentation's currency list, and allows up to 60 days of account hashrate history. Its documented account-details rate limit is 15 requests per minute.

Responses print formatted JSON by default. Use `--output raw` for the original response bytes, `--timeout 10s` to change the timeout, or `--dry-run` to inspect the path and body without sending a request. `--base-url` is available for a trusted alternate API origin or a local test server.

Mutating endpoints execute immediately, including withdrawal and distribution commands. Review their payloads with `--dry-run` before use. This project has no live API credentials, so automated tests verify request behavior against a local server rather than the F2Pool service.
