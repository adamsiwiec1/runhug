# Install the latest runhug Windows amd64 release from adamsiwiec1/runhug-cli.
# Asset is a bare .exe: runhug-cli_<ver>_windows_amd64.exe → runhug.exe
# Usage: irm https://raw.githubusercontent.com/adamsiwiec1/runhug-cli/main/scripts/install.ps1 | iex
$ErrorActionPreference = "Stop"
$Repo = "adamsiwiec1/runhug-cli"
$AssetPrefix = "runhug-cli_"
$BinName = "runhug.exe"

$api = "https://api.github.com/repos/$Repo/releases/latest"
$headers = @{ Accept = "application/vnd.github+json"; "User-Agent" = "runhug-install" }
$release = Invoke-RestMethod -Uri $api -Headers $headers
$tag = $release.tag_name
$ver = $tag.TrimStart("v")
$assetName = "${AssetPrefix}${ver}_windows_amd64.exe"
$asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) {
  throw "Asset not found: $assetName (tag $tag)"
}

$destDir = Join-Path $env:LOCALAPPDATA "runhug\bin"
New-Item -ItemType Directory -Force -Path $destDir | Out-Null
$dest = Join-Path $destDir $BinName

Write-Host "Downloading $($asset.browser_download_url)"
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $dest

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not ($userPath -split ";" | Where-Object { $_ -eq $destDir })) {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$destDir", "User")
  $env:Path = "$env:Path;$destDir"
  Write-Host "Added $destDir to your user PATH (new shells pick this up)."
}

Write-Host "Installed $BinName → $dest ($tag)"
try { & $dest --version } catch { try { & $dest version } catch {} }
