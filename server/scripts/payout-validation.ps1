param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$ProxySecret = "secret_token_for_buyers",
    [string]$DeviceId = "sim-node-001",
    [string]$Wallet = "11111111111111111111111111111111"
)

$ErrorActionPreference = "Stop"
$headers = @{ "X-Exra-Token" = $ProxySecret; "Content-Type" = "application/json" }

Write-Host "1) Small payout precheck (0.05 USD)"
$small = @{ device_id = $DeviceId; amount_usd = 0.05; recipient_wallet = $Wallet } | ConvertTo-Json
try {
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/payout/precheck" -Headers $headers -Body $small | ConvertTo-Json -Depth 6
} catch {
    Write-Host $_.Exception.Message
}

Write-Host "2) Tiny payout precheck (0.005 USD)"
$tiny = @{ device_id = $DeviceId; amount_usd = 0.005; recipient_wallet = $Wallet } | ConvertTo-Json
try {
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/payout/precheck" -Headers $headers -Body $tiny | ConvertTo-Json -Depth 6
} catch {
    Write-Host $_.Exception.Message
}

Write-Host "3) Request payout"
$request = @{ device_id = $DeviceId; amount_usd = 0.05; recipient_wallet = $Wallet } | ConvertTo-Json
try {
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/payout/request" -Headers $headers -Body $request | ConvertTo-Json -Depth 6
} catch {
    Write-Host $_.Exception.Message
}
