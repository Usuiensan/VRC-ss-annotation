@echo off
REM VRChat画像メタデータ確認用バッチファイル
REM ドラッグ&ドロップ対応：JSON形式でメタデータを表示＆保存

cd /d "%~dp0"

if "%~1"=="" (
    echo 画像ファイルをドラッグ・アンド・ドロップしてください。
    pause
    exit /b 1
)

REM 複数ファイル対応：各ファイルを個別に処理
:LOOP
if "%~1"=="" goto END

echo.
echo ========================================
echo 処理中: %~nx1
echo ========================================

REM ログファイル名生成（画像のあったディレクトリに出力）
set LOGFILE=%~dp1%~nx1_metadata.json
set TEMPFILE=%TEMP%\metadata_temp_%RANDOM%.json

REM JSON出力を一時ファイルへ
VRCSSAnnotationTool.exe --json --pretty --no-escape "%~1" > %TEMPFILE% 2>&1

REM 有効なデータがあるかチェック（ファイルサイズ > 10バイト）
for %%A in (%TEMPFILE%) do set FILESIZE=%%~zA
if %FILESIZE% GTR 10 (
    REM 画面表示
    type %TEMPFILE%
    REM 正式なログファイルに移動
    move /Y %TEMPFILE% %LOGFILE% >nul
    echo.
    echo [OK] 結果を %LOGFILE% に保存しました
) else (
    REM 空または無効なデータ - ログファイル作成しない
    type %TEMPFILE%
    del %TEMPFILE%
    echo.
    echo [SKIP] メタデータがないため、ログファイルは作成されませんでした
)

shift
goto LOOP

:END
echo.
echo ========================================
echo 全ての処理が完了しました
echo ========================================
timeout /t 10
