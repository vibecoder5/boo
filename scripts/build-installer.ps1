# Собирает boo.exe без консоли и setup.exe с вложением.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Read-BooVersion {
    $text = Get-Content -Raw -Path (Join-Path $root "CHANGELOG.md")
    $match = [regex]::Match($text, '## \[(\d+\.\d+\.\d+)\]')
    if (-not $match.Success) {
        throw "в CHANGELOG.md нет заголовка версии"
    }
    return $match.Groups[1].Value
}

$version = Read-BooVersion
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

Write-Host "версия $version"
go run .\scripts\genicon.go
if (Get-Command go -ErrorAction SilentlyContinue) {
    $icon = Join-Path $root "install\icon.ico"
    if (Test-Path $icon) {
        $rsrc = "github.com/akavel/rsrc@v0.10.2"
        try {
            go run $rsrc -ico $icon -arch amd64 -o (Join-Path $root "rsrc_windows_amd64.syso")
            go run $rsrc -ico $icon -arch amd64 -o (Join-Path $root "cmd\installer\rsrc_windows_amd64.syso")
        } catch {
            Write-Host "иконка в exe пропущена: $_"
        }
    }
}

$ld = "-X boo/internal/install.Version=$version"
go test ./internal/install ./cmd/installer
go build -ldflags "-H windowsgui -s -w $ld" -o (Join-Path $dist "boo.exe") .
go build -ldflags "-H windowsgui -s -w $ld" -o (Join-Path $dist "uninstall.exe") ./cmd/installer
& (Join-Path $dist "uninstall.exe") -pack -bin (Join-Path $dist "boo.exe") -out (Join-Path $dist "boo-setup.exe")
if ($LASTEXITCODE -ne 0) {
    throw "не удалось упаковать setup"
}

Remove-Item -ErrorAction SilentlyContinue (Join-Path $root "rsrc_windows_amd64.syso")
Remove-Item -ErrorAction SilentlyContinue (Join-Path $root "cmd\installer\rsrc_windows_amd64.syso")

$setupPath = Join-Path $dist "boo-setup.exe"
$setup = $null
foreach ($i in 1..8) {
    try {
        $setup = [System.IO.File]::ReadAllBytes($setupPath)
        break
    } catch {
        Start-Sleep -Milliseconds 200
    }
}
if ($null -eq $setup) {
    throw "не удалось прочитать $setupPath"
}
$magic = [System.Text.Encoding]::ASCII.GetString($setup[($setup.Length - 24)..($setup.Length - 17)])
if ($magic -ne "BOOINST1") {
    throw "setup.exe без вложения (магия $magic)"
}

Write-Host "готово:"
Write-Host "  dist\boo.exe"
Write-Host "  dist\boo-setup.exe"
Write-Host "тихая установка: .\dist\boo-setup.exe -silent"
