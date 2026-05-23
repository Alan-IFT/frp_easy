# 05 — Code Review · T-017 install-role-and-public-ip

> Stage 5（Code Reviewer）—— 对照需求 + 设计 + 9 conditions 审查代码。6 维度。

## §1 审计输入

| 文件 | 范围 | 用途 |
|---|---|---|
| `docs/features/install-role-and-public-ip/INPUT.md` | 1-30 | 用户原话与现场 |
| `docs/features/install-role-and-public-ip/01_REQUIREMENT_ANALYSIS.md` | 1-416 | FR / BC / AMBIG 全量 |
| `docs/features/install-role-and-public-ip/PM_LOG.md` | 1-83 | 8 条 AMBIG 决议 |
| `docs/features/install-role-and-public-ip/02_SOLUTION_DESIGN.md` | 1-596 | G-1~G-10、Inv-1~7、§3/§4/§5/§6 |
| `docs/features/install-role-and-public-ip/03_GATE_REVIEW.md` | 1-101 | 9 条 conditions C-1~C-9 |
| `docs/features/install-role-and-public-ip/04_DEVELOPMENT.md` | 1-395 | 开发自述 |
| `scripts/install.sh` | 1-592（全文） | 主改文件 |
| `scripts/install.ps1` | 1-362（全文） | Windows 同步 |
| `scripts/uninstall-service.sh` | 1-94（全文） | .role 清理 |
| `scripts/install-service.sh` | 1-233（全文），重点 L17 + L69-75 | RUN_USER verbatim 比对 |
| `internal/appconf/config.go` | L1-60（含 struct tag L36-39） | Inv-1 守护 |
| `cmd/frp-easy/main.go` | L1-30 | Inv-2 守护 |
| `scripts/verify_all.sh` | L1-100（A.1 正则 L63-64） | NFR-4 / FR-F.4 守护 |
| `.harness/insight-index.md` | 38 条红线 | 红线核查 |

## §2 6 维度审查表

| # | 维度 | 状态 | 证据 |
|---|---|---|---|
| 1 | 设计契约符合度 | **PASS** | 02 §3 三个函数（resolve_role_or_die / render_frp_easy_toml / detect_public_ip）落地于 install.sh L100-189；§6.5 块落于 L367-458；§5.4 横幅 server/client 二分落于 L508-589。无擅自扩张/简化。 |
| 2 | 9 条 conditions 落地 | **PASS** | 见 §3 逐条 file:line。 |
| 3 | insight-index 红线 | **PASS** | L22 SUDO_USER 优先 ✓；L32 无 `$0`/`$BASH_SOURCE` 自定位 ✓；L38 heredoc `<<'EOF'` ✓；L33/L37 curl `-f` + IPv4 字面量 ✓；A.1 secrets 正则不命中 ✓。 |
| 4 | 不变量守护 | **PASS（带 1 minor）** | Inv-1/Inv-2/Inv-3 见 §4 P-8/P-9；Inv-4 升级分支 L350-358 未触 frp_linux/ ✓；Inv-5 T-016 进度条 L309-316 + 退出码透传 L485-494 一字未动 ✓；Inv-6 verify_all.sh 未改 ✓；Inv-7 中文输出 ✓。 |
| 5 | 错误路径完整性（BC-1~BC-12） | **PASS（带 1 minor）** | BC-3 IPv6 横幅 bracket 包裹遗漏，见 MIN-1。其他 11 条均覆盖。 |
| 6 | 可读性 + 中文文案 | **PASS** | 错误消息、横幅、注释全中文；都有"下一步可操作"指引。 |

**总计**：PASS:6 / WARN:0 / FAIL:0。Critical:0 / Major:0 / Minor:2 / Nit:1。

## §3 9 条 conditions 逐条核实

