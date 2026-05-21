# 06 测试报告 — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 6 产出。QA Tester 独立对抗性验证，不信任上游自述。
> 改动尚未 commit，依据工作区实际文件状态验证。
> 上游：01 需求（24 条 AC）、02 设计、03 Gate Review（3 条开发期条件 F-1/F-2/F-3）、04 开发、05 代码评审（APPROVED，2 MINOR 已修）。

## 测试结论

**verdict：PASS**

24 条验收标准全部通过独立验证。verify_all 在 PowerShell 与 Git Bash 双 shell 下均 19 项 PASS / 0 FAIL / 0 WARN。网络默认值变更（端口 8080→7800、绑定 127.0.0.1→0.0.0.0）经实跑二进制核实，NF-2 兼容性（老用户显式配置不被静默改写）经独立对抗性场景证伪未果——实现存活。Gate Review 3 条开发期条件全部落实。FRP 业务代理端口白名单 5 文件未误改。无 BLOCKER / CRITICAL / MAJOR 缺陷。

## 测试计划

| 验收标准 | 测试手段 | 验证文件 / 重现器 |
|---|---|---|
| AC-1 README 十章节 | grep 章节标题 | `README.md` |
| AC-2 README 无 8080 / URL 127.0.0.1:7800 | grep | `README.md` |
| AC-3 功能亮点覆盖 T-006+ 能力 | grep E2E/部署套件/浏览器/日志轮转/CI | `README.md` |
| AC-4 默认端口表四行 | grep 7800/7400/7500/7000 | `README.md` |
| AC-4b 许可证章节 + 不建 LICENSE | grep + ls | `README.md`、仓库根 |
| AC-5 status 头部更新日期 | grep | `docs/project-status.html` |
| AC-6 测试基线 167/57/224 | grep + 比对 baseline.json | `docs/project-status.html` |
| AC-7 §2 补 T-005~T-010、§3 补两模块 | grep | `docs/project-status.html` |
| AC-8 verify_all 19 项 | grep + 实跑 | `docs/project-status.html` |
| AC-9 status HTML 无外链 | grep `<link href` / `<script src` | `docs/project-status.html` |
| AC-10 全仓库无 8080 作 UI 端口 | 全仓库 grep 排除 _archived | 全仓库 |
| AC-11 openapi servers url 7800 | grep | `openapi.yaml` |
| AC-12 过时点清单留档 | 文本检查 | `02_SOLUTION_DESIGN.md §7` |
| AC-12b architecture Go167/前端57/补三模块/日期 | grep | `docs/architecture.html` |
| AC-12c architecture HTML 无外链 | grep | `docs/architecture.html` |
| AC-13 appconf 单测 PASS / 默认值 7800 | `go test ./internal/appconf/...` | `internal/appconf/config_test.go` |
| AC-14 Default()/Load() 回填 7800 / build OK | Read 代码 + `go build ./...` | `internal/appconf/config.go` |
| AC-15 FRP 代理端口白名单仍 8080 | grep 5 文件 | 白名单 5 文件 |
| AC-16 vite/playwright/e2e 脚本 7800 / E2E PASS | grep + verify_all C.1 | `web/*.config.ts`、`scripts/start-e2e-server.*` |
| AC-17 默认 UIBindAddr 0.0.0.0 | 单测 + 实跑生成 toml | `config_test.go`、实跑二进制 |
| AC-18 首启 stderr 安全提示三要素 / 回环不打 | 实跑二进制（场景 A/B/E/F） | `cmd/frp-easy/main.go` |
| AC-19 浏览器 URL 改写为 127.0.0.1:7800 | Read 代码 + 场景 A 监听地址核实 | `cmd/frp-easy/main.go:267-271` |
| AC-20 显式 127.0.0.1 配置不被改写 | 新增单测 + 独立实跑（场景 B） | `TestLoad_ExplicitLoopbackNotOverwritten` |
| AC-21 双 shell verify_all PASS≥19 / 测试≥223 | 实跑 ps1 + sh | `scripts/verify_all.{ps1,sh}` |

## 边界测试

本任务为文档刷新 + 网络默认值变更，未新增功能模块；QA 在 Developer 新增的 `TestLoad_ExplicitLoopbackNotOverwritten` 之外，以独立实跑二进制覆盖以下边界（详见对抗性测试段）：

- 缺 `UIBindAddr` 字段的旧配置 → 补默认为 `0.0.0.0`（场景 C）。
- `UIBindAddr = "::"`（IPv6 unspecified）→ 触发安全提示（场景 E）。
- `UIBindAddr = "localhost"`（回环主机名）→ 不触发安全提示（场景 F）。
- 端口被占用（双实例同端口）→ 友好提示 + 退出码 2（场景 D）。
- appconf 测试连跑 10 次（稳定性，无 flake）。

## Adversarial tests（对抗性测试 — 按验收标准给出证伪用例）

