# 03 — Gate Review · T-017 install-role-and-public-ip

> Stage 3（Gate Reviewer）—— 开发前最后一道闸门。8 维度审计 + 实际打开文件核对设计引用。

## §1 审计输入（已读，全部 verbatim）

- `docs/features/install-role-and-public-ip/INPUT.md`（1-30）
- `docs/features/install-role-and-public-ip/01_REQUIREMENT_ANALYSIS.md`（1-416）
- `docs/features/install-role-and-public-ip/PM_LOG.md`（1-61）
- `docs/features/install-role-and-public-ip/02_SOLUTION_DESIGN.md`（1-596）
- `AI-GUIDE.md` / `.harness/rules/00-core.md` / `.harness/rules/50-fullstack.md` / `.harness/rules/05-insight-index.md`
- `.harness/insight-index.md`（1-38，38 条）
- `scripts/install.sh`（1-325，全文）
- `scripts/install-service.sh`（1-233，全文）
- `scripts/install.ps1`（1-286，全文）
- `scripts/uninstall-service.sh`（1-89，全文）
- `internal/appconf/config.go`（1-146，全文）
- `cmd/frp-easy/main.go`（1-424，全文）
- `internal/httpapi/handlers_system.go`（L43-214 片段）
- `scripts/verify_all.sh`（1-294，全文）

## §2 8 维度审计表

| # | 维度 | 状态 | 证据 |
|---|---|---|---|
| 1 | 需求完整性 | **PASS** | 02 §1.1 G-1~G-10 对照 01 §3 的 35 FR + §4 12 BC + PM 决议 8 条全部 1:1 映射；OOS-1~12 完整继承 |
| 2 | 设计可行性 | **PASS** | bash heredoc 单引号、`chown` 局部、curl `--max-time 3` 是现成机制；公网 IP 三候选 URL 国内有降级风险但 BC-1 已覆盖 |
| 3 | 复用审计核实 | **PASS（带注解）** | 18 项复用逐项核对：appconf struct tag L36-39 ✓；fetchPublicIP L155-201 ✓（设计写 L155-200 偏 1 行）；systemd_escape_path L19-34 ✓（设计写 L20-34 偏 1 行）；install-service.sh L69-75 ✓（设计写 L68-75 偏 1 行）；install.sh 步骤分块 L80/L99/L124/L158/L187/L227/L249/L255/L270/L292 100% 对齐 |
| 4 | 接口与不变量 | **PASS** | 退出码 0/1/2 既有 + 新 3 不冲突；toml 字段名 UIBindAddr/UIPort/DataDir/LogDir 严格 = struct tag；T-011 NF-2 用户显式值优先靠 D1 保留；T-014 frp_linux/ 升级保留靠 §6.2 不动 cp + 仅 chown |
| 5 | 测试可行性 | **WARN** | M-3：02 §12 提到"bash 函数 source 测试"但 install.sh 是 top-level 主流程；推荐 Developer 选"不做 source 测试，QA 走集成"路线 |
| 6 | 回归风险 | **WARN** | M-1：install.sh 中 RUN_USER 表达式必须与 install-service.sh L69-74 **完全等价**；M-2：T-016 `set +e/-e + rc=$?` 模式在 §6.5 chown 块的 fail-fast 决策 |
| 7 | insight-index 红线 | **WARN** | 关键核实 C 已确认 verify_all A.1 不会误命中预生成 toml（关键词谓词 api_key/secret/password/token 不出现于字段名旁）；但 Developer 自查务必遵循 "字段名 = 默认值" 模板，不要在 heredoc 内出现这四关键词 |
| 8 | OOS 完整性 | **PASS** | 01 §6 12 条 OOS + 02 §10 重申；分区只有 dev-backend；dev-frontend / dev-db 不参与 |

**计数**：PASS:5 / WARN:3 / FAIL:0；Critical:0 / Major:3 / Minor:5。

## §3 关键核实 A-G 结果

