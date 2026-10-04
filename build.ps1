# 在项目根目录生成 tsukiyo-master / tsukiyo-agent
$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

Write-Host "Building tsukiyo-master..."
Push-Location (Join-Path $Root "master")
go build -o (Join-Path $Root "tsukiyo-master.exe") ./cmd/master
Pop-Location

Write-Host "Building tsukiyo-agent (linux amd64)..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
Push-Location (Join-Path $Root "agent")
go build -ldflags="-s -w" -o (Join-Path $Root "tsukiyo-agent") ./cmd/agent
Pop-Location
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "Done:"
Get-Item (Join-Path $Root "tsukiyo-master.exe"), (Join-Path $Root "tsukiyo-agent") | Format-Table Name, Length, LastWriteTime
