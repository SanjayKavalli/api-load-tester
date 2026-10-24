# Single API: 100 requests together

Use this for one endpoint.

```json
"burst_size": 100,
"max_inflight": 100
```

## Edit

1. Replace `url`.
2. Set `method`, `body_file`, and `expected_statuses`.

## Token

```powershell
New-Item -ItemType Directory .\secrets -Force
Set-Content .\secrets\resource-api.token "<JWT>"
```

Optional override:

```powershell
$env:RESOURCE_API_TOKEN = "<JWT>"
```

## Run and response

```powershell
.\loadtest.exe -config .\configs\examples\single-api-100-burst.json -log-response-body=false
```

Open the newest `responses\responses_*.json` file.
