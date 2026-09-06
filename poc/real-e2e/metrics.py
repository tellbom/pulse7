#!/usr/bin/env python3
"""Session metrics for the 3-scenario regression set."""
import json, sys, os
for p in sys.argv[1:]:
    rounds=tools=writes=0; final=None
    for line in open(p, encoding="utf-8"):
        m=json.loads(line); r=m.get("role")
        if r=="assistant":
            if m.get("tool_calls"):
                rounds+=1
                for tc in m["tool_calls"]:
                    if tc["function"]["name"] in ("write","edit"): writes+=1
            elif m.get("content"): final=m["content"]
    asked = bool(final) and any(k in final for k in ["？","?"]) and any(k in final for k in ["具体","指什么","哪些","希望","是否"])
    print(f"{os.path.basename(p)}: rounds={rounds} writes={writes} asks_user={asked} converged={bool(final)}")
