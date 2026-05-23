# 07 DELIVERY · T-018 upload-bin-multiport-ip-probe

> Delivered by PM Orchestrator
> 日期：2026-05-23
> 结果：**DELIVERED**

## 1. 任务总览

三个 UX 增强合并为单任务 T-018：

| 模块 | 用户诉求 | 落地形态 |
|---|---|---|
| **A 上传** | "frpc/frps 容易下载失败，加上传入口" | AppLayout banner 内并列两按钮（下载 / 上传），后端 `POST /api/v1/system/upload-bin`，64 MiB 上限，平台头校验（ELF/MZ），原子落盘 |
| **B 公网 IP** | "用 ip.cn 等大陆网站校验" | 5 源并发探测（ipify + my-ip.io + ip.cn + ipw.cn + bilibili 等），保留国际源 + 加大陆源，HTML 私有段过滤，`FRP_EASY_PUBLIC_IP` Go 端 env 短路 |
| **C 多端口** | "FRP 支持多端口，UI 不支持，加预设/探测" | 批量端口 `POST /api/v1/proxies/batch`（portsExpr 解析 `6000-6010,7000`，单事务回滚），10 个常用端口预设 NTag 快速选择，`POST /api/v1/system/probe-ports` 探可用性（≤64 端口，dual-stack wildcard），前端折叠分组显示 |

## 2. 改动清单

### 后端（10 新 + 9 改）
- 新增：`internal/portrange/{portrange.go, portrange_test.go}` / `internal/downloader/{install.go, install_test.go}` / `internal/storage/proxies_batch_test.go` / `internal/httpapi/{handlers_upload_test.go, handlers_system_publicip_test.go, handlers_batch_test.go, port_probe_test.go}`
- 修改：`internal/downloader/downloader.go`（doDownload 改调 Install）/ `internal/storage/{store.go, proxies.go}`（UpsertProxiesTx 持 s.mu）/ `internal/httpapi/{handlers_system.go, handlers_proxies.go, router.go}` / `openapi.yaml` / `docs/dev-map.md`

### 前端（11 新 + 4 改）
- 新增：`web/src/composables/{usePortPresets.ts, useProxyGrouping.ts}` / `web/src/components/UploadBinButton.vue` + 5 个新 vitest spec
- 修改：`web/src/types.ts` / `web/src/api/{system.ts, proxies.ts}` / `web/src/stores/proxies.ts` / `web/src/components/{AppLayout.vue, ProxyForm.vue}` / `web/src/pages/Proxies.vue` / `docs/dev-map.md`

### 闸门 / 测试
- 修改 `scripts/baseline.json` v6→v7（test_count 231→333，go 174→237 / frontend 57→96）

## 3. verify_all 实测

```
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO/FIXME budget ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agents in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md ... PASS
[E.6] Adversarial tests section in completed task reports ... PASS

=== Summary ===
  PASS: 19   WARN: 0   FAIL: 0   SKIP: 0
```

## 4. 流水线复盘

| 阶段 | 关键事件 |
|---|---|
| 1 需求 | 10 条 PM-DECIDED 自填（用户授权"你来决策"） |
| 2 设计 | dev-backend + dev-frontend 并行分区；dev-db 不参与 |
| 3 闸门 | **首轮 CHANGES REQUIRED**（3 P0 + 3 P1 + 7 P2），Architect 修订 v2 后二次 **APPROVED** |
| 4 开发 | 后端 14 包 PASS / 前端 vitest 96 PASS / 自跑 verify_all PASS:19 |
| 5 评审 | **首轮 CHANGES REQUIRED**（2 P0 字段名漂移 + 1 P1 上限值偏离），dev 修复后 **APPROVED FOR QA** |
| 6 QA | 30+ AC 全部 PASS + 20 个 adversarial reproducer 全 PASS |
| 7 交付 | baseline v6→v7，verify_all PASS:19 |

**亮点**：流水线 7 阶段全部走完，**两轮人工反馈**（Gate Review + Code Review）各自捕获了不同层级的问题（前者是文档级 / API 签名级，后者是契约级），证明 PM 必须严格执行"独立评审"而非走过场。

**痛点**：前后端字段名漂移（`size↔sizeBytes` / `basename↔namePrefix`）在双方 mock 测试都 PASS 时无法被捕获 —— 这是本次最大的工程教训，候选 insight。

