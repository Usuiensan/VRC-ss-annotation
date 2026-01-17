@echo off
REM VRChat画像XMPメタデータ確認専用バッチファイル
REM check-xmp.ps1 へのラッパー

cd /d "%~dp0"

powershell -ExecutionPolicy Bypass -File "%~dp0check-xmp.ps1" %*
