$ErrorActionPreference = 'Stop'
Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
$env:GOENV='off'; $env:GOWORK='off'; $env:GOTOOLCHAIN='local'; $env:GOFLAGS='-mod=readonly'; $env:CGO_ENABLED='0'; $env:GOOS='windows'; $env:GOARCH='amd64'
$repo=(Get-Location).Path
$out=Join-Path $repo '.security/windows-native-ci'
if (Test-Path $out) { throw 'unique native output required' }
New-Item -ItemType Directory $out | Out-Null
$paths=@(git ls-files '*.go' go.mod go.sum x/go.mod x/go.sum tools/go.mod tools/go.sum | Sort-Object)
if ($LASTEXITCODE -ne 0) { throw 'tracked source discovery failed' }
function SourceMap {
  $map=[ordered]@{}
  foreach ($relative in $paths) { $map[$relative]=(Get-FileHash (Join-Path $repo $relative) -Algorithm SHA256).Hash }
  return $map
}
$sourceBefore=SourceMap
$before=$sourceBefore['signals_windows.go']
$head=(git rev-parse HEAD); $tree=(git rev-parse 'HEAD^{tree}')
$go=(Get-Command go).Source
$version=(go 'version')

$steps=@();$success=$false; $propertiesValidated=0; $failure=$null; $stage='native-toolchain-preflight'
try {
  if ($version -ne 'go version go1.26.6 windows/amd64') { throw "wrong native toolchain: $version" }
  $effective=go 'env' 'GOOS' 'GOARCH' 'CGO_ENABLED' 'GOTOOLCHAIN' 'GOENV' 'GOWORK'
  if ($LASTEXITCODE -ne 0 -or $effective[0] -ne 'windows' -or $effective[1] -ne 'amd64' -or $effective[2] -ne '0') { throw 'effective native environment mismatch' }
  Remove-Item Env:ASYNQ_WINDOWS_SIGNAL_MUTANT,Env:ASYNQ_WINDOWS_SIGNAL_CHILD,Env:ASYNQ_WINDOWS_SIGNAL_READY,Env:ASYNQ_WINDOWS_SIGNAL_REQUIRE_COVER,Env:ASYNQ_WINDOWS_SIGNAL_CASES -ErrorAction SilentlyContinue
  $env:ASYNQ_WINDOWS_SIGNAL_REQUIRE_COVER='1'; $env:ASYNQ_WINDOWS_SIGNAL_CASES='100'
  $stage='baseline-build'
  $exe=Join-Path $out 'baseline.test.exe'
  go 'test' '-c' '-cover' '-covermode=atomic' '-coverpkg=github.com/austinyuch/asynq' '-o' $exe '.' *> (Join-Path $out 'baseline-build.log')
  $buildExit=$LASTEXITCODE; if ($buildExit -ne 0) { throw 'native baseline compile failed' }
  $env:ASYNQ_WINDOWS_SIGNAL_ARTIFACTS=Join-Path $out 'baseline-children'
  $stage='baseline-native'
  & $exe '-test.run=^TestWindowsNativeSignals$' '-test.v' '-test.timeout=120s' "-test.coverprofile=$out/parent.cover" *> (Join-Path $out 'baseline.log')
  $exit=$LASTEXITCODE
  $steps+=@{name='baseline';build_exit=$buildExit;native_exit=$exit;binary_sha256=(Get-FileHash $exe -Algorithm SHA256).Hash}
  if ($exit -ne 0) { throw 'native baseline failed; retained raw logs' }
  $children=Get-ChildItem $env:ASYNQ_WINDOWS_SIGNAL_ARTIFACTS -Directory
  if ($children.Count -ne 100) { throw 'expected child census100 mismatch' }
  foreach ($child in $children) {
    if ($child.Name -notmatch '^\d{3}-(server|scheduler)$') { throw 'unknown expected child directory' }
    $identity=Get-Content (Join-Path $child.FullName 'identity.txt') -Raw
    if ($identity -notmatch 'pid=\d+ creation_hi=\d+ creation_lo=\d+ kind=(server|scheduler) owned_job=true new_process_group=true') { throw 'missing owned process start/job identity' }
    $custody=Get-Content (Join-Path $child.FullName 'custody.txt') -Raw
    if ($custody -notmatch 'wait=<nil> closed=true') { throw 'missing normal owned child terminal' }
    $profile=Get-Content (Join-Path $child.FullName 'child.cover')
    if ($profile[0] -notmatch '^mode:') { throw 'missing own child profile' }
    $targets=if ($child.Name -match 'server$') {@(18,19,20,21)} else {@(25,26,27,28)}
    foreach ($line in $targets) {
      $matched=$false
      foreach ($entry in $profile) {
        if ($entry -match 'signals_windows\.go:(\d+)\.(\d+),(\d+)\.(\d+)\s+(\d+)\s+(\d+)$') {
          if ([int]$Matches[1] -le $line -and [int]$Matches[3] -ge $line -and [int]$Matches[6] -gt 0) { $matched=$true }
        }
      }
      if (-not $matched) { throw "missing bounded original signal line $line" }
    }
    $propertiesValidated++
  }
  # Counter blocks corroborate only these known straight-line statements; every
  # child also returned from the receive normally. Never generalize span hits.
  $original=[IO.File]::ReadAllText((Join-Path $repo 'signals_windows.go'))
  if ([regex]::Matches($original,[regex]::Escape(', windows.SIGINT')).Count -ne 2 -or [regex]::Matches($original,[regex]::Escape('<-sigs')).Count -ne 2) { throw 'exact two-function mutation sites mismatch' }
  $variants=@{ 'missing-interrupt'=$original.Replace(', windows.SIGINT',''); 'premature-return'=$original.Replace('<-sigs','_ = sigs') }
  $env:ASYNQ_WINDOWS_SIGNAL_MUTANT='1'; $env:ASYNQ_WINDOWS_SIGNAL_REQUIRE_COVER='0'
  $seenMutantBinaryHashes=@{}
  foreach ($name in $variants.Keys) {
    if ($variants[$name] -eq $original) { throw 'mutant did not modify subject' }
    $dir=Join-Path $out $name;New-Item -ItemType Directory $dir | Out-Null
    # Mutate only an owned private module copy; no tracked checkout mutation.
    $private=Join-Path $dir 'module'; New-Item -ItemType Directory $private | Out-Null
    foreach ($relative in (git ls-files '*.go' go.mod go.sum x/go.mod x/go.sum tools/go.mod tools/go.sum)) {
      $dest=Join-Path $private $relative
      New-Item -ItemType Directory -Force (Split-Path $dest -Parent) | Out-Null
      [IO.File]::WriteAllBytes($dest,[IO.File]::ReadAllBytes((Join-Path $repo $relative)))
    }
    $replacement=Join-Path $private 'signals_windows.go'
    if ((Get-FileHash $replacement -Algorithm SHA256).Hash -ne $before) { throw 'private source before mismatch' }
    [IO.File]::WriteAllText($replacement,$variants[$name])
    if ((Get-FileHash $replacement -Algorithm SHA256).Hash -eq $before) { throw 'private mutant unchanged' }
    $mutantSourceHash=(Get-FileHash $replacement -Algorithm SHA256).Hash
    $stage="$name-build"
    $binary=Join-Path $dir 'mutant.test.exe'
    Push-Location $private
    try { go 'test' '-c' '-o' $binary '.' *> (Join-Path $dir 'build.log') } finally { Pop-Location }
    $buildExit=$LASTEXITCODE;if ($buildExit -ne 0) { throw 'unviable mutant compile' }
    if ((Get-FileHash $binary -Algorithm SHA256).Hash -eq (Get-FileHash $exe -Algorithm SHA256).Hash) { throw 'mutant binary identical to baseline' }
    $mutantBinaryHash=(Get-FileHash $binary -Algorithm SHA256).Hash
    if ($seenMutantBinaryHashes.ContainsKey($mutantBinaryHash)) { throw 'mutant binaries are identical' }
    $seenMutantBinaryHashes[$mutantBinaryHash]=$name
    $env:ASYNQ_WINDOWS_SIGNAL_ARTIFACTS=Join-Path $dir 'children'
    $stage="$name-native"
    & $binary '-test.run=^TestWindowsNativeSignals$' '-test.v' '-test.timeout=30s'  *> (Join-Path $dir 'native.log')
    $exit=$LASTEXITCODE
    $logPieces=@(Get-Content (Join-Path $dir 'native.log') -Raw)
    $logPieces+=@(Get-ChildItem $env:ASYNQ_WINDOWS_SIGNAL_ARTIFACTS -Filter *.log -Recurse | ForEach-Object { Get-Content $_.FullName -Raw })
    $logs=$logPieces -join "`n"
    $steps+=@{name=$name;build_exit=$buildExit;native_exit=$exit;private_source_before=$before;private_source_after=$mutantSourceHash;binary_sha256=(Get-FileHash $binary -Algorithm SHA256).Hash}
    $oracle=if ($name -eq 'missing-interrupt') {'waitForSignals failed to return after targeted CTRL_BREAK'} else {'waitForSignals returned before any owned signal'}
    $parentLog=Get-Content (Join-Path $dir 'native.log') -Raw
    if ($exit -ne 1 -or $logs -notmatch [regex]::Escape($oracle) -or $parentLog -notmatch '(?m)^--- FAIL: TestWindowsNativeSignals' -or $logs -match '(?im)(panic:|fatal error:|test timed out|^ERROR|owned child wait closure missing|owned job assignment preflight failed|owned console preflight|owned READY deadline|owned child completion deadline)') { throw 'ordinary mutant assertion not demonstrated' }
    $mutantChildren=@(Get-ChildItem $env:ASYNQ_WINDOWS_SIGNAL_ARTIFACTS -Directory)
    if ($mutantChildren.Count -ne 2) { throw 'mutant child census mismatch' }
    foreach ($child in $mutantChildren) {
      $terminal=Get-Content (Join-Path $child.FullName 'terminal.txt') -Raw
      $identity=Get-Content (Join-Path $child.FullName 'identity.txt') -Raw
      if ($terminal -notmatch '^pid=\d+ closed=true' -or $identity -notmatch 'owned_job=true new_process_group=true') { throw 'mutant owned child terminal missing' }
    }
  }
  $success=$true
 } catch {
  $failure=@{stage=$stage;error=$_.Exception.Message}
  throw
} finally {
  $sourceAfter=SourceMap
  $closedChildren=0
  $baselineChildren=Join-Path $out 'baseline-children'
  if (Test-Path $baselineChildren) {
    foreach ($child in @(Get-ChildItem $baselineChildren -Directory)) {
      $custody=Join-Path $child.FullName 'custody.txt'
      if ((Test-Path $custody) -and (Get-Content $custody -Raw) -match 'wait=<nil> closed=true') { $closedChildren++ }
    }
  }
  $after=(Get-FileHash signals_windows.go -Algorithm SHA256).Hash
  $sourceStable=(($sourceBefore|ConvertTo-Json -Compress) -eq ($sourceAfter|ConvertTo-Json -Compress)) -and ($before -eq $after)
  if (-not $sourceStable) { $success=$false; $failure=@{stage='source-final-readback';error='tracked source bindings changed'} }
  $files=@{};Get-ChildItem $out -File -Recurse|ForEach-Object {$files[$_.FullName]=(Get-FileHash $_.FullName -Algorithm SHA256).Hash}
  @{status=if($success){'BOUNDED_WINDOWS_NATIVE_SIGNALS_PASS'}else{'FAIL_RETAINED'};head=$head;tree=$tree;go_version=$version;go_sha256=(Get-FileHash $go -Algorithm SHA256).Hash;source_before=$before;source_after=$after;steps=$steps;properties_requested=100;properties_completed_closed=$closedChildren;properties_profile_validated=$propertiesValidated;failure=$failure;source_bindings_before=$sourceBefore;source_bindings_after=$sourceAfter;env_allowlist=@{GOENV=$env:GOENV;GOWORK=$env:GOWORK;GOTOOLCHAIN=$env:GOTOOLCHAIN;GOFLAGS=$env:GOFLAGS;GOOS=$env:GOOS;GOARCH=$env:GOARCH;CGO_ENABLED=$env:CGO_ENABLED;GOROOT_unset=(-not(Test-Path Env:GOROOT))};profile_model='known8 straight-line statements plus child receive-return oracle; not generic span filling';project_ratio=$null;materials=$files}|ConvertTo-Json -Depth 10|Set-Content (Join-Path $out 'receipt.json')
  if (-not $sourceStable) { throw 'tracked source bindings changed; FAIL receipt retained' }
}

# Expected mutant exits are handled above; only validated success reaches this epilogue.
if (-not $success -or -not $sourceStable -or $null -ne $failure) { throw 'validated campaign success required' }
exit 0
