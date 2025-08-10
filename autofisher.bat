@echo off
REM filepath: c:\Users\miwam\OneDrive\ドキュメント\右クリックソフト\autofisher.bat
cd /d "%~dp0"
call c:\Users\miwam\OneDrive\ドキュメント\右クリックソフト\myenv\Scripts\activate
python main.py
timeout /t 5