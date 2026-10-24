# 100 virtual users: many shared records

Use this scenario when each virtual-user group may select a different record.
All API calls inside one group still use the same selected record.

```json
"shared_dataset_mode": "random"
```

Use predictable rotation instead:

```json
"shared_dataset_mode": "cycle"
```

## Files

- Config: `configs/examples/100-users-many-records.json`
- Shared data: `data/test-patients.json`
- Request bodies: change each scenario's `body_file` when the APIs need different bodies.

## Edit

1. Add all test records to `data/test-patients.json`.
2. Replace both `url` values.
3. Set the correct `method` for each API.
4. Set each `body_file`.
5. Keep `overlay_dataset_keys` as `true`.
6. Add more scenarios when more actors must run together.
7. Set `max_inflight` to at least `burst_size × scenario count`.

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
  -config .\configs\examples\100-users-many-records.json `
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
