# 03 GATE REVIEW · T-018 upload-bin-multiport-ip-probe

> Reviewer：Gate Reviewer（首轮）
> 日期：2026-05-23
> 输入：
> - `01_REQUIREMENT_ANALYSIS.md`
> - `02_SOLUTION_DESIGN.md`

## 1. 八维度审计

| # | 维度 | 结果 | 一句话原因 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 三诉求各自独立有 FR + AC，AC-A/B/C 共 29 条，全部可测；PM-DECIDED 10 项无 Open Questions。 |
| 2 | 设计完整性 | PASS | 每个 FR 在 02 都能找到落点（A.2 / B.1 / C.1-C.3），有 OpenAPI snippet 与 sequence。 |
| 3 | 复用正确性 | **WARN** | 复用表 §8 大致正确，但 §A.2 用 `ProcMgr.GetStatus(kind)` 方法名不存在（实际是 `Status(kind)`），返回签名也对不上；§B 假设"沿用 T-017 env 短路"，实际 Go 端**从未实现**此短路（只在 install.sh / install.ps1）。 |
| 4 | 风险覆盖 | **WARN** | §9 覆盖了 13 项风险，但漏了两个 P0 级具体陷阱：(a) axios 1.x 显式 `Content-Type: multipart/form-data`（无 boundary）会让请求失败；(b) sqlite 单连接 + `s.mu` 写锁与 `UpsertProxiesTx` 的事务交互未在设计中显式约束。 |
| 5 | 迁移安全 | PASS | 无 schema 改动、无 migration、无 feature flag。所有改动加性。回滚 = 滚动 tag 回退。 |
| 6 | 边界处理 | PASS | A/B/C 的 boundary 段共 25+ 条；空、超大、并发、特权端口、HTML 污染都覆盖了。 |
| 7 | 测试可行性 | PASS | 每条 AC 都有对应单测/handler 测；portrange table-driven 11 条；IP 源用 httptest + RoundTripper mock；端口探测用 `:0` 临时端口拿真实 port。 |
| 8 | Out-of-scope 清晰 | PASS | §11 显式列出 8 项不做。`dev-db` 不参与已明确。 |

## 2. 发现（按模块分组）

### B-1（B - 公网 IP）[P0] 设计假设 `FRP_EASY_PUBLIC_IP` Go 端已存在，实际不存在
- **位置**：02 §3.B.1（`if v := strings.TrimSpace(os.Getenv("FRP_EASY_PUBLIC_IP")); v != ""`）+ §13（"严格保留 FRP_EASY_PUBLIC_IP 短路"）。
- **问题**：grep `FRP_EASY_PUBLIC_IP` 在 `internal/`、`cmd/` 下零命中。T-017 的 short-circuit 是写在 `scripts/install.sh` 与 `scripts/install.ps1` 中，并未保证 Go 后端实现。设计文档把它当作"既有基线"来用，实际是**新引入 Go 代码**。
- **建议**：02 §3.B.1 加一行 "**NEW**：Go 端首次引入 `FRP_EASY_PUBLIC_IP` 读取（之前仅 install.sh）。本任务把它从安装期扩展到运行期"。

### B-2（A - 上传二进制）[P0] axios 显式 `Content-Type: multipart/form-data` 会丢 boundary
- **位置**：02 §2.A.3（`apiUploadBin` 的 `headers: { 'Content-Type': 'multipart/form-data' }`）。
- **问题**：axios 1.x 行为：用户不设 Content-Type 时自动用 `multipart/form-data; boundary=…`；显式设 Content-Type 时不再追加 boundary → 服务端 `MultipartReader` 解析失败。
- **建议**：02 §A.3 删除该 headers 行（留给 axios 自动加 boundary）；handler 测试用真实 FormData 才能捕获回归。

