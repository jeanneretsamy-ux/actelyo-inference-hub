$ErrorActionPreference = 'Stop'
$env:ACTELYO_HUB_HOME = Join-Path $PSScriptRoot 'legalya-hub-data'
$env:ACTELYO_HUB_BIND_HOST = '127.0.0.1'
$hubExecutable = Join-Path $PSScriptRoot 'actelyohub-windows.exe'
$keyPath = Join-Path $env:ACTELYO_HUB_HOME 'cle-pilotage.txt'
New-Item -ItemType Directory -Path $env:ACTELYO_HUB_HOME -Force | Out-Null
if (-not (Test-Path -LiteralPath $keyPath)) {
  $keyBytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($keyBytes) } finally { $rng.Dispose() }
  $managementKey = [Convert]::ToBase64String($keyBytes)
  Set-Content -LiteralPath $keyPath -Value $managementKey -NoNewline
  $currentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
  & icacls.exe $env:ACTELYO_HUB_HOME /inheritance:r /grant:r "${currentUser}:(OI)(CI)F" | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Impossible de proteger le dossier de configuration.' }
  & $hubExecutable set-web-key $managementKey | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Impossible de definir la cle de pilotage.' }
}
$managementKey = (Get-Content -LiteralPath $keyPath -Raw).Trim()
$headers = @{Authorization = ('Bearer ' + $managementKey)}
$base = 'http://127.0.0.1:8090'
$lms = Join-Path $env:USERPROFILE '.lmstudio\bin\lms.exe'
try { $models = Invoke-RestMethod 'http://127.0.0.1:1234/v1/models' -TimeoutSec 3 } catch {
  if (-not (Test-Path -LiteralPath $lms)) { throw 'LM Studio est requis pour LegalYA V30.' }
  & $lms server start --port 1234 | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Impossible de demarrer LM Studio.' }
  $models = Invoke-RestMethod 'http://127.0.0.1:1234/v1/models' -TimeoutSec 10
}
try { $status = Invoke-RestMethod ($base + '/api/status') -Headers $headers -TimeoutSec 2 } catch {
  # Ne pas modifier la cle d'une autre instance utilisant deja ce port.
  try { $occupied = Invoke-WebRequest $base -TimeoutSec 2 -UseBasicParsing } catch { $occupied = $null }
  if ($occupied) { throw 'Le port 8090 est deja utilise par une autre instance.' }
  $process = Start-Process -FilePath $hubExecutable -ArgumentList @('web', '8090') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $env:ACTELYO_HUB_HOME 'hub.log') -RedirectStandardError (Join-Path $env:ACTELYO_HUB_HOME 'hub-errors.log')
  for ($attempt = 0; $attempt -lt 30; $attempt++) {
    try { $status = Invoke-RestMethod ($base + '/api/status') -Headers $headers -TimeoutSec 2; break } catch { Start-Sleep -Milliseconds 300 }
  }
  if (-not $status) { throw 'Le hub ne repond pas.' }
}
function Hub-Post($path, $payload) {
  Invoke-RestMethod ($base + $path) -Method Post -Headers $headers -ContentType 'application/json; charset=utf-8' -Body ([System.Text.Encoding]::UTF8.GetBytes(($payload | ConvertTo-Json -Depth 12))) -TimeoutSec 60
}
$presetName = 'LegalYA V30 - local LM Studio'
$presets = @(Invoke-RestMethod ($base + '/api/presets') -Headers $headers)
$existing = $presets | Where-Object { $_.name -eq $presetName } | Select-Object -First 1
$presetId = if ($existing) { $existing.id } else { '' }
# Preserve measured sampling and context instead of overwriting the preset.
if ($existing) { $saved = @{ok=$true;id=$existing.id} } else {
  $saved = Hub-Post '/api/preset/external/save' @{id='';name=$presetName;url='http://127.0.0.1:1234/v1';model='legalya-v30';ctx='8192';vision=$false;key='';keyTouched=$true}
}
if (-not $saved.ok) { throw 'Enregistrement du preset impossible.' }
$preset = Invoke-RestMethod ($base + '/api/preset?id=' + $saved.id) -Headers $headers
$desiredContext = 8192
if ($preset.content -match '(?m)^CTX=(\d+)') { $desiredContext = [int]$Matches[1] }
$loaded = & $lms ps --json | ConvertFrom-Json
if (-not ($loaded | Where-Object { $_.identifier -eq 'legalya-v30' })) {
  & $lms load legalya-v30 --identifier legalya-v30 --context-length $desiredContext --parallel 1 --yes | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Impossible de charger LegalYA V30.' }
}
if (-not $preset.sysprompt) {
  $promptSaved = Hub-Post '/api/preset/save' @{id=$saved.id;name=$presetName;content=$preset.content;sysprompt='Tu es LegalYA V30, assistant local dans ACTELYO INFERENCE HUB. Reponds en francais, de facon concise et fidele a la demande. Signale tes incertitudes et ne fabrique pas de references.'}
  if (-not $promptSaved.ok) { throw 'Enregistrement du prompt impossible.' }
}
$presets = @(Invoke-RestMethod ($base + '/api/presets') -Headers $headers)
$selectedIndex = -1
for ($i = 0; $i -lt $presets.Count; $i++) { if ($presets[$i].id -eq $saved.id) { $selectedIndex = $i; break } }
if ($selectedIndex -lt 0) { throw 'Preset LegalYA introuvable.' }
$switched = Hub-Post '/api/switch' @{n=($selectedIndex + 1)}
if (-not $switched.ok) { throw 'Selection du preset impossible.' }
Write-Output 'LegalYA V30 configure : http://127.0.0.1:8090 (inference locale via LM Studio).'
Write-Output ('Cle de pilotage : ' + $keyPath)
