param(
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"

New-Item -ItemType Directory -Force -Path $dist | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $dist "agit-*")

$targets = @(
    @{ GOOS = "linux";   GOARCH = "amd64"; Output = "agit-linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Output = "agit-linux-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Output = "agit-darwin-amd64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Output = "agit-darwin-arm64" },
    @{ GOOS = "windows"; GOARCH = "amd64"; Output = "agit-windows-amd64.exe" }
)

foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"

    $output = Join-Path $dist $target.Output
    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $output ./cmd/agit

    $hash = Get-FileHash -Algorithm SHA256 $output
    "$($hash.Hash.ToLower())  $($target.Output)" | Set-Content -NoNewline -Encoding ascii "$output.sha256"
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "Built release assets in $dist"
