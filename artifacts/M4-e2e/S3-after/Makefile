.PHONY: run clean

# 运行费用计算脚本
run:
	python src/expenses.py

# 清理临时文件
clean:
	del /Q *.pyc 2>nul || exit 0
	rmdir /S /Q __pycache__ 2>nul || exit 0