@echo off
setlocal
set BASE=C:\Users\user\win7-agent
set GIT=%BASE%\runtime\git\cmd\git.exe
set DATA=%BASE%\pocdata
set GIT_AUTHOR_NAME=poc
set GIT_AUTHOR_EMAIL=poc@local
set GIT_COMMITTER_NAME=poc
set GIT_COMMITTER_EMAIL=poc@local

echo ===== GATE A : MinGit 2.46.2 x64 =====
"%GIT%" --version && echo MARK version-ok || echo MARK version-FAIL

rmdir /s /q "%DATA%" 2>nul
mkdir "%DATA%"

set RA=%DATA%\repoA
mkdir "%RA%"
cd /d "%RA%"
"%GIT%" init -b main . && echo MARK init-ok || echo MARK init-FAIL
echo alpha-line-1 > a.txt
echo keep-me > keep.txt
echo orig-content > orig.txt
copy /y "%BASE%\poc-b.exe" tracked-bin.exe >nul
"%GIT%" add -A && echo MARK add-ok || echo MARK add-FAIL
"%GIT%" commit -m user-commit-1 && echo MARK commit-ok || echo MARK commit-FAIL
echo [status]:
"%GIT%" status --porcelain
echo [log]:
"%GIT%" log --oneline

echo ----- PART 2: private-ref checkpoint, no history pollution -----
echo alpha-line-2 >> a.txt
"%GIT%" add -A
for /f %%i in ('%GIT% write-tree') do set TREE=%%i
for /f %%i in ('%GIT% commit-tree %TREE% -p HEAD -m agent-checkpoint-1') do set CKPT=%%i
"%GIT%" update-ref refs/win7-agent/checkpoints/t1/1 %CKPT% && echo MARK privateref-ok || echo MARK privateref-FAIL
echo [user view - git log --oneline on HEAD, must NOT contain checkpoint]:
"%GIT%" log --oneline
echo [agent view - log of private ref]:
"%GIT%" log --oneline refs/win7-agent/checkpoints/t1/1
echo [for-each-ref]:
"%GIT%" for-each-ref refs/win7-agent

echo ----- PART 3: extended change set -----
echo alpha-HACKED >> a.txt
echo agent-created > created.txt
mkdir newdir
echo nested > newdir\n.txt
echo junk-corruption >> tracked-bin.exe
del keep.txt
ren orig.txt orig-moved.txt
echo m1 > m1.txt
echo m2 > m2.txt
echo ignored.log > .gitignore
echo should-not-track > ignored.log
echo [status --porcelain]:
"%GIT%" status --porcelain
echo [diff --stat vs checkpoint]:
"%GIT%" diff --stat refs/win7-agent/checkpoints/t1/1

echo ----- rollback: reset --hard to private ref -----
"%GIT%" reset --hard refs/win7-agent/checkpoints/t1/1 && echo MARK reset-ok || echo MARK reset-FAIL
type a.txt
find /c "alpha-HACKED" a.txt
if errorlevel 1 (echo MARK rollback-content-ok) else (echo MARK rollback-content-FAIL)
if exist keep.txt (echo MARK deleted-file-restored) else (echo MARK deleted-file-FAIL)
if exist orig.txt (echo MARK rename-reverted) else (echo MARK rename-FAIL)
fc /b tracked-bin.exe "%BASE%\poc-b.exe" >nul && echo MARK binary-rollback-ok || echo MARK binary-rollback-FAIL
echo [status after reset - untracked leftovers need manifest cleanup]:
"%GIT%" status --porcelain

echo ----- manifest-style cleanup of created files -----
if exist created.txt del created.txt && echo MARK cleanup-created
if exist orig-moved.txt del orig-moved.txt && echo MARK cleanup-renamed-leftover
if exist m1.txt del m1.txt
if exist m2.txt del m2.txt
if exist newdir rmdir /s /q newdir && echo MARK cleanup-newdir
if exist ignored.log del ignored.log
if exist .gitignore del .gitignore && echo MARK cleanup-gitignore
echo [final status, expect empty]:
"%GIT%" status --porcelain
echo MARK part3-final-clean

echo ----- PART 4: non-git workspace via separate git-dir/work-tree -----
set WS=%DATA%\wsB
set CK=%DATA%\ckptB\checkpoint.git
mkdir "%WS%"
mkdir "%DATA%\ckptB"
echo userfile-1 > "%WS%\u1.txt"
echo userfile-2 > "%WS%\u2.txt"
"%GIT%" init --bare "%CK%" && echo MARK bare-init-ok || echo MARK bare-init-FAIL
"%GIT%" --git-dir="%CK%" --work-tree="%WS%" add -A
for /f %%i in ('%GIT% --git-dir^="%CK%" --work-tree^="%WS%" write-tree') do set TREE2=%%i
for /f %%i in ('%GIT% --git-dir^="%CK%" commit-tree %TREE2% -m ck-B1') do set CKPT2=%%i
"%GIT%" --git-dir="%CK%" update-ref refs/win7-agent/checkpoints/wsB/1 %CKPT2% && echo MARK sep-ckpt-ok || echo MARK sep-ckpt-FAIL
echo hacked >> "%WS%\u1.txt"
echo created > "%WS%\c1.txt"
echo [sep status]:
"%GIT%" --git-dir="%CK%" --work-tree="%WS%" status --porcelain
"%GIT%" --git-dir="%CK%" --work-tree="%WS%" reset --hard refs/win7-agent/checkpoints/wsB/1 && echo MARK sep-reset-ok || echo MARK sep-reset-FAIL
type "%WS%\u1.txt"
find /c "hacked" "%WS%\u1.txt"
if errorlevel 1 (echo MARK sep-content-ok) else (echo MARK sep-content-FAIL)
if exist "%WS%\c1.txt" (del "%WS%\c1.txt" && echo MARK sep-manifest-cleanup)
echo [sep final status, expect empty]:
"%GIT%" --git-dir="%CK%" --work-tree="%WS%" status --porcelain
echo MARK part4-done

echo ===== GATE A DONE =====
cd /d "%BASE%"
