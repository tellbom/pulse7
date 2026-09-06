@echo off
set E=C:\Users\user\real-e2e
rd /s /q %E%\S1 2>nul & mkdir %E%\S1
> %E%\S1\calc.py (
echo # bill splitter: divide a total amount evenly among people
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
)
rd /s /q %E%\S2 2>nul & mkdir %E%\S2
> %E%\S2\utils.py (
echo def add(a, b^):
echo     return a + b
echo.
echo def sub(a, b^):
echo     return a - b
)
> %E%\S2\main.py (
echo from utils import add
echo.
echo def main(^):
echo     print("add:", add(2, 3^)^)
echo.
echo if __name__ == "__main__":
echo     main(^)
)
rd /s /q %E%\S3 2>nul & mkdir %E%\S3
echo 随手记：买东西 35 + 快递 12> %E%\S3\notes.txt
echo print('old'^)> %E%\S3\old_tmp.py
echo RESET-DONE
