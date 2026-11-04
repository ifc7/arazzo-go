# arazzo-go

A command-line runner for [Arazzo 1.0.1](https://spec.openapis.org/arazzo/v1.0.1.html) workflows. It loads an Arazzo document, resolves the referenced OpenAPI sources, and executes HTTP steps with a cookie jar so session cookies persist across calls.

## Install

```bash
go install github.com/ifc7/arazzo-go/cmd/arazzo@latest
```

Or from this repo:

```bash
go run ./cmd/arazzo --help
```

### Docker

Multi-arch images (`linux/amd64`, `linux/arm64`) are published to [Docker Hub](https://hub.docker.com/r/ifc7/arazzo) for each release tag (`vX.Y.Z` and `latest`). The image's working directory is `/work`, so mount your workflow files there:

```bash
docker run --rm -v "$PWD:/work" ifc7/arazzo execute-workflow wf.yaml \
  --workflow-id <id> --inputs '{"baseUrl":"http://host.docker.internal:8080"}'
```

The container runs as a non-root user. If you write results into the mounted directory with `--output`, add `--user "$(id -u):$(id -g)"` so the container can write the file. Use `host.docker.internal` (Docker Desktop) or `--network host` (Linux) to reach APIs running on your machine.

To build locally:

```bash
docker build -t arazzo .
```

## Commands

```bash
arazzo list-workflows <file>
arazzo describe-workflow <file> --workflow-id <id>
arazzo execute-workflow <file> --workflow-id <id> [--inputs <json>] [--verbose]
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
| `--verbose`, `-v` | Log each HTTP request and response (`execute-workflow` only) |
| `--log-level` | `debug`, `info`, `warn`, or `error` (default `info`). Request traces require `debug` or `--verbose`. |

The runner does not inject authentication on its own. Pass credentials as workflow inputs, and rely on the cookie jar for subsequent steps after a login. Password, token, and session fields are redacted from request logs when `--verbose` is on.