## 5. 用户视角变化

打开 Web UI 后用户会看到：

1. **顶部 banner（binary 缺失时）**：原"一键下载 frpc / 一键下载 frps"按钮**旁边**多了两个 "上传 frpc / 上传 frps" 按钮，网络受限时可手动上传二进制。
2. **代理规则页"新增规则"对话框**：
   - 顶部加 "快速选择" Tag 行：SSH (22) / RDP (3389) / HTTP (80) / HTTPS (443) / MySQL (3306) / PostgreSQL (5432) / Redis (6379) / MongoDB (27017) / SMB (445) / VNC (5900)，点击一键填表
   - 本地端口字段旁加 "探测可用性" 按钮，立刻告诉用户该端口本机是否被占
   - 加 "批量模式" 切换：basename + portsExpr（`6000-6010,7000`）一次创建多条
3. **代理规则列表**：按 name 前缀自动折叠分组，例 `web-6000~web-6010` 显示为 `web (TCP 6000-6010, 11 条)`，可展开
4. **公网 IP 检测**：在国内 VM 上不再因为 ipify/my-ip.io 全 timeout 而失败 —— 5 源（国际 + 大陆混合）并发，任一成功就返回；可用 `FRP_EASY_PUBLIC_IP=x.x.x.x` 环境变量强制覆盖

## 6. 已知限制 / Follow-up

- **L-1**：反代部署时 `client_max_body_size` 必须 ≥ 64 MiB（已文档化为 02 R-16；建议在 README 部署章节加一行）。
- **L-2**：端口探测存在竞态（"先探可用 → 立即被占 → 保存 → frpc 启动失败"），由 procmgr lastErr 暴露，UI 文案明确"探测仅供参考"。
- **L-3**：HTML 公网 IP 源仅抓 IPv4（B 模块 R-9 已说明）。
- **L-4**：批量创建无法跨 type 混合（一次只能 tcp 或 udp，http/https 用 customDomains 不适用批量），UI 在批量模式自动锁 type 为 tcp/udp。

## 7. 配置 / 部署变化

- 新增可选环境变量 **`FRP_EASY_PUBLIC_IP`**（运行期）：若设置且为合法 IP，公网 IP 检测直接返回此值不发出站请求。
- 无 migration、无 schema 变更、无配置文件格式变化。
- 二进制 vendoring 无变化（frp 仍在运行时下载 / 现可手动上传）。
- 滚动 tag 发布按 release.yml 走（main 推送即触发）。

## 8. 提交策略

本次三个模块共一个 commit + 一次 push：

```
feat(T-018): upload-bin-multiport-ip-probe — 二进制手动上传 + 公网 IP 大陆源 + 多端口批量/预设/探测

verify_all PASS:19
```

## Insight

- **2026-05-23** · 前端 TS 接口与后端 Go struct 的 JSON 字段名漂移在双方 mock 测试都 PASS 时无法被捕获；本任务出现两处 P0：`size↔sizeBytes` 与 `basename↔namePrefix`，前端 spec mock 用自定字段名，后端单测用 OpenAPI 字段名，各自绿但生产必崩。补救：spec 测试用 OpenAPI codegen（如 openapi-typescript）做"契约一锤定音"，而非两边各自从 OpenAPI 抄一遍 · evidence: T-018 05_CODE_REVIEW P0-1/P0-2
- **2026-05-23** · `scripts/verify_all.sh` 的 E.6 regex 是 `^##\s+Adversarial\s+tests`，**不允许数字编号前缀**（如 `## 2. Adversarial tests` 会 FAIL）；insight L31 已警告精确英文但未明示禁子编号；写 QA 06 时标题必须是裸 `## Adversarial tests` 不带任何前缀 · evidence: T-018 verify_all 首跑 E.6 FAIL
- **2026-05-23** · gate-reviewer / code-reviewer 等 review 类 sub-agent 倾向把完整 review 内容返回到消息体而不写入对应 `0X_*.md` 文件；派发时 prompt 必须显式 "**必须直接写到 <文件名> 文件**" 才稳；否则 PM 要手工落盘 · evidence: T-018 stage 3/5 两次 reviewer 不落盘
