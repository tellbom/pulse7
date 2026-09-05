"""费用计算脚本"""

# 费用明细
items = [
    ("买东西", 35),
    ("快递", 12),
]

total = sum(amount for _, amount in items)

print("费用明细：")
for name, amount in items:
    print(f"  {name}: {amount} 元")
print(f"合计: {total} 元")