### B-3（A - 上传二进制）[P0] `ProcMgr.GetStatus(kind)` 方法名不存在
- **位置**：02 §2.A.2 `if info, ok := h.deps.ProcMgr.GetStatus(kind); ok && info.State == "running"`。
- **问题**：`internal/procmgr/manager.go` 实际签名是 `func (m *Manager) Status(kind string) ProcessInfo`（单返回值，无 ok）；`State` 是 `procmgr.State`（string alias），应用 `procmgr.StateRunning` 常量比对。
- **建议**：02 §A.2 步骤 7 改为：
  ```go
  if h.deps.ProcMgr != nil {
      info := h.deps.ProcMgr.Status(kind)
      if info.State == procmgr.StateRunning {
          advisory = "上传成功；如需立即生效请到运行控制重启 " + kind
      }
  }
  ```

### B-4（A - 上传二进制）[P1] Wizard.vue 中没有下载按钮可挂载
- **位置**：01 §A.1 FR-A.1 "Wizard 与设置页两处"；02 §A.3 "现有'一键下载'按钮在 AppLayout.vue 与 Wizard.vue 两处"。
- **问题**：`web/src/pages/Wizard.vue` 实测没有任何下载/二进制相关 UI（grep `frpc / frps / 二进制 / handleDownload / downloaderStore` 全无命中）。下载按钮**仅在** `AppLayout.vue` L21-29 的 binMissing banner 内。
- **建议**：02 §A.3 收紧挂载范围到 `AppLayout.vue` banner；若仍要在 Wizard 入口，明示"新增"（非现状沿用）。01 §FR-A.1 文案同步修正。

### B-5（C - 批量端口）[P1] `UpsertProxiesTx` 与 `s.mu` 的并发约束未写
- **位置**：02 §4.C.1.3（`UpsertProxiesTx` 说明 "不复用 mu.Lock"）。
- **问题**：`internal/storage/store.go` `SetMaxOpenConns(1)` → sqlite 单连接，所有写靠 `s.mu` 串行化。若 `UpsertProxiesTx` 不持 `s.mu`，事务内 INSERT 与并行 `UpsertProxy/DeleteProxy` 共享物理连接 → `database is locked` 概率虽低但语义错误。
- **建议**：02 §C.1.3 显式约束 "`UpsertProxiesTx` 整段持 `s.mu.Lock()`"；storage 单测加并发用例。

### B-6（A - 上传二进制）[P1] MultipartReader 读取顺序假设过强
- **位置**：02 §A.2（"`for` 循环里假设客户端先发 kind 后发 file"）。
- **问题**：浏览器 / curl / 各 client 对 `FormData` append 顺序跨平台并非强保证。流式 reader 若 file 先到，循环 `break` 后 `kind` 还没读 → 422 "缺字段 kind"。
- **建议**：改 `r.ParseMultipartForm(maxMemory)` + `r.FormValue("kind")` + `r.FormFile("file")`，更稳。或保留流式但锁死前后端契约 "前端 fd.append('kind', kind) 必须先于 fd.append('file', file)"。

### B-7（B - 公网 IP）[P2] 并发 5 源对外部站点的影响
- **位置**：02 §B.1（`cancel()` 触发所有 in-flight 取消）。
- **问题**：第一个成功后取消其它，但 HTML 源（ip.cn）可能已开始拉 256 KiB body。每次 cache miss 会对 5 个站点全部发请求。
- **建议**：分波探测（先国内 2 源、3s 仍无果触发国际 3 源），或文档化 "cache miss 最多 ~600 KiB 出站流量"。

### B-8（C - 端口探测）[P2] modernc sqlite UNIQUE 文本对复合索引格式未实证
- **位置**：02 §13 表 L16。
- **问题**：insight-index L16 给的样例是单列 `proxies.name`。复合 UNIQUE INDEX 在 modernc 实际报错文本未在历史任务中验证。
- **建议**：Developer 实现 `isDuplicateTcpRemote` 前先写 storage adversarial 单测获得真实 err.Error()，再据此写 `strings.Contains` 关键字。02 §C.1.3 加一行 "实际关键字以单测断言为准"。

