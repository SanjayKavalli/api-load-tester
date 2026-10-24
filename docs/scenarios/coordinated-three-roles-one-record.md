# Coordinated calls: three roles, one record

Use this for maximum contention between three roles or APIs.

```json
"execution_mode": "coordinated",
"shared_dataset_mode": "fixed"
```

Each virtual user sends all three scenarios against the first record.

## Edit

1. Replace each `url`.
2. Set each method, body, and status list.
3. Add or remove roles.
4. Keep `max_inflight` at least `burst_size × 3`.

## Tokens

```powershell
New-Item -ItemType Directory .\secrets -Force
Set-Content .\secrets\role-a.token "<ROLE-A-JWT>"
Set-Content .\secrets\role-b.token "<ROLE-B-JWT>"
Set-Content .\secrets\role-c.token "<ROLE-C-JWT>"
```

Optional overrides:

```powershell
$env:ROLE_A_TOKEN = "<ROLE-A-JWT>"
$env:ROLE_B_TOKEN = "<ROLE-B-JWT>"
$env:ROLE_C_TOKEN = "<ROLE-C-JWT>"
```

## Run and response

```powershell
.\loadtest.exe -config .\configs\examples\coordinated-three-roles-one-record.json -log-response-body=false
```

Open the newest `responses\responses_*.json` file.
