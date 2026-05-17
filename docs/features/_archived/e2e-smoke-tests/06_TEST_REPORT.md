# 06 Test Report — T-006 e2e-smoke-tests

**测试日期**：2026-05-17  
**测试人**：QA Tester (Claude)  
**环境**：Windows 11 + Git Bash + Node.js + Go 1.22+

---

## 验收结论

**PASSED** — 全部 5 条 TC 通过，verify_all PASS:18 FAIL:0 SKIP:0

---

## E2E 测试结果

```
Running 5 tests using 1 worker

  ✓ TC-01 未初始化时访问 / 自动跳转 /setup (152ms)
  ✓ TC-02 setup 表单提交成功后离开 /setup (284ms)
  ✓ TC-03 login 表单提交成功后离开 /login (268ms)
  ✓ TC-04 dashboard 关键元素可见 (570ms)
  ✓ TC-05 退出登录跳转 /login，session 清除 (356ms)

  5 passed (2.9s)
```

---

## verify_all 输出

```
PASS: 18 | WARN: 0 | FAIL: 0 | SKIP: 0
C.1 E2E smoke (playwright) ... PASS
```

---

## Adversarial tests

| 场景 | 预期 | 实际 |
|---|---|---|
| 使用旧二进制（dist/ 未嵌入）启动服务器 | 前端显示占位页，TC-01~TC-05 全部失败 | 已通过时间戳检查逻辑触发重建，正确 |
| 复用已有服务器（reuseExistingServer:true）但数据库有用户 | TC-01 不能跳转 /setup | 在 CI 中 reuseExistingServer=false 避免；本地开发使用者需知晓 |
| App.vue 缺失 NMessageProvider | useMessage() 抛出，组件不渲染，所有表单测试超时 | 已修复，验证通过 |
| TC-04 getByText('仪表盘') 匹配多个元素 | Playwright 严格模式违反 | 已改为 .first()，测试通过 |
| bypassWizard 无效 CSRF token | POST /wizard/complete 返回 403 | 测试内先 GET /auth/csrf 获取有效 token，正确 |

---

## 回归检查

所有既有 Go 单元测试（119 个）和 Vue 单元测试保持通过，无回归。
