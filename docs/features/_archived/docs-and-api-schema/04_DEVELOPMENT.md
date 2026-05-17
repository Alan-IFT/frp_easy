# Development Record — T-005 docs-and-api-schema

## Summary

三個子任務全部完成：

- **子任務 A**：更新 README.md（技術債章節）和 docs/project-status.html（數字、TD/OPT 状态标记）
- **子任務 B**：修復 verify_all.sh 和 verify_all.ps1 的 D.1 條件（改為檢測 go.mod）
- **子任務 C**：新建 openapi.yaml（OpenAPI 3.0.3，28 條路由，17 個 schema）

## Files changed

| 文件 | 改動類型 | 說明 |
|---|---|---|
| `README.md` | 修改 | 技術債章節改為表格，反映 T-004/T-005 清偿状态 |
| `docs/project-status.html` | 修改 | 数字更新（PASS 12→17、SKIP 6→1、Go 测试 117→119）；TD-1～TD-7 标注已修复；OPT-1～OPT-9 标注已实现 |
| `scripts/verify_all.sh` | 修改 | D.1 前置条件从 `src/apps/packages` 改为 `go.mod` |
| `scripts/verify_all.ps1` | 修改 | D.1 前置条件同步修改 |
| `openapi.yaml` | 新建 | 项目根目录，OpenAPI 3.0.3，28 条路由，17 个 schema |
| `docs/dev-map.md` | 修改 | 追加 openapi.yaml 条目 |

## verify_all result

```
PASS: 17
WARN: 0
FAIL: 0
SKIP: 1   ← C.1 E2E（待 T-006 Playwright 后从 SKIP 变 PASS）
```

D.1 從 SKIP 變 PASS（+1）。

## Design drift

**DownloadState.status 值**：設計文檔草稿寫 `done/error`，實際 Go 常量為 `success/failed`。Gate Review 捕獲，openapi.yaml 以代碼為準使用正確值。

**OPT-9 in project-status.html**：Gate Review 條件 C-2 要求 QA 階段標注，但由于 PM 在文檔更新中直接確認功能已實現，統一在本 Developer 階段補全（開發完成即可標注）。

## Verdict

READY FOR REVIEW
