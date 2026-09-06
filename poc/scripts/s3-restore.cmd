@echo off
rd /s /q C:\Users\user\real-e2e\S3 2>nul
mkdir C:\Users\user\real-e2e\S3
echo 随手记：买东西 35 + 快递 12> C:\Users\user\real-e2e\S3\notes.txt
echo print('old')> C:\Users\user\real-e2e\S3\old_tmp.py
echo S3-RESTORED
