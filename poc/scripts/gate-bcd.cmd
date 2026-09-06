@echo off
set BASE=C:\Users\user\win7-agent
cd /d "%BASE%"

echo ===== GATE B : Go 1.20.14 cross-compiled binary =====
poc-b.exe
if errorlevel 1 (echo MARK gateb-FAIL) else (echo MARK gateb-ok)

echo ===== GATE C : go-openai over mock endpoint (HTTP) =====
start /b cmd /c "poc-net.exe serve 45 > serve.log 2>&1"
ping -n 4 127.0.0.1 >nul
poc-net.exe client http://127.0.0.1:8080/v1
if errorlevel 1 (echo MARK gatec-http-FAIL) else (echo MARK gatec-http-ok)

echo ===== GATE D : HTTPS TLS1.2+ with real cert verification =====
poc-net.exe client https://127.0.0.1:8443/v1 ca.pem
if errorlevel 1 (echo MARK gated-https-FAIL) else (echo MARK gated-https-ok)
poc-net.exe client-noca https://127.0.0.1:8443/v1
if errorlevel 1 (echo MARK gated-noca-FAIL) else (echo MARK gated-noca-ok)

echo ----- schannel contrast (PS2.0 expected to FAIL) -----
powershell -NoProfile -Command "try { (New-Object Net.WebClient).DownloadString('https://127.0.0.1:8443/v1/models') } catch { 'PS-SCHANNEL-FAILED: ' + $_.Exception.Message }"

echo ===== SERVER LOG =====
type serve.log
taskkill /f /im poc-net.exe >nul 2>&1
echo ===== GATE B/C/D DONE =====
