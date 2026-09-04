# arazzo-go

A command-line runner for [Arazzo 1.0.1](https://spec.openapis.org/arazzo/v1.0.1.html) workflows. It loads an Arazzo document, resolves the referenced OpenAPI sources, and executes HTTP steps with a cookie jar so session cookies persist across calls.

## Install

```bash
go install github.com/shaunhoulihan/arazzo-go/cmd/arazzo@latest
```

Or from this repo:

```bash
go run ./cmd/arazzo --help
```

## Commands

```bash
arazzo list-workflows <file>
arazzo describe-workflow <file> --workflow-id <id>
arazzo execute-workflow <file> --workflow-id <id> [--inputs <json>]
```

`--inputs` is a JSON object of workflow inputs. If the workflow uses a `baseUrl` input (or you need to target a specific host), include it there.

```bash
arazzo execute-workflow testdata/login.arazzo.yaml \
  --workflow-id login-with-password \
  --inputs '{"email":"user@example.com","password":"...","baseUrl":"http://localhost:8080/api/v0"}'
```

## Options

| Flag | Description |
| --- | --- |
| `--workflow-id` | Workflow to run or describe |
| `--inputs` | JSON object of workflow inputs (default `{}`) |
| `--log-level` | `debug`, `info`, `warn`, or `error` (default `info`) |

The runner does not inject authentication on its own. Pass credentials as workflow inputs, and rely on the cookie jar for subsequent steps after a login. Password, token, and session fields are redacted from request logs.
