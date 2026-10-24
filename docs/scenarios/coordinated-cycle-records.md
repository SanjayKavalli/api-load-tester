# Coordinated calls: cycle records

Use this when every virtual user must call every API, while records rotate.

```json
"execution_mode": "coordinated",
"shared_dataset_mode": "cycle"
```

## Edit

1. Add records to `data/test-patients.json`.
2. Replace each `url`.
3. Set methods, bodies, and status codes.
4. Add or remove scenarios.
5. Set `max_inflight` to `burst_size × scenario count`.

## Tokens

```powershell
New-Item -ItemType Directory .\secrets -Force
Set-Content .\secrets\service-a.token "<SERVICE-A-JWT>"
Set-Content .\secrets\service-b.token "<SERVICE-B-JWT>"
```

Optional overrides:

```powershell
$env:SERVICE_A_TOKEN = "<SERVICE-A-JWT>"
$env:SERVICE_B_TOKEN = "<SERVICE-B-JWT>"
```

## Run and response

```powershell
.\loadtest.exe -config .\configs\examples\coordinated-cycle-records.json -log-response-body=false
```

Open the newest `responses\responses_*.json` file.