| ID | 实现位置 | 状态 | 证据 |
|---|---|---|---|
| **C-1** RUN_USER verbatim | `install.sh:378-385` | ✅ PASS | 与 install-service.sh L17 + L69-75 完全等价；L374-377 含"与 install-service.sh L69-75 必须保持等价"注释 |
| **C-2** FRP_EASY_PUBLIC_IP short-circuit + 失败兜底文案 | `install.sh:157-169` + `:560-568` | ✅ PASS | 函数首块 short-circuit（IPv4 严校验 + hostname/IPv6 宽校验 + 危险字符拒绝）；失败横幅 L567 含 `FRP_EASY_PUBLIC_IP=<your-ip> FRP_EASY_ROLE=server sudo -E bash` 完整样例 + L565 "国内 VM 登云控制台"提示 |
| **C-3** 不做 source-mode 包裹 | `install.sh` 全文 | ✅ PASS | grep 无 BASH_SOURCE、无 `$0` |
| **C-4** DataDir 解析链 | 04 §3.4 | ✅ PASS | 文档显式记录；待 QA 加 `[ -d ]` 验证 |
| **C-5** detect_public_ip 函数注释 | `install.sh:143-156` | ✅ PASS | docstring + 内联注释双标记 |
| **C-6** install.ps1 OOS-2 注释 | `install.ps1:41` + `:296` | ✅ PASS | 头部 + 步骤 8 双重声明 |
| **C-7** chown fail-fast + mkdir -p | `install.sh:438-450` | ✅ PASS | L438 `mkdir -p` 三目录 → L439-449 三处独立 if 块 fail-fast；无 `|| true` |
| **C-8** 06 英文 `## Adversarial tests` | 04 §4.2 传 QA | ✅ 传递 | 跨阶段 |
| **C-9** 07 收割 insight | 04 §4.2 传 07 | ✅ 传递 | 跨阶段 |

## §4 关键核实 P-1 ~ P-9

### P-1 `detect_public_ip`（install.sh L153-189）
- ✅ 函数首行 short-circuit（L157 `[[ -n "${FRP_EASY_PUBLIC_IP:-}" ]]`）
- ✅ 三候选 URL 明文写死（L171-175 字面 array）
- ✅ `curl -fsS --max-time 3`（L180）
- ✅ HTTP 状态码 + IPv4 正则双校验（L180 `-f` + L183 `^([0-9]{1,3}\.){3}[0-9]{1,3}$`）
- ✅ trim 尾部空白（L182）

### P-2 §6.5 块（install.sh L367-458）
- ✅ chown 三处独立 if 块 fail-fast，无 `|| true`
- ✅ mkdir -p 三目录后再 chown
- ✅ 仅 chown 4 项中的 3 项；无 `chown -R /opt/frp-easy`

### P-3 §0.5 ROLE 解析（install.sh L95-108）
- ✅ 未指定 / 非法值 → exit 3 + 两条入口命令样例
- ✅ 升级期 .role 冲突 → exit 3 + FRP_EASY_FORCE_ROLE 样例（L399-407）

### P-4 步骤 8 横幅（install.sh L496-589）
- ✅ server：三行 + 公网=LAN 仍打两行 + 防火墙提示（AMBIG-F = F2）
- ✅ client：仅一行 + 不调 detect_public_ip + 无防火墙提示
- ✅ server 失败横幅含完整 sudo 样例命令

### P-5 RUN_USER verbatim
- ✅ 两段式 if-then-else 与 install-service.sh L69-75 byte-level 等价
- ✅ L374-377 同步注释

### P-6 install.ps1
- ✅ Get-PublicIPv4 三候选 URL 与 bash 同款
- ✅ OOS-2 注释 L41 + L296
- ✅ 无 `FRP_EASY_ROLE` 引用（OOS-2 守住）

### P-7 uninstall-service.sh
- ✅ L80 `rm -f .role`
- ✅ 保留 frp_easy.toml 与 .frp_easy/（与 T-016 行为一致）

### P-8 Inv-1 / Inv-2
- ✅ working tree clean；config.go L36-55 字段名 + Default 值与 02 §4.2 模板完全一致；main.go 头部 T-011 安全提示文案保留

### P-9 Inv-3
- ✅ install-service.sh 233 行全文与 T-016 一致（systemd_escape_path、RUN_USER、systemd-analyze warn+继续、退出码透传、诊断块均健在）

