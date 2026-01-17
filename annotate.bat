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
main.exe --annotate %*

REM 処理終了時に一瞬画面を表示（確認用）
echo.
echo 処理完了。このウィンドウを閉じてください...
pause
