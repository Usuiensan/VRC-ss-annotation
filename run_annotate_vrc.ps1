# Run annotation over all images under C:\Users\miwam\Pictures\VRChat (recursive)
# - Process files from oldest -> newest (CreationTime)
# - Avoid modifying originals by copying each file to $env:TEMP and passing the temp copy to the exe
# - Avoid overwriting existing files in destination by appending -1, -2, ... to filename

$ErrorActionPreference = 'Continue'
$src = 'C:\Users\miwam\Pictures\VRChat'
$dest = 'D:\library\Auto-import\VRC-stamped'
$exe = Join-Path (Get-Location) 'VRCSSAnnotationTool.exe'

if (-not (Test-Path $exe)) {
    Write-Error "Executable not found: $exe. Build it (go build -ldflags `"-s -w`" -o VRCSSAnnotationTool.exe .) and run again."
    exit 1
}
if (-not (Test-Path $src)) {
    Write-Error "Source path not found: $src"
    exit 1
}
if (-not (Test-Path $dest)) {
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
}

$files = Get-ChildItem -Path $src -File -Recurse -Include *.png,*.webp,*.jpg,*.jpeg | Sort-Object CreationTime
$total = $files.Count
Write-Output "Found $total files. Processing oldest -> newest (by CreationTime)."

$i = 0
foreach ($f in $files) {
    $i++
    try {
        $srcFile = $f.FullName
        $base = $f.BaseName
        $ext = $f.Extension

        # choose unique name in destination
        $destName = $f.Name
        $j = 1
        while (Test-Path (Join-Path $dest $destName)) {
            $destName = "{0}-{1}{2}" -f $base, $j, $ext
            $j++
        }

        $tempPath = Join-Path $env:TEMP $destName
        Copy-Item -LiteralPath $srcFile -Destination $tempPath -Force -ErrorAction Stop

        Write-Output "[$i/$total] Processing: $srcFile  => will output as: $destName"
        # Run the annotation tool; capture exit code and output
        $proc = Start-Process -FilePath $exe -ArgumentList @("--output-dir", $dest, "--annotate", $tempPath) -NoNewWindow -PassThru -Wait -ErrorAction Stop
        if ($proc.ExitCode -ne 0) {
            Write-Warning ("[{0}/{1}] Tool exited with code {2} for {3}" -f $i, $total, $proc.ExitCode, $srcFile)
        }

        # Remove temp copy
        Remove-Item $tempPath -Force -ErrorAction SilentlyContinue
    } catch {
        Write-Warning ("[{0}/{1}] Unexpected error processing {2}: {3}" -f $i, $total, $f.FullName, $_.Exception.Message)
        # try to cleanup temp if left behind
        if (Test-Path $tempPath) { Remove-Item $tempPath -Force -ErrorAction SilentlyContinue }
        continue
    }
}

Write-Output "All done. Processed $i files. Output dir: $dest"
exit 0
