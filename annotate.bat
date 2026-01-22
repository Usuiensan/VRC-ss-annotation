@echo off
REM VRChat画像アノテーション用バッチファイル
REM ドラッグ&ドロップ対応：自動的に --annotate フラグを付与

cd /d "%~dp0"

if "%~1"=="" (
    echo 画像ファイルをドラッグ・アンド・ドロップしてください。
    pause
    exit /b 1
)

REM すべての引数に対して --annotate フラグを付与して実行
@REM echo 低負荷モードが有効です。処理を開始します...
VRCSSAnnotationTool.exe --annotate --output-dir "E:\VRC-annotated-pic" %*
REM 処理終了時に一瞬画面を表示（確認用）
echo.
echo 処理完了。このウィンドウを閉じてください...
timeout /t 5 /nobreak >nul
