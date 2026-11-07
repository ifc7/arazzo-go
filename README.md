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
arazzo execute-workflow <file> --workflow-id <id> [--inputs <json>] [--inputs-file <path|->] [--env-file <path>] [--format text|json|junit] [--output <path>] [--verbose]
```

## Workflow Inputs

Inputs can be supplied from multiple sources, merged with the following precedence (lowest to highest):

1. **Environment variables** (`ARAZZO_INPUT_<NAME>`)
2. **Env file** (`--env-file <path>`)
3. **Inputs file** (`--inputs-file <path|->` for JSON/YAML from file or stdin)
4. **CLI flag** (`--inputs <json>`)

Remaining unset inputs take their default values from the workflow's input schema (`applyWorkflowInputDefaults`).

### Name Mapping and Type Coercion

Workflow input names are camelCase (e.g. `baseUrl`, `password`, `portNumber`). Environment variables and `.env` keys are matched case-insensitively against the input schema's `properties` (e.g. `ARAZZO_INPUT_BASEURL` or `ARAZZO_INPUT_baseUrl` maps to `baseUrl`).

Values from environment variables and `.env` files are coerced according to the schema property's `type`:
- `integer`: coerced to integer (e.g. `8080`)
- `number`: coerced to float (e.g. `3.14`)
- `boolean`: coerced to boolean (`true`, `false`, `1`, `0`, `yes`, `no`)
- `object`: parsed as JSON
- `array`: parsed as JSON
- `string`: passed as-is

If coercion fails, the runner reports a clear error without leaking sensitive values.

### Input Examples

```bash
# Via CLI flag
arazzo execute-workflow testdata/login.arazzo.yaml \
  --workflow-id login-with-password \
  --inputs '{"email":"user@example.com","password":"...","baseUrl":"http://localhost:8080/api/v0"}'

# Via environment variables
export ARAZZO_INPUT_BASEURL="http://localhost:8080/api/v0"
export ARAZZO_INPUT_EMAIL="user@example.com"
export ARAZZO_INPUT_PASSWORD="secret"
arazzo execute-workflow testdata/login.arazzo.yaml --workflow-id login-with-password

# Via env file
arazzo execute-workflow testdata/login.arazzo.yaml --workflow-id login-with-password --env-file .env

# Via YAML/JSON file or stdin
arazzo execute-workflow testdata/login.arazzo.yaml --workflow-id login-with-password --inputs-file inputs.yaml
cat inputs.json | arazzo execute-workflow testdata/login.arazzo.yaml --workflow-id login-with-password --inputs-file -
```

## Output Formats

`execute-workflow` supports three output formats via `--format`:

- **`text`** (default): Human-readable terminal output styled with ANSI colors using Lipgloss. Displays workflow status, execution duration, and step-by-step progress.
- **`json`**: Structured JSON report providing `workflowId`, `success`, `durationMs`, `error`, `outputs`, and individual `steps[]` with their HTTP status codes and retries. Secret fields (`password`, `token`, `session`, `cookie`, etc.) are automatically redacted to `***`. Conforms to the JSON schema in [`schemas/workflow-result.schema.json`](schemas/workflow-result.schema.json).
- **`junit`**: Standard JUnit XML format (`<testsuites>`, `<testsuite>`, `<testcase>`, `<failure>`) for native test reporting in CI/CD platforms (e.g. GitHub Actions, GitLab CI, CircleCI). Each workflow step is mapped to a test case. If execution fails before steps run (e.g. invalid inputs or connection error), a synthetic test case is emitted so CI pipelines receive a complete failure report.

### File Output & CI Reporting

Direct reports to a file using `--output <path>` or `-o <path>`. Parent directories are created automatically if they do not exist:

```bash
# Export JUnit XML report for CI test reporting
arazzo execute-workflow testdata/login.arazzo.yaml \
  --workflow-id login-with-password \
  --format junit \
  --output reports/junit.xml \
  --inputs-file inputs.yaml

# Export sanitized JSON execution results
arazzo execute-workflow testdata/login.arazzo.yaml \
  --workflow-id login-with-password \
  --format json \
  -o reports/result.json \
  --inputs-file inputs.yaml
```

In machine-readable modes (`json` and `junit`), CLI errors are routed to `stderr` without ANSI color codes, keeping stdout or report files clean and parseable. The CLI exits with a non-zero code (`1`) on workflow failure so CI pipelines properly register failures while capturing test report artifacts.

## Options

| Flag | Description |
| --- | --- |
| `--workflow-id` | Workflow to run or describe |
| `--inputs` | JSON object of workflow inputs (default `{}`) |
| `--inputs-file` | Path to JSON/YAML inputs file, or `-` for stdin (`execute-workflow` only) |
| `--env-file` | Path to `.env` file (`execute-workflow` only) |
| `--format` | Output format: `text`, `json`, or `junit` (default `text`, `execute-workflow` only) |
| `--output`, `-o` | Write report to file instead of stdout (`execute-workflow` only) |
| `--verbose`, `-v` | Log each HTTP request and response (`execute-workflow` only) |
| `--log-level` | `debug`, `info`, `warn`, or `error` (default `info`). Request traces require `debug` or `--verbose`. |
| `--version` | Print the release version, git commit, and build time, then exit. |

The runner does not inject authentication on its own. Pass credentials as workflow inputs, and rely on the cookie jar for subsequent steps after a login. Password, token, and session fields are redacted from request logs when `--verbose` is on.