### B-9（C - 端口探测）[P2] `0.0.0.0:port` 在 Windows 上语义有别
- **位置**：02 §C.3.2。
- **问题**：Windows 上 `net.Listen("tcp", "0.0.0.0:N")` 仅探 IPv4 wildcard；若被 IPv6-only 监听者占用，会**误判为可用**。
- **建议**：改 `net.Listen("tcp", fmt.Sprintf(":%d", port))`（dual-stack wildcard），或文档化此局限作为 R-13。

### B-10（A）[P2] 64 MiB 上限与反代默认值不一致
- **位置**：02 §9 R-1。
- **建议**：QA 阶段在 06_TEST_REPORT 的 Adversarial 段加一条"反代 client_max_body_size 不足"实测。

### B-11（A - 文件头校验）[P2] PE 偏移 0x3C 校验注释自相矛盾
- **位置**：02 §A.2 `validateBinaryHeader`。
- **问题**：同时声称"PE 进一步校验 offset 0x3C 处的 PE\0\0"和"64-byte peek 可能不够；不强校验，仅 MZ 即可"——行为不确定。
- **建议**：要么 peek 256 字节做完整 PE\0\0 校验；要么注释明确 "仅 MZ 即接受，落盘后启动失败由 procmgr `lastErr` 暴露"。

### B-12（C - 批量端口）[P2] 前端折叠分组正则错位
- **位置**：02 §C.1.4 "按 `name` 的 `^([^-]+)-(\d+)$` 模式 group"。
- **问题**：basename 含 `-`（如 `my-web`）时 `my-web-6000` 会被错切。
- **建议**：正则改 `^(.+)-(\d{1,5})$`（greedy 取最后一段数字）；测试覆盖含连字符的 basename。

### B-13（CC）[P2] errgroup 未引入已确认；HTML 源仅抓 IPv4 已隐含
- **位置**：CC-2 / 02 §B.1。
- **结论**：02 用 `sync.WaitGroup` + `context`（stdlib），未引入新依赖。✓

## 3. Insight-index 命中

| Insight | 体现 | 状态 |
|---|---|---|
| L10 Windows os.Rename | A 的 Install 沿用 doDownload Remove+Rename | ✓ |
| L17 AtomicWrite 双 chmod | A 的 Install rename 后 chmod | ✓ |
| L16 modernc UNIQUE 文本 | C.1 `isDuplicateTcpRemote` | △ 复合索引格式未实证（B-8） |
| L28 GOOS 可注入 seam | install_test 沿用 goosFunc | ✓ |
| L31 `## Adversarial tests` 英文标题 | 提示 QA | ✓ |
| L37 GitHub API UA | B 所有出站带 UA=frp_easy | ✓ |
| L40 国内公网 IP + env 短路 | B 保留 env 短路 | △ Go 端是新引入（B-1） |
| T-006 NMessageProvider | App.vue 已包 | ✓ |

## 4. Verdict

**CHANGES REQUIRED**

设计大体过硬，但有 **3 个 P0 + 3 个 P1** 必须回 Architect 修：

- **P0**：B-1（FRP_EASY_PUBLIC_IP Go 端是新引入）/ B-2（axios Content-Type 不能显式设）/ B-3（ProcMgr 方法名签名错）
- **P1**：B-4（Wizard 没下载按钮）/ B-5（UpsertProxiesTx 须持 s.mu）/ B-6（MultipartReader 顺序假设过强）
- **P2**：B-7 ~ B-13 可在 Architect 修订时一并吸收，或在 Developer 阶段以注释/小补丁吸收。

修订后**无需重走 Stage 1**（01 文档结构 OK，仅 FR-A.1 措辞需同步收紧）。

---

## 二次评审 · 2026-05-23

### 修订核验表

