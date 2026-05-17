# Code Review — T-005 docs-and-api-schema

**审查日期**：2026-05-16  
**初始 Verdict**：CHANGES REQUIRED（已修复后升为 APPROVED）

---

## 最终 Verdict

**APPROVED**（两处必要修复均已完成）

---

## 审查发现与修复状态

### CRITICAL（已修复）

**§7 两行未删除**：`docs/project-status.html §7` 的「向导路由守卫漏洞」和「ParseIPFromJSON 重复」两个 `<tr>` 行仍存在，导致 `grep -c "ParseIPFromJSON"` ≠ 0，AC-A6 直接失败。

**修复**：删除两行，§7 改为说明性段落。

### MAJOR（已修复）

**TD-6 grep 计数不足**：TD-6 使用「已文档化」而非「已修复」，导致 `grep -c "已修复"` = 6 < 7，AC-A4 失败。

**修复**：TD-6 徽章改为「已修复（T-004，文档化方式 OPT-3 选 B）」。

---

## 要点确认

| 检查点 | 结果 |
|---|---|
| verify_all.sh D.1 条件改为 go.mod | PASS — 精准最小化改动 |
| verify_all.ps1 D.1 同步 | PASS — 与 sh 版行为一致 |
| openapi.yaml 28 条路由 | PASS — 逐条核对完全吻合 |
| DownloadState.status 值 | PASS — idle/downloading/success/failed 正确 |
| LoginRequest schema 字段 | PASS — username/password 与代码一致 |
| project-status.html 数字 | PASS（MINOR：PASS=17/SKIP=1 为 T-005 完成后的值，可接受） |
