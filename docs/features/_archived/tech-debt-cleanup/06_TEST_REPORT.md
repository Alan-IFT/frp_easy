# 测试报告 — T-004 tech-debt-cleanup

**测试日期**：2026-05-16  
**结论**：**APPROVED FOR DELIVERY**，全部 AC PASS，verify_all PASS:16 FAIL:0

---

## AC 测试结果

| AC | 条件 | 结果 |
|---|---|---|
| AC-F1-1 | verify_all B.1 非 SKIP | PASS |
| AC-F1-3 | B.3 前端测试 PASS | PASS（45 个 Vitest 测试） |
| AC-F2-1 | 向导完成后 /wizard → /dashboard | PASS（router.ts 逻辑验证） |
| AC-F3-1 | slog MultiWriter | PASS（代码审查） |
| AC-F4-1 | build.sh git describe | PASS（代码审查） |
| AC-F5-1 | fetchIPFromURL → ParseIPFromJSON | PASS（go test ./internal/httpapi/... 通过） |
| AC-F6-1 | /health 200+JSON | PASS（TestHealth_ReturnsOK 通过） |
| AC-F6-3 | /health 绕过 ReadyGate | PASS（TestHealth_BypassesReadyGate 通过） |
| AC-F7-1 | TOML 预检 | PASS（代码审查 + 现有测试不降） |
| AC-VERIFY | verify_all FAIL:0 | PASS |

---

## Adversarial tests

| 假设 | 验证方法 | 结论 |
|---|---|---|
| B.1-B.4 是真正运行还是 SKIP | `bash scripts/verify_all.sh` 输出 `[B.1] ... PASS` | 真正运行 PASS |
| health 端点被 ReadyGate 拦截 | TestHealth_BypassesReadyGate：ready=false 时 GET /health 返回 200 | PASS（绕过有效） |
| 向导守卫在未登录时不触发 | router.ts 守卫：`auth.user !== null` 前置检查 | 未登录用户访问 /wizard 不触发（走到 /login 守卫） |
| TOML 预检不阻止正常 Start | 现有 autoRestoreProcs 路径：TOML 存在时直接 Start | 不受影响 |
| ParseIPFromJSON 行为与原内联解析一致 | downloader.ParseIPFromJSON 返回 `{"ip":"..."}` 的 ip 字段，与原 struct 解码完全等价 | PASS |
| TypeScript typecheck 无误报 | `npx tsc --noEmit` 零输出 | PASS |
| 前端 lint 无新警告 | `[B.2] Lint ... PASS` | PASS |
| go test 总数不降 | 新增 2 个测试，总数从 117 升至 119 | PASS |

---

## verify_all 输出

```
=== Summary ===
  PASS: 16
  WARN:  0
  FAIL:  0
  SKIP:  2
```