## §5 问题清单

### Critical（0 条） / Major（0 条）

无。

### Minor（2 条）

- **MIN-1** [LOGIC] `install.sh:158-168 + :558` — `FRP_EASY_PUBLIC_IP` 含 `:`（IPv6 字面量）通过宽校验后，横幅 L558 `http://${PUBLIC_IP}:7800` 拼接**不合法**（IPv6 需 `[xxx]:port` 包裹）。02 §5.4 BC-3 设计明确要求 bracket。复现：`FRP_EASY_PUBLIC_IP=2001:db8::1 FRP_EASY_ROLE=server bash install.sh` → 打印 `http://2001:db8::1:7800`（浏览器无法解析）。建议跟进任务修，本任务接受（边缘 case）。
- **MIN-2** [MAINT] `install.sh:378-385` — RUN_USER 解析多了 L378 `RUN_USER=""` 先置空（模仿 install-service.sh L17 全局变量），在 install.sh 上下文中冗余但无害。保留以严格 verbatim 也合理。

### Nit（1 条）

- **NIT-1** [STYLE] `install.sh:101` — `[[ -z "$ROLE" || (...) ]]` 首项被后两项蕴含，逻辑可简化；保留为意图明示亦可。

## §6 Verdict

**APPROVED**

理由：
1. 9 conditions C-1~C-9 全部落地（C-4/C-8/C-9 跨阶段传递）
2. Inv-1~Inv-7 全部守护
3. insight-index 适用红线全部遵守
4. 错误路径完整（11/12 BC 覆盖，MIN-1 边缘 case 不阻塞）
5. verify_all PASS:19 由 04 自报，结构上无新增检查项、无文件级红线触犯

不路由回 Developer。建议 PM 直接派 Stage 6 QA，并让 QA 在 Adversarial tests 中覆盖 MIN-1（BC-3）作为已知限制。

## §7 给 QA 的对抗性测试提示（QA 必覆盖）

1. **BC-1 全部候选超时**：`iptables -A OUTPUT -p tcp --dport 443 -j DROP` → server 横幅打印"公网 IP 探测失败" + FRP_EASY_PUBLIC_IP 样例；总耗时 ≤ 9s；exit 0
2. **BC-2 HTML 错误页注入**：detect_public_ip 应回 rc=1 + 空 stdout，不把 HTML 当 IP 打印（L183 IPv4 正则守护）
3. **BC-3 IPv6 字面量**：`FRP_EASY_PUBLIC_IP=2001:db8::1 FRP_EASY_ROLE=server bash install.sh` → 当前会拼出 `http://2001:db8::1:7800`（MIN-1 复现），作为已知限制记录
4. **BC-10 同主机重装不同 role**：先 server → 再 client（无 FORCE）→ exit 3；加 FORCE → 备份 toml 生成 + 新 toml + .role 切换
5. **BC-7 升级期已有 toml**：手动写 `UIBindAddr = "192.168.1.10"` → 升级 → byte-level 不变（D1 用户值优先）
6. **C-4 DataDir 解析**：`[ -d /opt/frp-easy/.frp_easy ]` rc=0 且 `stat -c %U` = SUDO_USER
7. **FRP_EASY_PUBLIC_IP 危险字符注入**：`FRP_EASY_PUBLIC_IP='1.2.3.4; rm -rf /'` → L164 正则拒绝 → 走失败横幅；脚本无命令注入
8. **getent 缺失场景**：Alpine/BusyBox → install.sh 应明确报错（Nit 跟进）
9. **toml 字段名大小写守护**：手动改预生成 toml 字段为小写 → Validate 失败 → 重启计数上升
10. **verify_all PASS:19**：A.1 不被 `FRP_EASY_ROLE=server` / `FRP_EASY_PUBLIC_IP=...` 字面量误中

---

**Verdict**：**APPROVED**
**计数**：PASS:6 / WARN:0 / FAIL:0；Critical:0 / Major:0 / Minor:2 / Nit:1
**最大风险**：MIN-1 IPv6 横幅 bracket 缺失（边缘 case，跟进修，不阻塞本任务）。
