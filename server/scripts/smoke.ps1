param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$ProxySecret = "secret_token_for_buyers"
)

$ErrorActionPreference = "Stop"

Write-Host "1) Health check"
$health = Invoke-RestMethod -Method Get -Uri "$BaseUrl/health"
$health | ConvertTo-Json -Depth 5

Write-Host "1.1) Public nodes and stats"
$nodes = Invoke-RestMethod -Method Get -Uri "$BaseUrl/nodes"
$stats = Invoke-RestMethod -Method Get -Uri "$BaseUrl/nodes/stats"
$nodes | ConvertTo-Json -Depth 5
$stats | ConvertTo-Json -Depth 5

Write-Host "2) Register buyer"
$buyer = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/buyer/register" -Headers @{
    Authorization = "Bearer $ProxySecret"
}
$buyer | ConvertTo-Json -Depth 5
$buyerApiKey = $buyer.api_key
if (-not $buyerApiKey) {
    throw "Buyer API key is missing"
}

$buyerHeaders = @{ "X-Exra-Token" = $buyerApiKey }

Write-Host "3) Top up buyer balance"
$topupBody = @{ amount_usd = 5 } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/buyer/topup" -Headers $buyerHeaders -Body $topupBody -ContentType "application/json" | Out-Null

Write-Host "4) Start session"
$sessionStart = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/session/start" -Headers $buyerHeaders
$sessionStart | ConvertTo-Json -Depth 5
$sessionId = $sessionStart.session_id
if (-not $sessionId) {
    throw "Session ID missing from start response"
}

Write-Host "5) End session"
$sessionEnd = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/session/$sessionId/end" -Headers $buyerHeaders
$sessionEnd | ConvertTo-Json -Depth 5

Write-Host "6) End session again (idempotency check)"
$sessionEndAgain = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/session/$sessionId/end" -Headers $buyerHeaders
$sessionEndAgain | ConvertTo-Json -Depth 5

Write-Host "7) Fetch profile and sessions"
$buyerProfile = Invoke-RestMethod -Method Get -Uri "$BaseUrl/api/buyer/me" -Headers $buyerHeaders
$sessions = Invoke-RestMethod -Method Get -Uri "$BaseUrl/api/buyer/sessions" -Headers $buyerHeaders

$buyerProfile | ConvertTo-Json -Depth 5
$sessions | ConvertTo-Json -Depth 5

Write-Host "Smoke checks completed."
