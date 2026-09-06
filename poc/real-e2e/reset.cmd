@echo off
rem Standard 3-scenario regression set - FULL workspace reset
rem Runs git reset --hard + git clean -fd on the project copy, then
rem recreates S1/S2/S3 workspaces from scratch. Prints verification.
setlocal
set E=%~dp0

rem --- Reset real project copy (R1/R2 workspace) ---
if exist "%E%..\..\artifacts\prerc02-e2e\T7\process-copy\.git" (
  pushd "%E%..\..\artifacts\prerc02-e2e\T7\process-copy"
  git reset --hard HEAD >nul 2>&1
  git clean -fd >nul 2>&1
  for /f %%h in ('git rev-parse --short HEAD') do echo [project-copy] reset to %%h
  git status --porcelain | findstr /r ".*" >nul 2>&1
  if errorlevel 1 (
    echo [project-copy] CLEAN
  ) else (
    echo [project-copy] DIRTY - MANUAL CHECK NEEDED
  )
  popd
)

rem --- Reset S1/S2/S3 toy workspaces ---
rd /s /q "%E%S1" 2>nul & mkdir "%E%S1"
(echo # bill splitter: divide a total amount evenly among people
 echo def split(total, people^):
 echo     if people ^< 1:
 echo         raise ValueError("people must be ^>= 1"^)
 echo     return total / people
 echo.
 echo def main(^):
 echo     total = 120
 echo     people = 0
 echo     print("each pays:", split(total, people^)^)
 echo.
 echo if __name__ == "__main__":
 echo     main(^)
) > "%E%S1\calc.py"

rd /s /q "%E%S2" 2>nul & mkdir "%E%S2"
(echo def add(a, b^):
 echo     return a + b
 echo.
 echo def sub(a, b^):
 echo     return a - b
) > "%E%S2\utils.py"
(echo from utils import add
 echo.
 echo def main(^):
 echo     print("add:", add(2, 3^)^)
 echo.
 echo if __name__ == "__main__":
 echo     main(^)
) > "%E%S2\main.py"

rd /s /q "%E%S3" 2>nul & mkdir "%E%S3"
echo 随手记：买东西 35 + 快递 12> "%E%S3\notes.txt"
echo print('old'^)> "%E%S3\old_tmp.py"

echo [S1/S2/S3] RESET-OK
