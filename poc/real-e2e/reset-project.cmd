@echo off
rem Full reset of the real project copy on Win7 (R1/R2 workspace)
set W=C:\Users\user\T7\process-copy
if not exist "%W%\.git" (
  echo FATAL: %W% is not a git repository
  exit /b 1
)
cd /d %W%
git reset --hard HEAD >nul 2>&1
git clean -fd >nul 2>&1
for /f %%h in ('git rev-parse --short HEAD') do echo [project-copy] reset to %%h
git status --porcelain 2>nul | findstr /r ".*" >nul 2>&1
if errorlevel 1 (
  echo [project-copy] CLEAN - ready for testing
  exit /b 0
) else (
  echo [project-copy] DIRTY - manual check needed
  exit /b 1
)
