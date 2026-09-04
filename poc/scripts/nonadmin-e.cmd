@echo off
set SBX=C:\Program Files\Sandboxie\Start.exe
set R=C:\Users\user\win7-agent\nonadmin-result.txt
echo NONADMIN-START > %R%
whoami >> %R%
"%SBX%" /box:Win7Agent /wait cmd.exe /c "echo NA-BOX-OK > C:\Users\user\win7-agent\na-box.txt" >> %R% 2>&1
echo NA-WAIT-EC=%errorlevel% >> %R%
type "C:\Sandbox\user\Win7Agent\drive\C\Users\user\win7-agent\na-box.txt" >> %R% 2>&1
echo NONADMIN-DONE >> %R%
