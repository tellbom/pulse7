@echo off
rem Standard 3-scenario regression set - workspace reset (run before each round)
set E=%~dp0
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
echo RESET-OK
