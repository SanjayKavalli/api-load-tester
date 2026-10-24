# 100 virtual users: one shared record

Use this scenario to send both API calls against the first dataset record.

```json
"shared_dataset_mode": "fixed"
```

## Files

- Config: `configs/examples/100-users-one-record.json`
- Shared data: `data/test-patients.json`
- Request bodies: change each scenario's `body_file` when the APIs need different bodies.

## Edit

1. Replace both `url` values.
2. Set the correct `method` for each API.
3. Set each `body_file`.
4. Keep `overlay_dataset_keys` as `true`.
5. Add more scenarios when more actors must run together.
6. Set `max_inflight` to at least `burst_size × scenario count`.

## Tokens

Create local token files:

```powershell
New-Item -ItemType Directory -Path .\secrets -Force
Set-Content .\secrets\api-a.token "<API-A-JWT>"
Set-Content .\secrets\api-b.token "<API-B-JWT>"
```

Optional environment values override the files:

```powershell
$env:API_A_TOKEN = "<API-A-JWT>"
$env:API_B_TOKEN = "<API-B-JWT>"
```

## Run

```powershell
.\loadtest.exe `
  -config .\configs\examples\100-users-one-record.json `
  -log-request-body=false `
  -log-response-body=false
```

## Response

Open the newest file in `responses`:

```powershell
Get-ChildItem .\responses\*.json |
  Sort-Object LastWriteTime -Descending |
  Select-Object -First 1
```

Check `summary`, `status`, `duration_ms`, `error`, and `body`.
