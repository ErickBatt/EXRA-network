# PowerShell скрипт для компиляции сервера для Linux

Write-Host "🔨 Компиляция Go сервера для Linux..." -ForegroundColor Cyan

Set-Location server

$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Write-Host "   GOOS=linux GOARCH=amd64" -ForegroundColor Gray

go build -o exra-server-linux .

if (Test-Path "exra-server-linux") {
    $size = (Get-Item "exra-server-linux").Length / 1MB
    Write-Host "✅ Сервер скомпилирован: exra-server-linux ($([math]::Round($size, 2)) MB)" -ForegroundColor Green
    
    Write-Host ""
    Write-Host "📋 Следующие шаги:" -ForegroundColor Yellow
    Write-Host "   1. Загрузите файл на сервер:"
    Write-Host "      scp server/exra-server-linux root@103.6.168.174:/root/exra/server/" -ForegroundColor White
    Write-Host ""
    Write-Host "   2. На сервере выполните:"
    Write-Host "      cd /root/exra/server" -ForegroundColor White
    Write-Host "      chmod +x exra-server-linux" -ForegroundColor White
    Write-Host "      pkill -f exra-server || true" -ForegroundColor White
    Write-Host "      nohup ./exra-server-linux > server.log 2>&1 &" -ForegroundColor White
    Write-Host ""
    Write-Host "   3. Проверьте TMA: https://app.exra.space" -ForegroundColor White
} else {
    Write-Host "❌ Ошибка компиляции!" -ForegroundColor Red
    exit 1
}

Set-Location ..