- **A. install.sh 步骤分块行号**：PASS。实际：步骤 0=L30、步骤 1=L79、步骤 2=L98、步骤 3=L123、步骤 4=L157、步骤 5=L186、步骤 6=L227（升级 L228 / 全新 L249）、步骤 7=L255、步骤 8=L291。L253(fi)~L255(注释) 之间正好是 §6.5 块的插入点。
- **B. appconf TOML 大小写**：PASS。`pelletier/go-toml/v2` 大小写敏感；02 §4.2 模板 `UIBindAddr` 等照搬 struct tag，匹配。
- **C. verify_all A.1 secrets 正则**：PASS。`(api[_-]?key|secret|password|token)` 关键词谓词不出现在预生成 toml 文本附近 → 不命中。
- **D. RUN_USER 表达式同源**：**MAJOR M-1**。install-service.sh L69-75 用两段式 if-then-else；02 §7 复用审计写 `${SUDO_USER:-$(id -un)}` 是等价简写但未明示 Developer 用哪种 → Developer 必须 verbatim 复制两段式。
- **E. DataDir 相对路径解析**：**MINOR m-1**。`./.frp_easy` + `WorkingDirectory=/opt/frp-easy` → `/opt/frp-easy/.frp_easy`，02 §7/§8 未显式确认 → 04 阶段补一句 + 06 阶段加验证。
- **F. FRP_EASY_PUBLIC_IP short-circuit 位置**：**MINOR m-2**。02 §3.1 函数契约缺失，Developer 在 detect_public_ip 函数首行补 `[[ -n "${FRP_EASY_PUBLIC_IP:-}" ]] && { ... return 0; }`。
- **G. 公网 IP 候选 URL 国内可达性**：**MAJOR M-2**。api.ipify.org / ifconfig.me / icanhazip.com 在腾讯云国内 VM 上**预期高概率全部失败**。BC-1 降级会被频繁触发——Developer 必须确保失败横幅显式打印 `FRP_EASY_PUBLIC_IP=<IP> sudo -E bash ...` 样例 + "国内 VM 出口 IP 可登云控制台复制"提示。

## §4 发现问题清单（按严重度）

### Critical（0 条）

无。

### Major（3 条）

- **M-1**（关键核实 D · 路由：Developer @ 04）：install.sh §6.5 块的 RUN_USER 解析**必须 verbatim 复制** `install-service.sh` L69-75 的两段式 if-then-else；并加注释 "与 install-service.sh L69-75 必须保持等价"。
- **M-2**（关键核实 G · 路由：Developer @ 04）：detect_public_ip 在国内 VM **预期失败**。必须：① 函数首行实现 `FRP_EASY_PUBLIC_IP` short-circuit；② 步骤 8 server 失败横幅打印 `FRP_EASY_PUBLIC_IP=<IP> sudo -E bash ...` 样例 + "登云控制台"提示。
- **M-3**（维度 5 · 路由：Developer @ 04）：保持 install.sh top-level 主流程，**不**做 `[[ "${BASH_SOURCE[0]}" == "$0" ]]` source-mode 包裹；QA 走集成测试。

### Minor（5 条）

- **m-1**（关键核实 E）：04 §实现说明补 `./.frp_easy → /opt/frp-easy/.frp_easy` 解析链；06 加 `[ -d /opt/frp-easy/.frp_easy ]` 验证。
- **m-2**（关键核实 F）：04 显式记 FRP_EASY_PUBLIC_IP short-circuit 在 detect_public_ip 函数首行。
- **m-3**（复用审计行号偏 1）：02 §2 几处行号偏 1 行——可接受，不影响实现。
- **m-4**（§6.5 插入位置）：02 §1.2 Inv-3 提到"L253/L255 之间"；L254 实际为空行，§6.5 块插在 L253(fi) 后即可。
- **m-5**（Windows 横幅）：install.ps1 横幅注释明示 "Windows 安装期暂无 role 区分（OOS-2），公网 IP 探测一律执行" 以免 QA 误报。

