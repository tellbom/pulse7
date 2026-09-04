@echo off
C:\Users\user\win7-agent\Sandboxie-Plus-x64-v1.18.2.exe --default-answer --accept-messages --confirm-command install > C:\Users\user\win7-agent\ifw.log 2>&1
echo EXITCODE=%errorlevel% >> C:\Users\user\win7-agent\ifw.log
