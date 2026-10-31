# Generic Go API Load Tester — User & Testing Team Guide

**Developed by [Sanjay Kavalli](https://github.com/SanjayKavalli)**

Read the [setup and testing guide](docs/TESTING-GUIDE.md) in this repository. The [HTML user manual](https://sanjaykavalli.github.io/api-load-tester/docs/USER-MANUAL.html) is hosted on GitHub Pages and may require access to the private site.

GitHub Pages publishes documentation, configuration examples, and the simulated tour below; it does not host the Go application. Run the real browser interface locally using the steps below.

## Guided Demo (No Installation)

**[Open the interactive guided demo](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html)** without running Go or connecting to an API. Choose a use case, click the highlighted traffic option, generate a dummy payload preview, and click **Start demo**. Click a result to inspect its dummy request and response, or download the sample JSON report.

| Traffic mode | Tour |
| --- | --- |
| Normal / steady traffic | [Start steady tour](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#normal) |
| Burst / simultaneous users | [Start burst tour](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#burst) |
| Multi-API Burst / single or repeated groups | [Start multi-API tour](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#multi_api_burst) |
| Weighted / custom percentages | [Start weighted tour](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#weighted) |
| Coordinated / shared records | [Start coordinated tour](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#coordinated) |

Scenario tours: [single API, 100 users](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#single-api-100-burst), [create/update 60:40](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#weighted-create-update-60-40), [100 users, one record](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#100-users-one-record), [100 users, many records](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#100-users-many-records), [cycle records](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#coordinated-cycle-records), and [three roles, one record](https://sanjaykavalli.github.io/api-load-tester/docs/DEMO.html#coordinated-three-roles-one-record).

Everything in the demo is simulated: dummy `.invalid` endpoints, synthetic patient data, canned responses, and illustrative latency/conflict results. It sends no API requests and does not execute the Go load-test engine. Real testing still requires the local application and authorized targets.

The tour follows the Classic workspace arrangement and includes the developer credit. **Export setup** and **Import setup** save and restore the tour's use case and burst schedule using a demo-only JSON format, not a real Load Lab setup or CLI configuration. Dummy preview samples can also be downloaded or copied. Dashed controls represent local-app-only features; hover, focus, or tap them for an explanation of API editing, uploads, field rules, authentication, recording, and analysis.

GitHub Pages publishes this tour with the documentation after changes are pushed to `main` and the Pages workflow succeeds. Private Pages sites may require access. For an offline tour, open [docs/DEMO.html](docs/DEMO.html) from a local checkout with its neighboring CSS and JavaScript files; GitHub's source view does not execute HTML.

## Browser Interface

Start the local UI from the project folder (Go 1.23 or newer):

```powershell
go run ./cmd/web -port 8080
```

Open http://127.0.0.1:8080. The frontend is embedded in the Go server; Node.js is not required. Build a shareable executable with `go build -o loadlab.exe ./cmd/web`.

### Choose the Workspace UI

The original Classic page is preserved at `/classic`. Balanced is available at `/balanced`, with a compact API sidebar, Add API beside the heading, focused request/rules/preview tabs, and a docked Run console. Both use the same controls and behavior scripts; no load-test engine or scenario-schema change is required.

```powershell
# Compare both; keep Classic as the initial page.
go run ./cmd/web -port 8080 -ui both -default-ui classic

# Compare both; make Balanced the initial page.
go run ./cmd/web -port 8080 -ui both -default-ui balanced

# After comparison, enable only the chosen page.
go run ./cmd/web -port 8080 -ui balanced
go run ./cmd/web -port 8080 -ui classic
```

These are alternative startup commands, not commands to run together on one port. Stop any active test and restart the UI server to change enabled layouts. Defaults are `-ui both` and Classic at `/`; when only Balanced is enabled, `/` opens Balanced automatically. Disabled layout routes return 404. The header offers the other layout only when both are enabled.

Balanced's API and settings dividers resize widths; the console divider resizes height; its request/response inspector also has an adjustable divider. Arrow keys, Shift+arrows, Home/End, divider double-click, and Reset pane sizes are supported. Only numerical pane dimensions are stored under the browser preference key `load-lab-balanced-layout-v1`. Filtering the API list does not remove APIs from the workload.

Use the same exported setup to compare pages. Each browser tab has an independent form, while both pages share one runner and its single-active-run limit. Focusing a layout refreshes run status. All five traffic modes, shared datasets, field rules, previews, import/export, recording, and analysis views remain available.

1. Select **Normal**, **Burst**, **Multi-API Burst**, **Weighted (custom percentages)**, or **Coordinated**. Configure duration, bursts per second, users per burst, concurrency, and timeout. Weighted selection is probabilistic, not an exact request-count split. Coordinated and Multi-API Burst run every scenario per user, without ordering or passing responses between scenarios.
2. Set each scenario's URL, HTTP method, percentage, headers, bearer token, and expected statuses. Add multiple APIs and choose any whole-number split, such as 30/30/40 or ten APIs at 10% each. Each percentage must be 1..100 and the total must equal 100%; the live allocation total shows remaining or excess percentage and Start stays disabled until valid. The server independently checks these rules. Percentages apply to Normal, Burst, and Weighted profiles; Coordinated sends every API and does not use a percentage split. The CLI continues to accept relative weights. Upload a local JSON object or edit the request body directly.
3. Optionally upload scenario datasets (JSON arrays of objects), or a shared dataset for coordinated runs. Choose fixed/shared, cycle, or random record selection. Preview uses the first dataset record; actual runs use the configured selection strategy.
4. Choose **Keep value**, **Regex**, or **Expression** for each scalar field, including nested objects and array entries. Rules run in displayed order, after dataset overlay and before built-in placeholders. Use the up/down arrows to put dependencies first.
5. For file placeholders such as `{{file_base64:../fixtures/sample-result.pdf}}`, choose **File (base64)** for that field and select the local file. Existing data-URL prefixes are preserved. Browser uploads cannot read server files or use server token files/environment-variable selectors.
6. Generate a payload preview, confirm authorization, and start the test. Stop cancels scheduling and in-flight client requests; a target server may still finish work it already received. Results include counts, failures, dropped arrivals, p95/p99 latency, and recent requests. Download the report for the full recorded run. Web response capture is limited to 2 KiB per request.
7. Export/import a **Load Lab setup** to preserve configuration, payloads, datasets, and rules together. This bundled format is not a CLI config; its `config` object uses the existing CLI schema, with referenced JSON files contained in `files`. Credential-like headers are excluded by default; explicitly enable **Include credentials in setup export** when needed. Test payload data is still included. Setup and credential data are not saved in browser local storage; Balanced stores pane dimensions only. Failed imports preserve the current setup, and large formatted bundles are accepted up to 16 MiB while compact submissions remain limited to 8 MiB.

External-link icons beside **Payload preview**, **Field rules**, and **Run results** open read-only analysis snapshots in separate tabs. They do not trigger requests or edit testing configuration. Results snapshots contain the latest 100 records; download the full report from the workspace. `/interface-options.html` offers three static interface mockups without replacing the current design or adding testing logic.

Audit fixes include ordered per-API dataset cycling, immutable dataset overlays, matching preview/run record selection, final-rule overlap protection, positive timeout/user validation, explicit redirect/truncation reporting, failed/partial generation outcomes, reconnecting status monitoring, and safe numeric identifier checks. Store large numeric identifiers as strings. CLI global-token behavior remains available; web runs use only explicitly configured request authentication.

Development checks: `go test ./...`, `go vet ./...`, and `node --test internal/web/ui.test.cjs`. Node is not required to run the application.

### Field Rule Examples

Regex generation uses Go/RE2 syntax: character classes, groups, alternation, anchors at the edges, and quantifiers. Lookarounds, backreferences, and word-boundary assertions are not supported. Unbounded quantifiers generate up to 12 extra repetitions, patterns are limited to 2048 bytes, and generation is limited to 4096 steps. Regex rules always produce strings; use an expression for numbers or booleans. Regex matching does not guarantee uniqueness or business validity.

| Field | Source | Rule |
| --- | --- | --- |
| `patient_id` | Regex | `PT-[0-9]{8}` |
| `phone_number` | Regex | `[1-9][0-9]{9}` |
| `email` | Expression | `lower(body.first_name) + "." + lower(body.last_name) + "@example.test"` |
| `amount` | Expression | `body.quantity * body.unit_price` |
| `eligible` | Expression | `body.age >= 18` |
| `ssn_passport` | Expression | `ssn(body.dob, body.gender)` |
| `external_id` | Expression | `"record-" + string(sequence)` |

Expressions use the sandboxed [Expr language](https://expr-lang.org/docs/language-definition), not arbitrary JavaScript or natural-language instructions. `body` contains the overlaid payload and earlier generated fields; `sequence` is a run-local generation counter, and `run_id` is a UUID unique to that generation session. Previews and validation use separate sessions and do not consume the actual run's counter. For literal keys containing dots, use `body["key.with.dots"]`. Expressions should reference fields generated by earlier rules, rather than unresolved built-in placeholders. Existing `{{uuid}}`, `{{timestamp}}`, `{{random_int:N}}`, and other CLI placeholders remain supported.

CLI scenarios can also specify rules directly:

```json
"field_rules": [
  {"path": ["patient_id"], "regex": "PT-[0-9]{8}"},
  {"path": ["items", "0", "amount"], "expression": "body.items[0].quantity * body.items[0].unit_price"}
]
```

The web server binds only to loopback, rejects foreign browser origins, limits uploads to 8 MiB per submission, and allows one active run at a time. Web runs are limited to one hour, 1000 in-flight requests (10,000 for Multi-API Burst), and a one-minute request timeout. Start with low traffic and only use authorized targets.

### Multi-API Burst and Repeated Runs

Select **Multi-API Burst**, add your APIs, and choose a single burst or repeated bursts at a chosen interval. For example, 100 users and 3 APIs produces 300 requests released through one start gate. **Match burst** sets enough concurrency for the group. Requests are prepared before release; a preparation failure sends none of that group. Busy repeat groups are dropped whole rather than sent partially. Arrival times are still subject to network and server scheduling; confirm deadlocks with server/database logs.

For fresh data, mark identifier rules **Unique**. The runner remembers reserved values across actual runs in `os.UserConfigDir()/load-lab/unique-values.jsonl`, using hashed field-path/value pairs and an OS file lock. CLI configurations may override `unique_history_file`; web submissions may not. A rule such as `"record-" + run_id + "-" + string(sequence)` works for flexible string IDs. Regex identifiers retain their format and retry collisions up to 128 times; if no unused value is found, the run fails instead of sending a duplicate. Keep the history between runs. This protection is local, not a query against your API database. Names and unmarked fields may repeat.

Choose **Reuse existing / shared records** to skip Unique rules and preserve template or overlaid IDs for contention tests. Other rules still run. Use a fixed shared dataset and enable overlay on each API to target the same record deliberately.

Preview up to 10 sample bodies per API and download them without consuming persistent Unique values. Enable **Record sent request bodies** to inspect actual sent JSON beside each response or in the report. It is off by default because bodies may contain sensitive data and large files. Reports include run IDs and per-API summaries; rapid repeated reports no longer overwrite each other.

CLI fields: `execution_mode: "multi_api_burst"`, `burst_mode: "once" | "repeat"`, `burst_interval: "1s"` (minimum 1ms), `data_mode: "fresh" | "existing"`, `record_request_bodies: true | false`, and per-rule `unique: true | false`. `rate` remains required but does not control multi-API burst timing. End-user guidance is served at `/user-manual.html`; developer details are in [the reference manual](docs/USER-MANUAL.html#multi-api-development).

This guide explains how to configure, build, run, and share the load tester on Windows. It is written for developers and QA/testing team members.

> **Safety:** Run load tests only against systems you are authorized to test, preferably a dedicated test/staging environment. Start with a low rate. This tool can create database records and upload files; it does not automatically clean them up.

## 1. What this tool does

The tool sends HTTP requests at a configured total request rate. It supports multiple weighted scenarios, a concurrency limit, request timeouts, JSON request templates, optional test-data overlays, bearer-token authentication, base64 file fixtures, expected HTTP status codes, and latency summaries.

The tool currently supports one HTTP request per scenario execution. Multi-step workflows (for example, create a patient, extract its ID, then create an appointment) are not implemented yet.

## 2. Project files and what each one does

```text
go-api-load-tester/
├── cmd/
│   ├── loadtest/                           # CLI entry point
│   └── web/                                # Browser-app server entry point
├── internal/
│   ├── config/                             # Loads and validates test configuration
│   ├── engine/                             # Schedules requests and coordinates runs
│   ├── generator/                          # Builds payloads and applies field rules
│   ├── report/                             # Summaries and per-response recording
│   ├── transport/                          # HTTP client and request handling
│   └── web/
│       ├── server.go                       # Web routes and run handling
│       └── static/                         # Classic and Balanced UI, shared assets
├── configs/
│   ├── examples/                           # Ready-to-edit scenario examples
│   └── patient-import.json                 # Main sample configuration
├── data/                                   # Request templates and test datasets
├── docs/                                   # User guides, scenarios, and design notes
├── fixtures/                               # Sample files used by request payloads
├── .github/workflows/                      # Automated Go checks and Pages publishing
├── go.mod                                  # Go module and direct dependencies
├── go.sum                                  # Dependency checksums
├── LICENSE                                 # MIT license and copyright attribution
└── README.md                               # Setup, usage, and project overview
```

Run-generated response reports are written to `responses/`; that directory is excluded from Git so test output is not published with the source.

### File responsibilities

- `cmd/loadtest/main.go`: reads `-config` and starts the run.
- `internal/config/config.go`: loads the config, request template, and optional dataset.
- `internal/engine/engine.go`: schedules requests, applies scenario weights, limits in-flight requests, and collects results.
- `internal/generator/body.go`: copies the body template, optionally overlays dataset fields, and expands placeholders.
- `internal/transport/client.go`: handles HTTP, headers, bearer token, and timeout.
- `internal/report/report.go`: prints request counts, status counts, average latency, p95 and p99.
- `configs/patient-import.json`: defines target URL, method, total rate, duration, scenario weights and data files.
- `data/patient-request-template.json`: the JSON structure sent to the API.
- `data/test-patients.json`: known test identities for existing-patient scenarios.
- `fixtures/sample-result.pdf`: file fixture embedded as base64 in `document_data.doc_data`.

## 3. Requirements

- Windows PowerShell.
- Go 1.23 or newer installed.
- Access to an approved test/staging API.
- Valid test data that the API accepts.
- If authentication is required, a test bearer token.
- If testing PDF processing, a valid test PDF fixture.

### Check Go installation

Open PowerShell.

```powershell
# Show the installed Go version.
go version

# Show the current Go environment and confirm the toolchain is available.
go env GOVERSION
```

If either command fails, install Go using your organization's approved software process before continuing.

## 4. First-time setup

### Step 1 — Extract the project

Extract `go-api-load-tester.zip` to a folder, for example:

`C:\tools\go-api-load-tester`

Open PowerShell in that folder. The folder you open must contain `go.mod`.

```powershell
# Move to the folder where you extracted the project; update this path if yours differs.
Set-Location "C:\tools\go-api-load-tester"

# Confirm that the Go module file exists in the current directory.
Get-Item .\go.mod
```

### Step 2 — Verify the source code

```powershell
# Format all Go source files.
gofmt -w .\cmd\loadtest\main.go .\internal\config\config.go .\internal\engine\engine.go .\internal\generator\body.go .\internal\transport\client.go .\internal\report\report.go

# Compile and run package checks without sending any API requests.
go test ./...
```

### Step 3 — Build the executable

```powershell
# Build a Windows executable; this command does not start the load test.
go build -o .\loadtest.exe .\cmd\loadtest

# Confirm that the executable was created.
Get-Item .\loadtest.exe
```

The testing team can run `loadtest.exe` without installing Go if you provide the executable and the required `configs`, `data`, and `fixtures` folders. Keep the folder structure intact because configuration paths are relative to the config file.

## 5. Configure the target API

Edit `configs/patient-import.json` before each test.

Example:

```json
{
  "duration": "2m",
  "rate": 1,
  "burst_size": 100,
  "max_inflight": 100,
  "timeout": "45s",
  "scenarios": [
    {
      "name": "new_patient_import",
      "weight": 60,
      "method": "POST",
      "url": "https://YOUR-TEST-HOST/YOUR-ENDPOINT",
      "body_file": "../data/patient-request-template.json",
      "overlay_dataset_keys": false,
      "headers": {
        "Accept": "application/json"
      },
      "expected_statuses": [200, 201, 202]
    },
    {
      "name": "existing_patient_import",
      "weight": 40,
      "method": "POST",
      "url": "https://YOUR-TEST-HOST/YOUR-ENDPOINT",
      "body_file": "../data/patient-request-template.json",
      "dataset_file": "../data/test-patients.json",
      "dataset_mode": "random",
      "overlay_dataset_keys": true,
      "headers": {
        "Accept": "application/json"
      },
      "expected_statuses": [200, 201, 202]
    }
  ]
}
```

Replace `https://YOUR-TEST-HOST/YOUR-ENDPOINT` with the approved test endpoint. Do not point the test at production unless you have explicit authorization and an agreed test plan.

### Configuration options

- `duration`: how long new requests are scheduled, such as `30s`, `2m`, or `10m`.
- `rate`: approximate number of bursts per second across all scenarios. It is not the rate per scenario.
- `burst_size`: number of requests launched in each burst. Set this to `100` to launch 100 users together each second. Defaults to `1`.
- `execution_mode`: `weighted` selects one scenario per virtual user; `coordinated` runs every scenario concurrently for each virtual user.
- `shared_dataset_file`: optional dataset overlaid into every scenario in a coordinated virtual-user group.
- `shared_dataset_mode`: `fixed` always uses the first record, `random` selects a record per virtual user, and `cycle` rotates records.
- `max_inflight`: maximum number of concurrent HTTP requests.
- `timeout`: maximum duration for one HTTP request.
- `scenarios`: list of request scenarios.
- `name`: unique scenario name used in the report.
- `weight`: relative probability of selecting this scenario. Weights `60` and `40` mean roughly 60% and 40%, not exact counts in a short test.
- `method`: HTTP method, such as `GET`, `POST`, `PUT`, `PATCH`, or `DELETE`.
- `url`: full target URL.
- `body_file`: JSON object template to send.
- `dataset_file`: optional JSON array of records to reuse.
- `dataset_mode`: currently records are selected randomly. `cycle` is accepted but currently also selects randomly.
- `overlay_dataset_keys`: when `true`, replace matching template keys with values from the selected dataset record.
- `headers`: extra request headers.
- `expected_statuses`: status codes considered successful by the report. If omitted, only 2xx responses count as successful.

### Coordinated concurrency and lock-contention tests

Use `execution_mode: "coordinated"` when multiple actors, roles, endpoints, or
methods must operate on the same entity at the same time. Scenario names are
arbitrary; this mode is not specific to any particular API or project.

```json
{
  "duration": "2m",
  "rate": 1,
  "burst_size": 100,
  "max_inflight": 200,
  "execution_mode": "coordinated",
  "shared_dataset_file": "../data/test-patients.json",
  "shared_dataset_mode": "fixed",
  "scenarios": [
    {
      "name": "actor_a_update",
      "weight": 1,
      "method": "PUT",
      "url": "https://API-A/update",
      "body_file": "../data/patient-request-template.json",
      "overlay_dataset_keys": true,
      "token_env": "API_A_TOKEN",
      "token_file": "../secrets/api-a.token"
    },
    {
      "name": "actor_b_update",
      "weight": 1,
      "method": "PATCH",
      "url": "https://API-B/update",
      "body_file": "../data/patient-request-template.json",
      "overlay_dataset_keys": true,
      "token_env": "API_B_TOKEN",
      "token_file": "../secrets/api-b.token"
    }
  ]
}
```

Each of the 100 virtual users schedules both scenarios with the same shared
record, producing 200 requests per burst. `fixed` makes all virtual users
contend on the first record. Use `random` or `cycle` when each virtual-user
group should share a record but different groups may target different records.

Set `max_inflight` to at least `burst_size × number of scenarios` if the full
coordinated burst must start without scheduler drops. Scenario `weight` is
used only in `weighted` mode.

Ready-to-edit examples:

- [Weighted create/update 60/40](configs/examples/weighted-create-update-60-40.json) with [short instructions](docs/scenarios/weighted-create-update-60-40.md)
- [Single API 100-request burst](configs/examples/single-api-100-burst.json) with [short instructions](docs/scenarios/single-api-100-burst.md)
- [100 users targeting one shared record](configs/examples/100-users-one-record.json) with [short instructions](docs/scenarios/100-users-one-record.md)
- [100 users targeting many shared records](configs/examples/100-users-many-records.json) with [short instructions](docs/scenarios/100-users-many-records.md)
- [Coordinated calls with cycling records](configs/examples/coordinated-cycle-records.json) with [short instructions](docs/scenarios/coordinated-cycle-records.md)
- [Three roles targeting one record](configs/examples/coordinated-three-roles-one-record.json) with [short instructions](docs/scenarios/coordinated-three-roles-one-record.md)

## 6. How request-body templates work

Edit `data/patient-request-template.json` to match the API contract exactly. The provided template is only an example based on the patient/result-import payload discussed with the developer; verify every field and data type against the real API.

### Supported placeholders

Placeholders are used inside JSON string values.

| Placeholder | Result |
|---|---|
| `{{uuid}}` | New UUID |
| `{{timestamp}}` | Current UTC timestamp in RFC3339Nano format |
| `{{sequence}}` | Increasing body-build sequence number |
| `{{random_int:8}}` | Eight random numeric digits |
| `{{random_string:10}}` | Ten random alphanumeric characters |
| `{{random_letters:10}}` | Ten random alphabetic characters |
| `{{ssn}}` | Fifteen-digit SSN compatible with the body's `dob` and `gender` |
| `{{file_base64:../fixtures/sample-result.pdf}}` | Reads the file and inserts its base64 content |

Example values in a JSON template:

```json
{
  "first_name": "LOAD {{random_letters:8}}",
  "birth_name": "BIRTH {{random_letters:8}}",
  "email": "loadtest+{{uuid}}@example.test",
  "external_patient_id": "patient-{{uuid}}",
  "request_number": "req-{{uuid}}",
  "document_data": [
    {
      "doc_type": "result",
      "doc_data": "data:application/pdf;base64,{{file_base64:../fixtures/sample-result.pdf}}"
    }
  ]
}
```

Use placeholders only where the API accepts generated values. For instance, a generated phone number must still satisfy the API's phone-number validation rules.

### PDF fixture path

The `file_base64` path is resolved relative to the folder containing `configs/patient-import.json`, not the current PowerShell directory. The example path `../fixtures/sample-result.pdf` therefore points to the project's `fixtures` folder.

The example uses base64 embedded in JSON. It is not a multipart/form-data upload. Use the format required by your endpoint.

## 7. How existing-patient dataset replacement works

`data/test-patients.json` must be a JSON array of objects.

Example:

```json
[
  {
    "external_patient_id": "LOAD_PATIENT_001",
    "first_name": "TEST PATIENT A",
    "last_name": "LoadTest",
    "birth_name": "TEST BIRTH A",
    "dob": "1995-04-12",
    "gender": "male",
    "email": "patient001@example.test",
    "phone_number": "{{random_int:14}}",
    "ssn_passport": "{{ssn}}"
  }
]
```

Replace the example values with authorized test records accepted by your API.

When `overlay_dataset_keys` is `true`:

1. The tool copies the request template.
2. It selects one dataset record.
3. It replaces values only for keys that already exist in the request template.
4. Extra dataset keys not present in the template are ignored.
5. The resulting JSON body is sent to the API.

For example, if `email` exists in both the template and dataset record, the dataset's email replaces the template email. If `external_patient_id` exists in the dataset but not in the template, it will **not** be added. Add every field you need to send to the template.

For a new-patient scenario, use `overlay_dataset_keys: false` and generate unique identity fields in the template. For an existing-patient scenario, use `overlay_dataset_keys: true` and include all required matching fields in both the template and dataset. Keep `external_patient_id` stable per patient and generate a new `request_number` for each new result import if that matches the API's rules.

## 8. Authentication

Each scenario can use its own bearer token. Configure `token_env` and
`token_file` together: the environment variable is used when it has a value,
otherwise the token is read from the file. Token-file paths are relative to
the configuration file.

```json
{
  "name": "patient_api",
  "token_env": "PATIENT_API_TOKEN",
  "token_file": "../secrets/patient-api.token"
}
```

```json
{
  "name": "billing_api",
  "token_env": "BILLING_API_TOKEN",
  "token_file": "../secrets/billing-api.token"
}
```

Each token file should contain only the JWT, with or without the `Bearer`
prefix. The `secrets` folder is ignored by Git. Do not commit or share it.

```powershell
# Optional: environment values override the corresponding token files.
$env:PATIENT_API_TOKEN = "PATIENT_API_JWT"
$env:BILLING_API_TOKEN = "BILLING_API_JWT"
```

When a scenario has neither `token_env` nor `token_file`, the existing global
`API_TOKEN` environment variable remains the fallback. A literal
`Authorization` value in `headers` is also supported, but is discouraged
because it stores the credential in the configuration. Do not combine a
literal `Authorization` header with `token_env` or `token_file`.

If the API uses a different authentication method, configure the required header in the scenario's `headers` object, or extend the tool for that authentication scheme. Avoid putting real credentials in scripts or shared config files.

## 9. Run the test

Before running, verify the target URL, method, request body, test identities, PDF fixture, authentication, expected status codes, and approval to generate traffic.

```powershell
# Run the test using the scenario configuration.
.\loadtest.exe -config .\configs\patient-import.json
```

Application logs are written as structured JSON to stderr. The default `info` level records startup, configuration, and completion; `warn` and `error` events include failed requests and body-generation errors. Use `-log-level debug` to log every successful request as well:

```powershell
# Enable per-request details and save stderr logs to a file.
.\loadtest.exe -config .\configs\patient-import.json -log-level debug 2> .\loadtest.log
```

To inspect response bodies, use `-log-response-body`. This logs status, content type, and up to 4 KiB of each response body (including successful responses). Response bodies may contain sensitive data; enable this only when necessary and protect the log file.

```powershell
.\loadtest.exe -config .\configs\patient-import.json -log-response-body 2> .\loadtest.log
```

To print the complete generated request body exactly as sent, use `-log-request-body`. This logs every request body in full, including generated placeholder values and dataset overlays. Request bodies may contain sensitive data; enable this only when necessary and protect the log file.

```powershell
.\loadtest.exe -config .\configs\patient-import.json -log-request-body 2> .\loadtest.log
```

Supported levels are `debug`, `info`, `warn`, and `error`. Unless the corresponding body-logging option is enabled, logs omit request bodies; headers, bearer tokens, and target URLs are not logged.

### Response file per run

Every run writes all responses to a new JSON file in the `responses` folder, named `responses_<timestamp>.json`. No existing file is overwritten.

```powershell
# Default: responses\responses_20260124-104649.json
.\loadtest.exe -config .\configs\patient-import.json

# Write the file somewhere else.
.\loadtest.exe -config .\configs\patient-import.json -response-dir .\my-results

# Disable response recording.
.\loadtest.exe -config .\configs\patient-import.json -response-dir ""
```

The file holds one entry per response plus a run summary:

```json
{
  "started_at": "2026-01-24T10:46:49.9459166+05:30",
  "responses": [
    {
      "index": 1,
      "scenario": "new_patient_import",
      "method": "POST",
      "url": "https://example.test/api/v1/fr/ext/patient",
      "started_at": "2026-01-24T10:46:49.9459166+05:30",
      "duration_ms": 85.292,
      "status": 200,
      "content_type": "application/json",
      "expected": true,
      "body": { "patient_id": "4ffadaa4-4d50-4056-aaba-0122fc9d9a72", "is_error": false },
      "body_bytes": 67
    }
  ],
  "response_count": 1,
  "summary": {
    "elapsed_ms": 4002.1,
    "completed": 1,
    "failed": 0,
    "network_errors": 0,
    "dropped_arrivals": 0,
    "body_generation_errors": 0,
    "statuses": { "200": 1 }
  }
}
```

JSON responses are embedded as JSON under `body`. Non-JSON responses, such as an HTML error page, are stored as text under `body_text`. Transport captures at most 2 MiB per response by default; use `-max-transport-response-body` to change this limit, or `0` to discard response bodies. `-max-response-body` applies a separate per-record storage limit and cannot increase the transport capture limit. Web runs cap capture and report storage at 2 KiB per response. Truncated entries set `body_truncated`; `body_bytes` reports captured bytes, and `declared_body_bytes` reports the server's declared length when available. Response files may contain sensitive data; store and share them accordingly.

```powershell
# Record all captured body bytes (default; transport cap is 2 MiB)
.\loadtest.exe -config .\configs\patient-import.json

# Opt in to a 64 KiB per-body cap
.\loadtest.exe -config .\configs\patient-import.json -max-response-body 65536

# Also limit transport memory capture to 64 KiB per response
.\loadtest.exe -config .\configs\patient-import.json -max-response-body 65536 -max-transport-response-body 65536
```

Latency percentiles use a fixed-memory HDR histogram with approximately three significant digits. Values from 1 microsecond through one year are tracked; values above that range are capped.

Example conservative progression:

| Run | Total rate | Duration | Max in-flight |
|---|---:|---:|---:|
| 1 — baseline | 1 RPS | 2 minutes | 10 |
| 2 — low load | 2 RPS | 5 minutes | 20 |
| 3 — moderate | 5 RPS | 5 minutes | 50 |
| 4 — higher | 10 RPS | 5 minutes | 100 |

For a synchronized user burst, use the same or a larger `max_inflight` than
`burst_size`. For example, `"rate": 1`, `"burst_size": 100`, and
`"max_inflight": 100` launches approximately 100 requests together every
second. If `max_inflight` is smaller than `burst_size`, the excess arrivals
are dropped and reported in `Dropped arrivals`.

Change these values in the config file between runs. These are starting test levels, not a guarantee that the service can sustain them. Stop if errors rise, latency keeps increasing, the DB connection pool is exhausted, or the service becomes unstable.

## 10. Understand the report

- `Completed`: HTTP requests that returned or errored during the run and were collected by the tool.
- `Failed`: network/request errors or responses whose status was not in `expected_statuses`.
- `Network/request errors`: requests that could not complete successfully at the HTTP client layer.
- `Average latency`: average duration for collected requests.
- `P95 latency`: 95% of collected requests completed at or below this latency.
- `P99 latency`: 99% of collected requests completed at or below this latency.
- `HTTP statuses`: count per returned HTTP status.
- `Dropped arrivals`: scheduled request arrivals skipped because all concurrency slots were occupied. These are not sent requests.
- `Body generation errors`: requests skipped because the template/fixture could not be processed.

The rate scheduler is approximate. At high rates or with slow responses, monitor dropped arrivals as well as completed throughput. A load test result is not valid if requests are silently being skipped or the generated payload is invalid.

## 11. Share with the testing team

For a team member who does not have Go installed, provide:

- `loadtest.exe` built for Windows.
- The complete `configs` folder.
- The complete `data` folder.
- The complete `fixtures` folder.
- This README.
- The approved test URL, test data instructions, and a secure method to obtain a test token.

Keep the directory layout unchanged. Do not include production credentials or sensitive patient data in the ZIP. Each tester should use their own approved test token.

To rebuild after source changes, the developer needs Go installed and runs:

```powershell
# Run package checks after changing the code.
go test ./...

# Rebuild the Windows executable after changing the code.
go build -o .\loadtest.exe .\cmd\loadtest
```

## 12. Current limitations

- One HTTP request per scenario execution; no dependent multi-step workflows or response-field extraction yet.
- `dataset_mode: cycle` currently behaves like random selection.
- Dataset overlay only replaces keys already present in the request template.
- File embedding supports base64-in-JSON; multipart/form-data upload is not implemented.
- No automatic cleanup of records or uploaded files.
- No arbitrary regex-based data generation.
- The tool measures HTTP behavior; it does not diagnose database locks, deadlocks, or server-side bottlenecks by itself. Monitor application logs, PostgreSQL, CPU, memory, and connection pools during testing.
