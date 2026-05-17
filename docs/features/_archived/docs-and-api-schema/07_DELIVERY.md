# 交付报告 — T-005 docs-and-api-schema

**交付日期**：2026-05-16  
**任务 ID**：T-005  
**Slug**：docs-and-api-schema

---

## 功能摘要

T-005 完成 OPT-9 及配套工程改进：

1. **README.md 更新**：末尾技术债章节由"存在 TD-1～TD-8 和 OPT-1～OPT-9"改写为表格，准确反映 T-004 清偿状态（TD-1～TD-7 ✅，TD-8 保留，OPT-1～OPT-8 ✅，OPT-9 ✅）。

2. **project-status.html 更新**：
   - 大数字更新（Go 117→119，合计 162→164，PASS 12→17，SKIP 6→1）
   - TD-1～TD-7 标注 `已修复（T-004）`
   - OPT-1～OPT-8 标注 `已实现（T-004）`，OPT-9 标注 `已实现（T-005）`
   - §7 两个已修复的待处理事项清空，改为说明性段落

3. **verify_all D.1 条件修复**：从检测 `src/apps/packages` 改为检测 `go.mod`，使 Go+Vue 项目结构下 D.1 不再永久 SKIP。无 openapi.yaml 时 WARN，有时 PASS。

4. **openapi.yaml（OPT-9）**：项目根目录，OpenAPI 3.0.3，覆盖全部 28 条路由，17 个 schema。字段名和类型对应实际 Go handler 结构体，含安全方案（cookieAuth + csrfToken）、完整请求/响应 body。

---

## 文件改动

| 文件 | 改动类型 |
|---|---|
| `README.md` | 修改（技术债章节改写为表格） |
| `docs/project-status.html` | 修改（数字更新、TD/OPT 状态标注、§7 清空） |
| `scripts/verify_all.sh` | 修改（D.1 条件改为 go.mod 检测） |
| `scripts/verify_all.ps1` | 修改（D.1 条件同步修改） |
| `openapi.yaml` | **新建**（OpenAPI 3.0.3，28 条路由） |
| `docs/dev-map.md` | 修改（追加 openapi.yaml 条目） |
| `scripts/baseline.json` | 修改（QA 更新基线：go 119，total 164） |

---

## verify_all 最终输出

```
=== Summary ===
  PASS: 17
  WARN: 0
  FAIL: 0
  SKIP: 1   ← C.1 E2E（T-006 将处理）
```

---

## AC 全部通过

| AC | 结果 |
|---|---|
| AC-A1～AC-A6（文档更新） | PASS |
| AC-B1～AC-B4（D.1 条件修复） | PASS |
| AC-C1～AC-C6（openapi.yaml） | PASS |
| AC-VERIFY | PASS |

---

## Insight

**openapi.yaml 字段名应以 Go 常量为权威，不以设计文档为准**：设计阶段 `DownloadState.status` 描述误写为 `done/error`，实际 Go 常量是 `StatusSuccess="success"` / `StatusFailed="failed"`。Gate Review 捕获了这个偏差。建议：每次新增 Go 类型后，openapi.yaml 编写者应直接读取 `.go` 文件而非复制设计文档草稿中的值。· evidence: `internal/downloader/downloader.go:32-36`