每条 AC 先写下"我预期它会失败，因为……"的证伪假设，再实跑。verdict 依据**实现是否在该证伪测试下存活**，而非 Developer 自述测试是否通过。重现器为 QA 独立编写，从 AC 出发，不复用 04_DEVELOPMENT.md 的测试代码。

| AC | 证伪假设（"我预期失败，当……"） | 重现器（QA 独立编写） | 结果（含工具输出） |
|---|---|---|---|
| AC-2/AC-10 | README 或文档残留 8080 当 UI 端口 | 全仓库 `grep -rn 8080` 排除 _archived + node_modules | **存活** — 命中仅 FR-4.4 白名单 5 文件（FRP 代理端口）、AC-20 新测试夹具、project-status.html §7 变更记录表（描述"8080→7800"本身）。无 UI 端口遗漏 |
| AC-17/AC-19 | 首启生成的 toml 不是 0.0.0.0/7800，或监听地址非预期 | 全新空目录实跑二进制 `timeout 4 frp-easy`，读生成的 frp_easy.toml | **存活** — 生成 `UIBindAddr='0.0.0.0'` / `UIPort=7800`；stderr `frp_easy UI 已启动：http://0.0.0.0:7800` |
| AC-18 | 0.0.0.0 默认下安全提示缺三要素之一 | 同上场景 A，grep stderr 三要素 | **存活** — 三要素齐全：① "局域网/公网内的设备均可访问"② "尽快用浏览器打开 UI 完成 setup 向导……完成 setup 前界面无密码保护"③ "将 UIBindAddr 改为 \"127.0.0.1\" 后重启" |
| AC-20 | 老用户显式写 `UIBindAddr="127.0.0.1"` + `UIPort=8080` 的旧配置被 Load() 静默改写为新默认值 | 独立写该 toml → 实跑二进制 → 读回 toml + 看 stderr 监听地址（场景 B，非复用 Developer 单测） | **存活** — 启动后 toml 仍为 `127.0.0.1`/`8080`，监听 `http://127.0.0.1:8080`，未被改写。NF-2 兼容性成立 |
| AC-18(b) | 回环绑定时仍打安全提示 | 场景 B（127.0.0.1）+ 场景 F（localhost）grep 安全提示关键词 | **存活** — 两场景 stderr 命中数均为 0，回环绑定不打提示 |
| 边界 | `UIBindAddr` 缺字段时回填值错误 | 写无 UIBindAddr 行的 toml（仅 UIPort=5599）实跑（场景 C） | **存活** — 补默认 `0.0.0.0`，安全提示端口正确为用户填的 5599（非写死） |
| 边界 | `UIBindAddr="::"` 不触发安全提示（漏 IPv6 unspecified） | 写 `UIBindAddr="::"` 实跑（场景 E） | **存活** — 触发安全提示。注：提示首行文案硬编码显示 `0.0.0.0:` 串（Code Review N-2 已记录 NIT，三要素不受损，不影响 AC-18） |
| AC-13/AC-21 | 端口占用提示建议值写死或退出码不对 | 双实例抢同端口 7800（场景 D） | **存活** — 第二实例 stderr "端口 7800 已被占用……UIPort = 7801"（相对逻辑），退出码 2 |
| AC-15 | 全仓库替换 8080 时误改了 FRP 代理端口夹具 | grep 白名单 5 文件 | **存活** — `storage_test.go:397 LocalPort:8080`、`qa_t007_adversarial_test.go:47 rp:=8080`、`qa_ac_test.go:480 rp:=8080+idx`、`qa_t007_adversarial.spec.ts:13/82`、`ProxyForm.spec.ts:55/61/66` 全部仍为 8080，一个未动 |
| AC-12b/M-1 | architecture.html API 路由表漏路由（Code Review M-1） | 数路由表 `<tr>` 行数 vs `router.go` 实际 `r.Get/Post/Put/Delete` 注册数 | **存活** — 路由表 28 行，与 `router.go` 实际 28 条 API 路由（行 72 `/health` + 行 84-123 共 27 条）精确吻合，含 M-1 列出的 6 条补全路由。无缺漏 |
| AC-6/F-3 | baseline.json 测试数与实跑不符 | `grep -rn "^func Test"` 计 Go 测试函数 vs baseline.json | **存活** — 实测 Go 测试函数 167，与 `baseline.json go_tests:167` 精确吻合；`test_count:224 = 167+57` |
| AC-9/AC-12c | HTML 引入外链导致离线打不开 | grep `<link href` / `<script src=` | **存活** — 两个 HTML 均 0 命中外链，纯内联 |
| AC-3 | README 功能亮点停留在 T-002 | grep T-006+ 能力关键词 | **存活** — E2E(T-006)/部署套件(T-008)/浏览器自动打开(T-010)/日志轮转(T-010)/CI(T-010) 各 1 处可定位（README:35/36/41/43/45） |
| 稳定性 | AC-20 新测试 flaky | `go test -count=10 -run TestLoad ./internal/appconf/...` | **存活** — 10 次连跑全 ok，无 flake |