## §5 Verdict

**APPROVED WITH CONDITIONS**

理由：
1. 设计完整覆盖 8 条 PM 决议、35 条 FR、12 条 BC；Inv-1~Inv-7 不变量守护 T-011/T-014/T-016；复用审计 18 项核对全对齐。
2. **无 critical 红线触犯**：
   - 无 `$0`/`$BASH_SOURCE` 自定位（insight L32）
   - sudo `id -un` 用 `SUDO_USER:-` 优先（insight L22）
   - heredoc 用 `<<'EOF'` 单引号封禁插值（insight L38）
   - verify_all A.1 不会误中预生成 toml（关键核实 C）
   - 06_TEST_REPORT.md 英文 `## Adversarial tests` 已在 02 §12.4 显式要求（insight L31）
3. 3 条 major 都是"实现期需 Developer 显式补做"的事项，**不需要**回退 Architect。
4. 5 条 minor 是文档级清理 + 边角说明，不阻塞实现。

## §6 APPROVED WITH CONDITIONS — Developer 在 04 阶段必做的补救

1. **C-1（M-1）**：install.sh §6.5 块的 RUN_USER 解析 **verbatim 复制** `install-service.sh` L69-75 两段式 if-then-else；加注释 "与 install-service.sh L69-75 必须保持等价，更改时同步两处"。
2. **C-2（M-2）**：detect_public_ip 函数首行实现 `FRP_EASY_PUBLIC_IP` 环境变量 short-circuit（合法 IPv4 字面量则直接返回，跳过 3 候选探测）；步骤 8 server 失败横幅打印 `FRP_EASY_PUBLIC_IP=<your-ip> sudo -E bash ...` 样例 + "国内 VM 可登云控制台复制出口 IP"提示。
3. **C-3（M-3）**：install.sh 保持 top-level 主流程结构，**不**做 source-mode 包裹；QA 走集成测试，不依赖单函数 source 测试。
4. **C-4（m-1）**：04 §实现说明显式记录 `DataDir = './.frp_easy'` 在 `WorkingDirectory=/opt/frp-easy` 下解析为 `/opt/frp-easy/.frp_easy`；QA 加 `[ -d /opt/frp-easy/.frp_easy ]` 验证。
5. **C-5（m-2）**：detect_public_ip 函数文档注释明示 FRP_EASY_PUBLIC_IP short-circuit 位置。
6. **C-6（m-5）**：install.ps1 横幅块旁注释 "Windows 路径暂无 role 区分（OOS-2），公网 IP 探测一律执行"。
7. **C-7（chown 失败处理）**：§6.5 块的 chown **不允许** `|| true` 静默吞失败；失败应 exit 1 + 中文错误（与 install.sh L80-95 错误模板风格一致）。frp_linux/ 目录若不存在则 `mkdir -p` 后再 chown，避免 chown 不存在路径报错。
8. **C-8（adversarial tests）**：06_TEST_REPORT.md 必含**英文** `## Adversarial tests` 标题（insight L31 verify_all E.6），覆盖至少 02 §12.4 列出的 4 项 + verify_all PASS:19。
9. **C-9（07 insight 收割）**：07_DELIVERY.md `## Insight` 段建议收割两行：
   - "install.sh 解包后必须 chown 给 RUN_USER 才能让 systemd User= 进程写 frp_easy.toml，否则 appconf.Load 写默认失败 → 死循环重启"
   - "公网 IP 探测在国内 VM 上 3/3 候选 URL 失败是预期，FRP_EASY_PUBLIC_IP 用户手动覆盖通道必须存在"

---

**Verdict**：**APPROVED WITH CONDITIONS**（9 条 Developer 必做补救）
**计数**：PASS:5 / WARN:3 / FAIL:0；Critical:0 / Major:3 / Minor:5
**最危险风险**：国内 VM 上 3/3 候选 URL 高概率失败，`FRP_EASY_PUBLIC_IP` 兜底通道必须在文案与函数实现层显式存在。
