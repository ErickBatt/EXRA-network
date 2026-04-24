@echo off
chcp 65001 >nul
title EXRA Deploy

echo.
echo  ================================
echo   EXRA — Деплой на продакшн
echo  ================================
echo.

cd /d "%~dp0"

python deploy/auto-deploy.py
