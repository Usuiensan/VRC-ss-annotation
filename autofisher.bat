@echo off
REM filepath: c:\Users\miwam\OneDrive\ドキュメント\右クリックソフト\autofisher.bat
cd /d "%~dp0"
call C:\Users\miwam\OneDrive\ドキュメント\右クリックソフト\myenv313\Scripts\activate.bat
python main.py
timeout /t 5