| Finding | Severity | 吸收状态 | 核验位置 / 说明 |
|---|---|---|---|
| B-1 | P0 | ✓ | 02 §3.B.1 注释明确 "**NEW（B-1 修订）**：Go 端首次引入 FRP_EASY_PUBLIC_IP 读取（之前仅 install.sh / install.ps1）"；§8 复用表新增一行"运行期 FRP_EASY_PUBLIC_IP env 短路（Go 端）"标 **NEW**。 |
| B-2 | P0 | ✓ | 02 §A.3 `apiUploadBin` 的 axios 配置对象内确实没有 `headers` 字段，仅保留 `onUploadProgress` 与 `timeout`；加 "B-2 修订：禁止显式设置 Content-Type" 注释。 |
| B-3 | P0 | ✓ | 02 §A.2 改为 `info := h.deps.ProcMgr.Status(kind)`（单返回值）+ `info.State == procmgr.StateRunning` 常量比对。 |
| B-4 | P1 | ✓ | 02 §A.3 "挂载点（B-4 修订收紧）" 段说明仅在 AppLayout.vue banner 挂；§12 Partition 表 Wizard.vue 一行加删除线 + "B-4 修订移除"；01 §FR-A.1 / §AC-A.9 已同步收紧。 |
| B-5 | P1 | ✓ | 02 §C.1.3 "B-5 修订（并发约束）" 段明确 `UpsertProxiesTx` 整段持 `s.mu.Lock()`，附伪代码骨架；§C.1.5 storage 测试表追加 `UpsertProxiesTx_ConcurrentWithUpsertProxy` 并发用例。 |
| B-6 | P1 | ✓ | 02 §A.2 handler 重写为 `MaxBytesReader → ParseMultipartForm(8 MiB) → FormValue + FormFile` 模式；显式区分 `*http.MaxBytesError`→413 与"非 multipart"→400。 |
| B-7 | P2 | ✓ | 02 §9 新增 R-14，选择"保持并发 5 源"而非分波探测，给出三条理由（5min 缓存压频 / 分波破坏 NF-B.1 / 600 KiB 可接受）。 |
| B-8 | P2 | ✓ | 02 §C.1.3 "B-8 修订" 段强制 Developer 先写 storage adversarial 单测捕获 modernc 复合 UNIQUE 真实 `err.Error()`，再据此写 `strings.Contains`。 |
| B-9 | P2 | ✓ | 02 §C.3.2 `probeOnePort` 改为 `net.Listen("tcp", fmt.Sprintf(":%d", port))`（dual-stack wildcard），附注释；§9 新增 R-15。 |
| B-10 | P2 | ✓ | 02 §A.4 测试点表追加 Adversarial `nginx_client_max_body_size` 条目；§9 新增 R-16。 |
| B-11 | P2 | ✓ | 02 §A.2 `validateBinaryHeader` 注释明确 "**仅 MZ 即接受 PE**。不再做 offset 0x3C 处的 PE\0\0 二次校验"，消除自相矛盾；代码与注释一致。 |
| B-12 | P2 | ✓ | 02 §C.1.4 折叠正则改为 `^(.+)-(\d{1,5})$`（greedy + 限定 1-5 位数字尾），举例 `my-web-6000` 正确切；前端单测要求覆盖 5 种用例。 |
| Q1 | – | ✓ | 02 §A.2 `Install` 函数注释明确 "`maxBytes <= 0` 表示不限大小（下载链路走此分支）；upload 链路必须传 > 0 的明确上限"。 |
| Q2 | – | ✓ | 02 §5.1 OpenAPI `BatchProxiesResponse` 已是对象版 `{created, items}`，与 §C.1.1 响应 201 例子一致；01 §FR-C.1.6 同步改写。 |

### 残留小瑕疵（不阻塞）

1. **02 §A.3 axios 调用旁残留陈旧注释**：`fd.append('file', file)` 行旁可能仍留 "必须在 kind 之后 append" 一类注释（B-6 修订后字段顺序无关）。**不阻塞**，Developer 顺手清理即可。

### 二次 Verdict

**APPROVED FOR DEVELOPMENT**

v2 已逐条吸收首轮全部 P0/P1（B-1~B-6）+ P2（B-7~B-12）+ Q1/Q2，复用表与风险表同步增补；01 三处文案（FR-A.1 / AC-A.9 / FR-C.1.6）也已同步。可入 Stage 4 Developer。