### 对抗性测试关键工具输出存证

场景 A（首启无配置，全新目录，`timeout 4 frp-easy`）：
```
退出码 124（timeout 正常）
生成的 frp_easy.toml:
  UIBindAddr = '0.0.0.0'
  UIPort = 7800
stderr:
  提示：frp_easy UI 当前监听 0.0.0.0:7800，局域网/公网内的设备均可访问本管理界面。
    · 请尽快用浏览器打开 UI 完成 setup 向导，创建管理员账号（完成 setup 前界面无密码保护）。
    · frp_easy 已内置认证加固：argon2id 密码哈希、会话 Cookie、CSRF 防护、登录失败限流。
    · 如仅需本机访问，可编辑 frp_easy.toml，将 UIBindAddr 改为 "127.0.0.1" 后重启。
  frp_easy UI 已启动：http://0.0.0.0:7800
```

场景 B（旧配置显式 `127.0.0.1`/`8080`，AC-20 NF-2 核心）：
```
启动前 toml: UIBindAddr="127.0.0.1" / UIPort=8080
启动后 toml: UIBindAddr="127.0.0.1" / UIPort=8080  （未被改写 ✓）
stderr: frp_easy UI 已启动：http://127.0.0.1:8080
安全提示关键词命中数: 0  （回环绑定不打提示 ✓）
```

场景 D（端口占用）：
```
第二实例退出码: 2
stderr: frp_easy UI 启动失败：端口 7800 已被占用。请关闭占用进程，
       或编辑 frp_easy.toml 中 UIPort = 7801 后重试。
```

## verify_all 结果

- 测试总数：223 → 224（Go +1：新增 `TestLoad_ExplicitLoopbackNotOverwritten`，166→167；前端 57 不变）
- PowerShell（`pwsh -File scripts/verify_all.ps1`）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- Git Bash（`bash scripts/verify_all.sh`）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- 新增测试：1（`TestLoad_ExplicitLoopbackNotOverwritten`，由 Developer 按设计 §2.2 落实，QA 已独立实跑二进制对抗验证 AC-20，非仅信任该单测）
- baseline 是否更新：是（Developer 已更新 version 4→5、test_count 223→224、go_tests 166→167、passing_count 218→219、updated 2026-05-21）。QA 核对：实跑 `go test ./...` 全 PASS，`grep "^func Test"` 计数 167 与 baseline `go_tests` 精确一致——F-3 人工核对通过。QA 不对 baseline 再做改动（数字已正确）。

## 缺陷

- 无 BLOCKER。
- 无 CRITICAL。
- 无 MAJOR。
- 无 MINOR。

说明：Code Review N-2 记录的 NIT —— `exposureNotice` 文案在 `UIBindAddr="::"` 罕见路径下首行硬编码显示 `0.0.0.0:` 字符串 —— QA 实跑场景 E 复现确认：提示三要素仍齐全、触发逻辑正确，仅首行字面量在 `::` 时不够精确（应为 `[::]:`）。属 NIT，不构成验收缺陷，不阻塞交付。Code Review 已将其标为可选微调，QA 不另行升级。

## Gate Review 3 条开发期条件核查

- **F-1**：`scripts/start-e2e-server.{sh,ps1}` 的 `UIBindAddr = "127.0.0.1"` 行（sh:52 / ps1:58）本已存在，Developer 仅改 `UIPort` 行数字为 7800，未补重复 UIBindAddr 键。核查通过 ✓
- **F-2**：verify_all A.3（TODO/FIXME 预算）双 shell 均 PASS；本任务未引入 TODO/FIXME；pass_count=19（非降级到 18）。核查通过 ✓
- **F-3**：新增 AC-20 测试后 baseline.json 已同步（go_tests 166→167、test_count 223→224）；QA 实跑 `go test ./...` 与 `grep "^func Test"` 计数 167，与 baseline 数字精确吻合。核查通过 ✓

## 稳定性

- verify_all 在 PowerShell 与 Git Bash 双 shell 下各跑 1 次，结果一致（PASS 19 / FAIL 0），无跨 shell 漂移。
- `internal/appconf` 测试 `-count=10` 连跑 10 次，全 PASS，无 flake。
- 实跑二进制对抗性场景 A~F 共 6 个，行为确定、可重现。

## verdict

**PASS**

24 条验收标准全部通过独立对抗性验证；verify_all 双 shell 19 项 PASS / 0 FAIL；Gate Review 3 条条件落实；NF-2 兼容性经独立实跑证伪未果，实现存活；FRP 代理端口白名单未误改；测试数 223→224（只升不降，红线 3 满足）。无 BLOCKER / CRITICAL / MAJOR / MINOR 缺陷。建议 PM 推进至 stage 7（交付）。
