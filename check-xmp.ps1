# VRChat画像XMPメタデータ確認専用PowerShellスクリプト
# ドラッグ&ドロップ対応：XMP（XML）を整形して表示

param(
    [Parameter(ValueFromRemainingArguments=$true)]
    [string[]]$Files
)

# 文字エンコーディング設定
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

# スクリプトのディレクトリに移動
Set-Location -Path $PSScriptRoot

# XMP出力ディレクトリ作成
$xmpDir = Join-Path $PSScriptRoot "xmp"
if (-not (Test-Path $xmpDir)) {
    New-Item -ItemType Directory -Path $xmpDir -Force | Out-Null
}

# 引数チェック
if ($Files.Count -eq 0) {
    Write-Host "画像ファイルをドラッグ・アンド・ドロップしてください。" -ForegroundColor Yellow
    Read-Host "Enterキーで終了"
    exit 1
}

# 各ファイルを処理
foreach ($file in $Files) {
    Write-Host ""
    Write-Host "========================================"
    Write-Host "XMP確認: $(Split-Path $file -Leaf)"
    Write-Host "========================================"
    
    try {
        # JSON出力を直接取得（エラー出力は破棄）
        $jsonOutput = & .\VRCSSAnnotationTool.exe --json --no-escape $file 2>$null
        
        # JSON が null または空の場合
        if (-not $jsonOutput) {
            Write-Host "--- メタデータ読み込み失敗 ---" -ForegroundColor Red
            continue
        }
        
        # JSONを文字列に変換してパース
        $jsonString = $jsonOutput -join "`n"
        $json = $jsonString | ConvertFrom-Json
        
        # XMPフィールドを抽出
        $xmp = if ($json.xmpRawWebP) { 
            $json.xmpRawWebP 
        } elseif ($json.xmpRawPNG) { 
            $json.xmpRawPNG 
        } else { 
            $null 
        }
        
        if ($xmp) {
            Write-Host "--- XMP データ検出 ---" -ForegroundColor Green
            
            # XML整形
            [xml]$xml = $xmp
            $stringWriter = New-Object System.IO.StringWriter
            $xmlWriter = [System.Xml.XmlTextWriter]::new($stringWriter)
            $xmlWriter.Formatting = [System.Xml.Formatting]::Indented
            $xmlWriter.Indentation = 2
            $xml.WriteContentTo($xmlWriter)
            $xmlWriter.Flush()
            
            $formattedXmp = $stringWriter.ToString()
            Write-Host $formattedXmp
            
            $xmlWriter.Close()
            $stringWriter.Close()
            
            # ファイルに出力
            $fileName = [System.IO.Path]::GetFileNameWithoutExtension((Split-Path $file -Leaf))
            $outputPath = Join-Path $xmpDir "$fileName.xml"
            $formattedXmp | Out-File -Encoding utf8 -FilePath $outputPath -Force
            Write-Host "保存: $outputPath" -ForegroundColor Cyan
        } else {
            Write-Host "--- XMP データなし ---" -ForegroundColor Red
        }
    }
    catch {
        Write-Host "エラー: $_" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "========================================"
Write-Host "全ての処理が完了しました"
Write-Host "========================================"
Start-Sleep -Seconds 1
