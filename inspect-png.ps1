# PNG ファイルのチャンク構造を確認するスクリプト

param(
    [Parameter(Mandatory=$true)]
    [string]$ImagePath
)

$data = [System.IO.File]::ReadAllBytes($ImagePath)

if ($data.Length -lt 8) {
    Write-Host "Invalid PNG file"
    exit 1
}

# PNG シグネチャ確認
$sig = $data[0..7]
$sigStr = [BitConverter]::ToString($sig).Replace("-", " ")
Write-Host "PNG Signature: $sigStr"

$expectedSig = @(0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A)
$sigMatch = $true
for ($i = 0; $i -lt 8; $i++) {
    if ($sig[$i] -ne $expectedSig[$i]) {
        $sigMatch = $false
        break
    }
}

if (-not $sigMatch) {
    Write-Host "Not a valid PNG file"
    exit 1
}

# チャンク情報を表示
$offset = 8
$chunkNum = 0
$textChunks = @()

while ($offset + 8 -le $data.Length) {
    $lengthBytes = $data[$offset..($offset+3)]
    if ([BitConverter]::IsLittleEndian) {
        [Array]::Reverse($lengthBytes)
    }
    $length = [BitConverter]::ToUInt32($lengthBytes, 0)
    
    $chunkType = [System.Text.Encoding]::ASCII.GetString($data[($offset+4)..($offset+7)])
    $chunkDataStart = $offset + 8
    $chunkDataEnd = $chunkDataStart + $length
    $chunkCRCEnd = $chunkDataEnd + 4

    if ($chunkDataEnd -gt $data.Length -or $chunkCRCEnd -gt $data.Length) {
        break
    }

    # テキストチャンクのみを記録
    if ($chunkType -in @("tEXt", "iTXt", "zTXt")) {
        $textChunks += @{
            Type = $chunkType
            Length = $length
            Data = $data[$chunkDataStart..($chunkDataEnd-1)]
        }
    }

    $offset = $chunkCRCEnd
    $chunkNum++
}

Write-Host "Total chunks: $chunkNum"
Write-Host "Text chunks found: $($textChunks.Count)"

foreach ($chunk in $textChunks) {
    Write-Host ""
    Write-Host "Chunk Type: $($chunk.Type) (Length: $($chunk.Length) bytes)"
    
    $chunkData = $chunk.Data
    
    if ($chunk.Type -eq "tEXt") {
        $nullIdx = [Array]::IndexOf($chunkData, 0)
        if ($nullIdx -ne -1) {
            $keyword = [System.Text.Encoding]::ASCII.GetString($chunkData[0..($nullIdx-1)])
            $text = [System.Text.Encoding]::UTF8.GetString($chunkData[($nullIdx+1)..($chunkData.Length-1)])
            Write-Host "  Keyword: $keyword"
            Write-Host "  Content (first 150 chars): $($text.Substring(0, [Math]::Min(150, $text.Length)))"
        }
    } elseif ($chunk.Type -eq "iTXt") {
        $nullIdx = [Array]::IndexOf($chunkData, 0)
        if ($nullIdx -ne -1) {
            $keyword = [System.Text.Encoding]::ASCII.GetString($chunkData[0..($nullIdx-1)])
            Write-Host "  Keyword: $keyword"
            if ($keyword -like "*XMP*" -or $keyword -like "*xmp*") {
                Write-Host "  XMP データ検出!"
            }
        }
    } elseif ($chunk.Type -eq "zTXt") {
        $nullIdx = [Array]::IndexOf($chunkData, 0)
        if ($nullIdx -ne -1) {
            $keyword = [System.Text.Encoding]::ASCII.GetString($chunkData[0..($nullIdx-1)])
            Write-Host "  Keyword: $keyword"
            if ($keyword -like "*XMP*" -or $keyword -like "*xmp*") {
                Write-Host "  XMP データ検出!"
            }
        }
    }
}
