# Install the latest runhug Windows amd64 release from adamsiwiec1/runhug.
# Next release asset: runhug_<ver>_windows_amd64.exe → runhug.exe
# Fallback: older tags may still publish runhug-cli_<ver>_windows_amd64.exe
# Usage: irm https://raw.githubusercontent.com/adamsiwiec1/runhug/main/scripts/install.ps1 | iex
$ErrorActionPreference = "Stop"
$Repo = "adamsiwiec1/runhug"
$AssetPrefixes = @("runhug_", "runhug-cli_")
$BinName = "runhug.exe"

$api = "https://api.github.com/repos/$Repo/releases/latest"
$headers = @{ Accept = "application/vnd.github+json"; "User-Agent" = "runhug-install" }
$release = Invoke-RestMethod -Uri $api -Headers $headers
$tag = $release.tag_name
$ver = $tag.TrimStart("v")

$asset = $null
foreach ($prefix in $AssetPrefixes) {
  $assetName = "${prefix}${ver}_windows_amd64.exe"
  $asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
  if ($asset) { break }
}
if (-not $asset) {
  throw "Asset not found for prefixes $($AssetPrefixes -join ', ') (tag $tag)"
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
