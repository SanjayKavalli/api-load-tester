# Weighted create and update: 60/40

Use this for normal mixed traffic.

```json
"execution_mode": "weighted"
```

`create_patient` receives about 60% of selected requests.
`update_patient` receives about 40%.

## Edit

1. Replace both `url` values.
2. Set the correct methods and status codes.
3. Change `body_file` if the APIs use different bodies.
4. Update `data/test-patients.json` for update records.

## Token

```powershell
New-Item -ItemType Directory .\secrets -Force
Set-Content .\secrets\patient-api.token "<JWT>"
```

Optional override:

```powershell
$env:PATIENT_API_TOKEN = "<JWT>"
```

## Run and response

```powershell
.\loadtest.exe -config .\configs\examples\weighted-create-update-60-40.json -log-response-body=false
```

Open the newest `responses\responses_*.json` file.
