# 02 — 解决方案设计：T-013 rolling-release-install

> Stage 2 of 7-stage `/harness` 流水线。模式：full。语言：中文。
> 任务 ID：**T-013** · Slug：`rolling-release-install`
> 上游：`01_REQUIREMENT_ANALYSIS.md`（Verdict = READY FOR DESIGN）+ `INPUT.md`（PM 5 项预决策）。
> 本文档为下游 Gate Review 与 Developer 的唯一设计输入。Developer 不得在本文档之外做设计决策。

---

## 1. 方案概述（架构层一句话）

在 `.github/workflows/release.yml` 同一构建 job 上叠加第二条触发路径：`push: branches: [main]` 时复用现有 `build.sh --all` + `package.sh` 产物，发布到一个**固定 tag `rolling` 的正式（非 prerelease）GitHub Release**；原 `push: tags: ['v*']` 路径行为完全不变（仍产版本化 Release）。`install.sh` / `install.ps1` 把 GitHub API 查询端点从 `releases/latest` 改为确定性的 `releases/tags/rolling`，从而永远锁定滚动发布、不受未来 `v*` 版本化发布的 `latest` 语义漂移干扰。两条触发路径在同一 workflow 内通过 step 级条件（`github.ref_type` / `github.ref` 判定）分流 release 步骤，构建/打包步骤共用。

---

## 2. 受影响模块（现有文件路径）

| 文件 | 性质 | 改动概述 |
|---|---|---|
| `.github/workflows/release.yml` | 编辑 | 新增 `main` 分支触发；构建 job 加滚动发布分支步骤；顶部注释更新 |
| `scripts/install.sh` | 编辑 | API 端点 `releases/latest` → `releases/tags/rolling`；顶部注释、404 文案、版本号打印调整 |
| `scripts/install.ps1` | 编辑 | 同上（Windows 侧对等改动） |
| `docs/DEPLOYMENT.md` | 编辑 | A.1 手动下载地址、A.0 一键安装说明更新为滚动发布语义 |
| `README.md` | 编辑 | L45 CI 说明改为同时描述两条发布路径 |

**不改动**（复用既有契约）：`scripts/build.sh`、`scripts/package.sh`、`scripts/install-service.{sh,ps1}`、`scripts/uninstall-service.{sh,ps1}`、`scripts/verify_all.{ps1,sh}`。

---

## 3. 开放问题裁决（OQ-1 / OQ-2 / OQ-3）

### 3.1 OQ-1 裁决 — 滚动发布标记与安装脚本查询端点

**裁决：采用候选 (b)。**

- 滚动发布标记为**正式 release（非 prerelease、非 draft）**。
- `install.sh` / `install.ps1` 的 API 查询端点改为 `https://api.github.com/repos/Alan-IFT/frp_easy/releases/tags/rolling`。

**理由：**

1. **确定性（NFR-3 / BC-6 硬约束）。** `releases/tags/<tag>` 直接按 tag 名定位，结果与 release 列表排序、`latest` 语义、是否 prerelease 全部解耦。无论将来是否打 `v*` tag，安装脚本永远取到滚动发布这一个对象，不会漂移。这是 INPUT.md 决策 2 明确倾向的方向。
2. **与 prerelease 决策自洽（BC-5 / BC-10 硬约束）。** `releases/tags/<tag>` 端点对 prerelease 和正式 release 一视同仁均返回 200，不存在"prerelease 被 `latest` 跳过"问题，所以 BC-5 描述的"取不到滚动发布"场景在本方案下不可能发生。
3. **为什么仍选正式 release 而非 prerelease（候选 c）。** 候选 (c) 把滚动发布标 prerelease，配合 `releases/tags/` 端点也能工作。但本项目当前阶段唯一的安装来源就是滚动发布——把它标成 prerelease 会让 GitHub Releases 页面不显示"Latest"徽章、且 prerelease 在仓库主页/Releases 列表中视觉降级，对"滚动发布即当前推荐安装包"的产品定位不利。候选 (b)（正式 release + 查固定 tag）兼得"确定性"与"Releases 页面正常呈现为推荐版本"。
4. **与未来 `v*` 版本化发布的共存（BC-10）。** 因为安装脚本查的是固定 tag `rolling`、不查 `latest`，即使将来某个 `v*` 版本化 Release 成为 `releases/latest`，安装脚本完全不受影响。`rolling` 与 `v*` 互不靠 `latest` 语义、互不干扰。两者唯一的间接交互是 GitHub Releases 页面"Latest"徽章会落在最新的那个正式 release 上——这是纯展示层、不影响任何脚本逻辑，可接受。

**与 INPUT.md 的一致性：** 本裁决与 INPUT.md 决策 2 的倾向（"查 `releases/tags/<固定标签>`，确定性更强"）一致，无需提请 PM。

> **GR / Developer 注意 — 与 T-012 的 AC-8 关系：** T-012 的 02_SOLUTION_DESIGN.md 曾要求 API_URL 是精确字面量 `.../releases/latest`。该约束属于 T-012 已归档任务，T-013 的 AC-9 明确"端点已调整为滚动发布方案选定的端点"，本任务即合法地取代了那个旧字面量。Developer 改 URL 不构成对归档文档的违反。

### 3.2 OQ-2 裁决 — 固定标签名与 Release 标题

**裁决：采用候选 (a) 的标签 + 收敛的标题。**

- 固定标签名：**`rolling`**
- Release 标题（`name` 字段）：**`最新构建（rolling）`**

**理由：**

1. **`rolling` 语义最准确。** 本方案是"每次 push 即刷新"而非"每日定时"，候选 (c) `nightly` 语义不符（RA 也已指出）。`rolling`（滚动发布）精确描述"标签随 main HEAD 滚动前移"的行为。
2. **不选 `latest`（候选 b）的理由。** tag 名 `latest` 与 GitHub API 的 `releases/latest` 端点同名，极易在阅读 workflow / 脚本 / 文档时混淆"tag 名 latest"与"latest 语义端点"。`rolling` 无歧义。
3. **命名空间不冲突。** 版本化 tag 形如 `v0.1.0`，`rolling` 不匹配 `v*` glob，两个 tag 命名空间完全分离。`release.yml` 的 `push.tags: ['v*']` 触发器不会被 `rolling` tag 的创建/移动触发（见 §5.4 风险 R-5）。

### 3.3 OQ-3 裁决 — 并发 push 竞态

**裁决：采用候选 (b) — 加 `concurrency` 组。**

在 `release.yml` 的 job 级（或 workflow 级）加：

```yaml
concurrency:
  group: rolling-release-${{ github.ref }}
  cancel-in-progress: true
```

**理由与设计细节：**

1. **`group` 必须含 `${{ github.ref }}`。** 若 group 用固定字面量（如 `group: rolling-release`），则 `v*` tag 触发的版本化发布与 `main` 触发的滚动发布会落进同一并发组、互相取消——这会破坏 BC-12（既有 `v*` 流程不受损）。用 `${{ github.ref }}` 后：
   - 多次 push `main` → ref 均为 `refs/heads/main` → 同组 → 新 run 取消旧 run。
   - push `v0.1.0` → ref 为 `refs/tags/v0.1.0` → 独立组 → 与滚动发布、与其他 tag 互不取消。
2. **`cancel-in-progress: true` 的效果。** 短时间连续 push `main` 时，后到的 run 取消正在跑的旧 run；最终只有最新 commit 的产物被发布。这比候选 (a)（裸最终一致性）更强：不仅最终一致，还避免"旧 run 在新 run 之后才完成、把过时资产覆盖回去"的逆序竞态。
3. **被取消的 run 的安全性。** GitHub Actions 取消 run 时，若 run 尚未走到 `softprops/action-gh-release` 步骤，则不产生任何发布副作用；若恰好在发布步骤中途被取消，最坏情况是资产上传不完整——但紧随其后的新 run 会重新完整刷新（见 §4.3 资产替换逻辑会先清后传）。满足 BC-9"不得是两次构建混合的损坏状态"——因为每个 run 的发布步骤是"该 run 自己构建的整套资产"，不存在跨 run 混合。

---

## 4. `release.yml` 完整改造设计（YAML 结构级）

### 4.1 触发器与并发

```yaml
on:
  push:
    branches:
      - main          # 新增：滚动发布路径
    tags:
      - 'v*'          # 保留：版本化发布路径

permissions:
  contents: write     # 不变：创建 release / 移动 tag / 上传资产均需

concurrency:
  group: rolling-release-${{ github.ref }}
  cancel-in-progress: true
```

> 说明：单个 `on.push` 块下同时写 `branches` 与 `tags` 是合法语法；GitHub 对二者取并集（push 到 main 或 push 任意 v* tag 均触发）。普通 `push` 到其他分支、PR 均不触发——满足 Out-of-scope"不改 PR / 非 main 分支 CI 行为"。

### 4.2 构建/打包步骤（共用，不分流）

`Checkout` → `Setup Go` → `Setup Node` → `Build` → `Package` 五个 step 对两条路径完全一致，**保持现状不动**：

- `actions/checkout@v4`
- `actions/setup-go@v5`，`go-version: '1.25'`（与 `go.mod` 对齐，insight 已确认，勿改）
- `actions/setup-node@v4`，`node-version: '20'`
- `run: bash scripts/build.sh --all`
- `run: bash scripts/package.sh --windows --skip-build`

> **`Checkout` 的 fetch-depth 注意（关键）：** `build.sh` L19 用 `git describe --tags --always --dirty` 生成版本号。`actions/checkout@v4` 默认 `fetch-depth: 1`（浅克隆，不拉 tag），`git describe --tags` 在无 tag 可见时回退到 `--always`（输出短 commit hash）。本设计**接受该行为**（见 §6 版本字符串说明）。若希望滚动发布版本号包含"距最近 tag 的提交数"，需显式设 `fetch-depth: 0`——但这是可选优化，**本期不做**，保持 `checkout@v4` 默认值，理由见 §6。

### 4.3 Release 步骤分流（核心改造）

把现有单一的 `Create GitHub Release & upload assets` step 替换为**两个互斥的条件 step** + 一个 tag 移动 step。

**Step A — 计算滚动发布版本展示名（仅 main 路径）：**

```yaml
- name: Compute rolling release name
  id: rollver
  if: github.ref == 'refs/heads/main'
  run: echo "shortsha=${GITHUB_SHA::7}" >> "$GITHUB_OUTPUT"
```

**Step B — 计算版本化发布名（仅 v* tag 路径，等价于现有逻辑）：**

```yaml
- name: Compute release name from tag
  id: ver
  if: startsWith(github.ref, 'refs/tags/v')
  run: echo "version=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"
```

**Step C — 发布滚动 release（仅 main 路径）：**

```yaml
- name: Publish rolling release
  if: github.ref == 'refs/heads/main'
  uses: softprops/action-gh-release@v2
  with:
    tag_name: rolling
    name: 最新构建（rolling）
    body: |
      与 main 分支同步的滚动发布（自动刷新）。
      本次构建 commit：${{ github.sha }}
      install.sh / install.ps1 一键安装下载的即为本发布的资产。
    prerelease: false
    make_latest: false
    files: |
      bin/release/*.tar.gz
      bin/release/*.zip
    fail_on_unmatched_files: true
```

**Step D — 发布版本化 release（仅 v* tag 路径，等价于现有逻辑）：**

```yaml
- name: Create GitHub Release & upload assets
  if: startsWith(github.ref, 'refs/tags/v')
  uses: softprops/action-gh-release@v2
  with:
    name: ${{ steps.ver.outputs.version }}
    files: |
      bin/release/*.tar.gz
      bin/release/*.zip
    generate_release_notes: true
    fail_on_unmatched_files: true
```

### 4.4 `softprops/action-gh-release@v2` 能力核对（BC-7 / BC-8）

设计依据该 action v2 文档化的既有行为，无需额外的"先删旧 tag/release"step：

| 需求 | action 行为 | 满足的 BC |
|---|---|---|
| tag 不存在 → 创建 | 指定 `tag_name: rolling`，action 自动在当前 `GITHUB_SHA` 上创建该 tag 并建 release | — |
| tag 已存在但指向旧 commit → 移动到新 HEAD | action v2 检测到 release 已存在时执行更新；并会把 tag ref 更新指向当前 workflow 的 commit（action 内部对已存在 tag 调用 `git`/API 更新 ref） | **BC-7** |
| release 已存在 → 复用而非报错 | action 默认对已存在 release 做更新（不是创建新 release），不会因"标签已存在"失败 | **BC-7** |
| 旧资产清理 | **关键：必须显式设 `clean_release_attachments`** | **BC-8** |

> **BC-8 必须显式处理 — Developer 强约束。** `softprops/action-gh-release@v2` 默认行为是把 `files` 列出的资产**追加/覆盖同名**到已存在 release，**不会删除 release 上已有但本次 files 未列出的旧资产**。由于资产文件名含版本号（`frp-easy-<VERSION>-linux-amd64.tar.gz`，滚动发布场景 VERSION 是短 sha，每次 commit 不同），两次构建的资产**文件名不同名**，默认行为下旧资产会**残留**——违反 BC-8 / AC-4。
>
> **解决方案：** 在 Step C 加 `action-gh-release@v2` 的 `clean_release_attachments: true`（该选项的语义是"发布前清空该 release 上的全部已有附件，再上传本次 files"）。Developer 须在实现时**核对所用 `softprops/action-gh-release` 具体版本是否支持 `clean_release_attachments` 选项**：
> - 若支持 → 直接在 Step C 加 `clean_release_attachments: true`，BC-8 解决。
> - 若该选项在 pin 的版本不可用 → 退化方案：在 Step C **之前**加一个 step，用 `gh release delete-asset` 或 GitHub API 删除 `rolling` release 上所有现存 `*.tar.gz` / `*.zip` 资产（用 `gh release view rolling --json assets` 枚举，对每个删除；release 不存在时忽略错误）。退化方案示例：
>   ```yaml
>   - name: Purge old rolling assets
>     if: github.ref == 'refs/heads/main'
>     env:
>       GH_TOKEN: ${{ github.token }}
>     run: |
>       # rolling release 不存在时 gh release view 失败，忽略即可（首次发布）
>       if gh release view rolling >/dev/null 2>&1; then
>         gh release view rolling --json assets -q '.assets[].name' \
>           | while read -r a; do gh release delete-asset rolling "$a" -y || true; done
>       fi
>   ```
> Developer 在 `04_DEVELOPMENT.md` 记录最终采用哪条路径（首选 `clean_release_attachments`）。

### 4.5 顶部注释改造（AC-14）

`release.yml` 顶部注释块替换为（中文，结构示意）：

```
# T-013 rolling-release-install · GitHub Actions release workflow
#
# 两条触发路径：
#   1) push 到 main 分支  → 滚动发布（rolling release）
#      - 复用 build.sh --all + package.sh 构建 Linux + Windows 包
#      - 创建/刷新固定 tag `rolling` 的 Release，替换其全部旧资产
#      - install.sh / install.ps1 一键安装下载的即为此发布
#   2) push v* tag（如 v0.1.0） → 版本化发布
#      - 行为与改造前一致，产出以去 v 前缀版本号命名的正式 Release
#      - 为将来正式发版保留，可选；不影响滚动发布
#
# 发版步骤（人工）：
#   常规情形：把代码推到 main 即自动刷新滚动发布，维护者无需任何手动发版动作。
#   正式版本化发布（可选）：
#     git tag v0.1.0 && git push origin v0.1.0
#
# 复用 scripts/build.sh + scripts/package.sh，不在 workflow 里复刻构建逻辑。
```

### 4.6 workflow 流程图

```
push 事件
  │
  ├─ ref = refs/heads/main ───┐
  │                            │
  ├─ ref = refs/tags/v* ──────┤
  │                            ▼
  │              concurrency: rolling-release-${ref}
  │              （main 多 run 互相取消；v* tag 各自独立）
  │                            │
  │                            ▼
  │         Checkout → Setup Go → Setup Node
  │              → build.sh --all → package.sh --windows --skip-build
  │              （两路径共用，产物 bin/release/*.tar.gz + *.zip）
  │                            │
  │             ┌──────────────┴──────────────┐
  │             ▼                              ▼
  │   if ref==refs/heads/main          if ref startsWith refs/tags/v
  │   ┌────────────────────┐           ┌────────────────────────┐
  │   │ (可选) 清旧资产      │           │ Compute version (#v)   │
  │   │ Publish rolling     │           │ Create versioned       │
  │   │  tag_name: rolling  │           │  Release (name=版本号) │
  │   │  prerelease: false  │           │  generate_release_notes│
  │   │  clean attachments  │           │  行为同改造前           │
  │   └────────────────────┘           └────────────────────────┘
  │             │                              │
  └─────────────┴──────────────────────────────┘
                          ▼
              构建失败 → run 失败、不发布残缺资产（BC-11，沿用 step 次序）
```

---

## 5. `install.sh` / `install.ps1` 改动清单

> 两脚本端点必须一致（AC-9）。除 API 端点 URL、与之相关的注释/文案外，下载/校验/解压/服务注册/升级语义**全部不动**（AC-11）。

### 5.1 `scripts/install.sh`

| 位置 | 现状 | 改为 |
|---|---|---|
| L25 `API_URL` | `https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest` | `https://api.github.com/repos/Alan-IFT/frp_easy/releases/tags/rolling` |
| L1–L21 顶部注释 | "调用 GitHub Releases API 取最新 release" | 改为"调用 GitHub Releases API 取固定标签 `rolling` 的滚动发布" |
| L147 404 文案 | "仓库尚未发布任何 release，无法一键安装；请改用源码构建（docs/DEPLOYMENT.md 路径 B）或等待首个 release。" | 改为滚动发布语义，例："滚动发布尚未生成（维护者尚未首次 push main），暂无法一键安装；请改用源码构建（docs/DEPLOYMENT.md 路径 B）或等待维护者首次 push。"（满足 BC-3 文案语义） |
| L182 版本号打印 | `echo "    最新版本：${VERSION}"` | 改为 `echo "    滚动发布版本：${VERSION}"`（VERSION 仍来自 `tag_name`，对 rolling release 即字面量 `rolling`；仅展示用，不参与逻辑） |
| L174 / L177 缺资产文案 | "最新 release 未包含当前平台..." | "滚动发布未包含当前平台（${PLATFORM}）的发布包..."（BC-4 语义不变，措辞同步） |

**JSON 解析逻辑核对（关键）：** `releases/tags/<tag>` 与 `releases/latest` 返回的 **JSON 结构完全相同**（均为单个 release 对象，含 `tag_name`、`assets[]`、每个 asset 的 `browser_download_url`）。因此 install.sh 步骤 4 的 `grep -oE '"tag_name"...'` 与 `grep -oE '"browser_download_url"...'` 解析逻辑**无需改动、依然成立**。仅注意：rolling release 的 `tag_name` 值是字面量 `"rolling"`，VERSION 变量会取到 `rolling`——这只影响进度打印（已在上表第 4 行处理），不影响资产匹配（资产匹配靠 `browser_download_url` 的 `frp-easy-.*-${PLATFORM}\.tar\.gz$` 正则，与 tag_name 无关）。

**失败分流不变（AC-10）：** 步骤 3 的 `curl -sSL -w '%{http_code}'`（去 `-f`）+ HTTP 状态码 case 分流（200/403/404/其他）**整体保留**。改端点后 404 的含义从"仓库无任何 release"变为"`rolling` tag 对应的 release 不存在"——文案已按上表第 3 行更新，BC-1/BC-2/BC-3/BC-4 四类分支均保留对应中文 stderr + 非 0 退出。insight"先判 HTTP 状态码、后解析 JSON""查询步骤不能用 `curl -f`"两条约束**继续遵守，不动**。

### 5.2 `scripts/install.ps1`

| 位置 | 现状 | 改为 |
|---|---|---|
| L30 `$ApiUrl` | `.../releases/latest` | `.../releases/tags/rolling` |
| L1–L21 顶部注释 | "取最新 release" | "取固定标签 `rolling` 的滚动发布" |
| L116 404 文案 | "仓库尚未发布任何 release..." | 同 install.sh：滚动发布尚未生成语义（BC-3） |
| L142 版本号打印 | `Write-Host "    最新版本：$version"` | `Write-Host "    滚动发布版本：$version"` |
| L136 缺资产文案 | "最新 release 未包含 Windows 平台..." | "滚动发布未包含 Windows 平台（$platform）的发布包。" |

**解析逻辑核对：** install.ps1 用 `ConvertFrom-Json` 后取 `$json.tag_name` 与 `$json.assets`。`releases/tags/<tag>` 返回结构与 `releases/latest` 一致，`ConvertFrom-Json` 解析无需改动。`Invoke-WebRequest` 的 catch 分流（无 Response → 网络层；403 / 404 / 其他）整体保留。

> **AC-9 注意：** 改后两脚本端点字符串必须**逐字一致**（除 `$`/无 `$` 的语言差异外，URL 主体相同），且与本文档 §3.1 裁决一致。建议 URL 仍写死整串字面量（便于 `grep -F` 走查），不要用变量拼 repo 名。

---

## 6. 滚动发布二进制的版本字符串行为说明

`build.sh` L19：`VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")`，注入 `-X main.Version=${VERSION}`。`package.sh` L69 同逻辑，决定发布包文件名 `frp-easy-<VERSION>-<os>-amd64.<ext>`。

**滚动发布场景下 `git describe` 的输出取决于 `actions/checkout@v4` 的 fetch 行为：**

| checkout 配置 | 仓库是否已有 `v*` tag | `git describe --tags --always` 输出 | 资产文件名示例 |
|---|---|---|---|
| 默认 `fetch-depth: 1`（本期采用） | — （浅克隆不拉 tag history） | 短 commit hash，如 `a1b2c3d` | `frp-easy-a1b2c3d-linux-amd64.tar.gz` |
| `fetch-depth: 0` | 无 tag | 短 commit hash，如 `a1b2c3d` | 同上 |
| `fetch-depth: 0` | 有 tag `v0.1.0`，HEAD 在其后 3 个提交 | `v0.1.0-3-ga1b2c3d` | `frp-easy-v0.1.0-3-ga1b2c3d-linux-amd64.tar.gz` |

**本期裁决：保持 `actions/checkout@v4` 默认 `fetch-depth: 1`，滚动发布二进制的版本字符串为 7 位短 commit hash（如 `a1b2c3d`）。**

**理由与可接受性：**

1. **可接受。** 短 commit hash 唯一标识构建来源的 commit，对"滚动发布=与 main 同步"的定位完全够用——用户/维护者能据此 hash 在 git 历史定位确切代码。`frp-easy --version` 会显示该 hash。
2. **不破坏资产命名约定。** RA §9 末尾要求"资产命名 `frp-easy-<VERSION>-<os>-amd64.{tar.gz,zip}`，安装脚本资产匹配正则不变"。install 脚本资产正则是 `frp-easy-.*-${PLATFORM}\.tar\.gz$`，`.*` 通配 VERSION 段，短 hash（`a1b2c3d`）落在 `.*` 内，**正则继续匹配，无需改**。
3. **不需要 `fetch-depth: 0`。** 拉全 tag 历史只为让版本号更"好看"（带 `v0.1.0-3-g` 前缀），但当前仓库**尚无任何 `v*` tag**，即便设 `fetch-depth: 0` 输出也仍是裸 hash。为一个当前无收益、且增加 checkout 时间的配置改动不值得——保持默认。
4. **`--dirty` 后缀。** CI 全新 checkout 工作区干净，不会触发 `-dirty`。无需担心。

> **Developer 注意：** 不要为"版本号更漂亮"擅自给 checkout 加 `fetch-depth: 0`——那属于本设计未授权的改动。若 GR/未来认为需要，单独提任务。

---

## 7. Reuse audit（复用审计）

| 需求 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| Linux + Windows 交叉编译 | `build.sh --all` | `scripts/build.sh` | 原样复用，不改 |
| tar.gz + zip 打包、staging 组装、资产命名 | `package.sh --windows --skip-build` | `scripts/package.sh` | 原样复用，不改 |
| 创建/上传 GitHub Release | `softprops/action-gh-release@v2` | `.github/workflows/release.yml` 现有 step | 复用同一 action，新增 `tag_name` + `clean_release_attachments` 参数派生滚动发布 step |
| `v*` tag 触发的版本化发布 | 现有 `on.push.tags` + Compute version + Create Release step | `.github/workflows/release.yml` | 原样保留，仅加 `if` 条件使其只在 tag 路径执行 |
| GitHub API 查询 + HTTP 状态码分流 + 无 jq JSON 解析 | install.sh 步骤 3/4、install.ps1 步骤 3/4 | `scripts/install.{sh,ps1}` | 复用整套分流/解析逻辑，仅换端点 URL |
| systemd / Windows 服务注册 | `install-service.{sh,ps1}` | `scripts/install-service.*` | 原样复用，install.* 仍按既有契约调用 |
| Go 版本对齐 | `setup-go@v5` `go-version: '1.25'` | `.github/workflows/release.yml` | 复用，不改（insight 已确认对齐 go.mod） |
| 滚动发布的固定 tag 强制移动 | （`softprops/action-gh-release@v2` 内置） | — | 复用 action 对已存在 tag 的 ref 更新能力（§4.4），无需自写 git tag -f |

**新依赖：无。** 不引入任何新 action、新库、新服务。`concurrency` 是 GitHub Actions 内建语法，非依赖。

---

## 8. 风险分析

| 编号 | 风险 | 影响 | 缓解 |
|---|---|---|---|
| **R-1** | `softprops/action-gh-release@v2` 默认**不删** release 上的旧资产；滚动发布资产文件名含短 hash、每次不同名 → 旧资产残留 | 违反 BC-8 / AC-4，滚动发布混入过时资产 | §4.4 强约束：Step C 显式加 `clean_release_attachments: true`；若 pin 版本不支持则用 `gh release delete-asset` 退化方案先清旧资产。Developer 必须二选一并在 04_DEVELOPMENT.md 记录。 |
| **R-2** | `concurrency.group` 若误用固定字面量，会让 `v*` tag 发布与滚动发布互相取消 | 破坏 BC-12（既有版本化发布能力受损） | §3.3 强约束：`group` 必须含 `${{ github.ref }}`，使 main 与各 tag 落入不同并发组。 |
| **R-3** | `softprops/action-gh-release@v2` 对"已存在 tag 指向旧 commit"的 ref 更新行为，若该 action 版本实际不更新 tag ref，则滚动发布 tag 永远停在首次 commit | BC-7 不满足，滚动发布资产虽刷新但 tag 指向陈旧 commit | Developer 在实现时核对所 pin 的 `action-gh-release` 版本对 `tag_name` 已存在场景的文档化行为；若该版本不移动 tag，在 Step C 前加显式 step：`git tag -f rolling $GITHUB_SHA && git push origin -f refs/tags/rolling`（`permissions: contents: write` 已足够）。AC-16 交付后人工验证会暴露此问题。 |
| **R-4** | 每次 `main` push 跑约 2–3 分钟构建，CI 额度消耗上升 | 额度成本 | NFR-4 已接受：公共仓库免费额度内；仅 main 触发、不在 PR/其他分支触发；`cancel-in-progress` 进一步减少无谓的并发 run。 |
| **R-5** | `softprops/action-gh-release@v2` 创建/移动 `rolling` tag 这个动作，是否会反过来触发 workflow 自身的 `push.tags` 触发器形成循环 | 自触发循环、额度浪费 | `push.tags` glob 是 `v*`，`rolling` tag 不匹配 `v*`，**不会**触发版本化路径。且 GitHub Actions 默认用 `GITHUB_TOKEN` 推的 ref 不会再触发 workflow（防循环内建机制）。双重保险，无循环风险。 |
| **R-6** | 本任务不真实 push main / 不触发真实 CI，AC-16 / AC-17 无法在流水线内验证 | 集成正确性需交付后才知 | RA §8.1 / PM 决策 5 已定：AC-16/AC-17 标"交付后人工验证、非完成闸门"；完成闸门为静态 AC-1…AC-15。PM 在 07_DELIVERY.md 写明实测前提。 |
| **R-7** | `release.yml` step 加 `if:` 条件后，YAML 缩进/表达式语法出错导致 workflow 不合法 | AC-1 FAIL | Developer 实现后用 `actionlint`（若 verify_all 含）或 YAML 解析校验；AC-1 即此校验。本文档 §4 已给出结构级 YAML，Developer 照搬缩进。 |

---

## 9. 迁移 / 上线计划

1. **向后兼容性。** `v*` tag 版本化发布路径行为 100% 保留（Step D `if: startsWith(github.ref,'refs/tags/v')` 等价于原无条件 step）。既有维护者若打 `v*` tag，结果与改造前一致（BC-12 / AC-5）。
2. **install 脚本兼容。** 端点改动对终端用户无感——用户命令行不变（`curl ... | sudo bash` / `irm ... | iex`），脚本内部换端点。改后在"滚动发布尚未生成"时给 BC-3 友好中文报错而非 T-012 旧的"仓库无 release"文案。
3. **数据迁移：无。** 不涉及 SQLite schema、不涉及任何持久化数据。
4. **feature flag：无。** workflow 改动一旦合并即生效。
5. **首次滚动发布的产生（交付后，非本任务）。** 本任务交付后，由用户 push 一次 `main`（PM 决策 5），workflow 首次运行创建 `rolling` tag 与 Release。在此之前 install 脚本查 `releases/tags/rolling` 会得 404 → BC-3 友好报错。这是预期过渡态，不是缺陷。
6. **回滚。** 若改造后 workflow 异常，回滚 = `git revert` 本任务 commit；`release.yml` 恢复为仅 `v*` tag 触发；install 脚本端点恢复 `releases/latest`。无残留副作用（顶多 GitHub 上多一个 `rolling` tag/release，可在 UI 手动删）。

---

## 10. Out-of-scope（设计边界）

- 不改 `build.sh` / `package.sh` 的构建/打包逻辑（含版本号生成逻辑）。
- 不给 `actions/checkout` 加 `fetch-depth: 0`（§6 已论证不需要）。
- 不改 `install-service.*` / `uninstall-service.*`。
- 不引入 GitHub token 认证提升 API 限流额度（install 脚本仍匿名请求）。
- 不做滚动发布的多份历史资产保留 / 版本回滚（滚动发布按定义只持最新一次 main 构建）。
- 不做 GPG / 数字签名校验（与 T-012 一致，仅校验非空 + 可解压）。
- 不在 PR / 非 main 分支触发发布。
- 不自动 push main / 不自动打 tag / 不触发真实发布（PM 决策 5）。
- 不把真实联网安装 / 真实 CI 接入 `verify_all`。
- 不做 ARM / RISC-V / macOS 专用包（发布产物仍 linux-amd64 + windows-amd64）。

---

## 11. 17 条 AC 覆盖映射

| AC | 描述要点 | 设计覆盖 | 验证方式 |
|---|---|---|---|
| AC-1 | `release.yml` 合法 YAML | §4 给出结构级 YAML，Developer 照缩进实现 | 静态/自动 |
| AC-2 | `on:` 同含 `push.branches`（main）与 `push.tags`（v*） | §4.1 | 静态/自动 |
| AC-3 | 存在"main push 创建/更新固定标签滚动发布"步骤，固定标签为字面量 | §4.3 Step C，`tag_name: rolling` 字面量 | 静态/自动 |
| AC-4 | "标签已存在更新"（BC-7）+"旧资产替换"（BC-8）逻辑可定位 | §4.4：action 更新 tag ref + `clean_release_attachments: true`（或退化清理 step） | 静态/自动 |
| AC-5 | `v*` 路径产版本化 Release 逻辑保留，行为同 T-010 | §4.3 Step B + Step D，加 `if` 后语义等价原 step | 静态/自动 |
| AC-6 | `bash -n scripts/install.sh` 通过 | 仅改 URL 字符串/注释/echo 文案，不改语法结构 | 静态/自动 |
| AC-7 | `install.sh` 过 `shellcheck` 无 error | 同上，无新语法引入 | 静态/自动 |
| AC-8 | `install.ps1` 可被 PowerShell 解析 | 仅改 `$ApiUrl` 字符串/注释/Write-Host 文案 | 静态/自动 |
| AC-9 | 两脚本端点已调整且一致、与本设计裁决一致 | §5.1 + §5.2：均改为 `.../releases/tags/rolling` | 静态/自动 |
| AC-10 | BC-1…BC-4 四类失败分支中文 stderr + 非 0 退出可定位 | §5.1/§5.2：保留步骤 3 状态码分流；404 文案更新为 BC-3 语义 | 静态/自动 |
| AC-11 | `set -euo pipefail` / `$ErrorActionPreference="Stop"` 保留，下载/校验/解压/服务/升级不破坏 | §5：明确除端点 URL + 相关注释/文案外全部不动 | 静态/自动 |
| AC-12 | `DEPLOYMENT.md` 发布/安装说明更新为滚动发布语义，手动下载地址为稳定 URL | §12 文档改动清单 | 静态/自动 |
| AC-13 | `README.md` 涉及发布说明同时描述两条路径 | §12 文档改动清单 | 静态/自动 |
| AC-14 | `release.yml` 顶部注释更新，述两路径 + 常规无需手动发版 | §4.5 注释块设计 | 静态/自动 |
| AC-15 | verify_all PASS 数 ≥ 基线 19，无新增 WARN/FAIL | 改动均为静态文件文本/YAML，无新测试破坏；Developer 跑 verify_all | 静态/自动 |
| AC-16 | push main 后 CI 真实产出滚动发布，资产含 linux+windows | §4 workflow 设计支撑；交付后人工验证 | 集成/人工（非闸门） |
| AC-17 | 滚动发布产生后真实联网一键安装跑通 | §5 install 脚本设计支撑；交付后人工验证 | 集成/人工（非闸门） |

**结论：AC-1…AC-15（15 条完成闸门 AC）全部有设计覆盖；AC-16/AC-17 为交付后人工验证、非闸门，设计已为其提供支撑。无遗漏。**

---

## 12. 文档改动清单（AC-12 / AC-13）

### 12.1 `docs/DEPLOYMENT.md`

| 位置 | 现状 | 改为 |
|---|---|---|
| A.1 L87 手动下载地址 | `<https://github.com/Alan-IFT/frp_easy/releases/latest>` | 改为滚动发布稳定 URL：`<https://github.com/Alan-IFT/frp_easy/releases/tag/rolling>`，并加一句"该地址始终指向与 main 同步的最新滚动发布构建" |
| A.0 一键安装段（L36–79 区域） | 未提"下载的是滚动发布" | 增补一句：明确"一键安装下载的是与 main 分支同步的滚动发布预编译包，维护者每次 push main 自动刷新" |
| B.6 打包说明（若提及发布前提） | Developer 走查 B 段，若有"需先 v* tag 发版才有产物"的隐含措辞 | 改为"main push 即自动产生滚动发布，无需手动 v* tag 发版" |

> Developer 实现时：B.6 的具体行号本设计未逐字定位，Developer 通读 DEPLOYMENT.md B 段，凡隐含"需手动发版才有产物"的措辞按滚动发布语义订正；§553 行的 frp 子二进制下载（T-002）与本任务无关，**不动**。

### 12.2 `README.md`

| 位置 | 现状 | 改为 |
|---|---|---|
| L45 | `- **GitHub Actions CI**（T-010）：push \`v*\` tag 自动构建并上传 GitHub Releases 资产。` | 改为同时描述两路径，例：`- **GitHub Actions CI**：push 到 \`main\` 自动构建并刷新滚动发布（rolling release，一键安装下载源）；push \`v*\` tag 产出版本化正式发布。` |

---

## 13. Partition assignment（分区分配）

> 项目存在 `.harness/agents/dev-*.md`（dev-db / dev-backend / dev-frontend）。但 PM 在 T-013 派发中明确**"单 Developer 分区"**。本任务所有改动文件归属如下：

| 文件 | 分区 | New / Edit | 依赖 |
|---|---|---|---|
| `.github/workflows/release.yml` | dev-backend（单 Developer 模式下统一承担） | edit | — |
| `scripts/install.sh` | dev-backend | edit | — |
| `scripts/install.ps1` | dev-backend | edit | — |
| `docs/DEPLOYMENT.md` | dev-backend | edit | — |
| `README.md` | dev-backend | edit | — |

### Dispatch order

单 Developer，顺序无强约束。建议顺序：
1. `.github/workflows/release.yml`（确立 `rolling` tag 名 → install 脚本端点须与之一致）
2. `scripts/install.sh` + `scripts/install.ps1`（端点须与 workflow 的 tag 名 `rolling` 对齐）
3. `docs/DEPLOYMENT.md` + `README.md`（文档措辞须与前两步实现一致）

### Parallelism

无需并行 —— 单 Developer、改动量小。各文件改动相互独立（唯一耦合是 `rolling` 这一字面量必须三处一致：workflow `tag_name`、install 脚本端点、文档下载地址）。

---

## 14. 给 Developer 的实现注意事项

1. **`rolling` 是贯穿全任务的关键字面量**，必须三处逐字一致：`release.yml` 的 `tag_name: rolling`、install 脚本端点 `.../releases/tags/rolling`、`DEPLOYMENT.md` 下载地址 `.../releases/tag/rolling`（注意 GitHub **网页** tag 地址是 `/releases/tag/<tag>` 单数，**API** 是 `/releases/tags/<tag>` 复数——别写错）。
2. **BC-8 必做（最易漏）：** Step C 必须加 `clean_release_attachments: true`；核对所 pin 的 `softprops/action-gh-release` 版本支持该选项，不支持则用 §4.4 的 `gh release delete-asset` 退化 step。在 `04_DEVELOPMENT.md` 记录采用哪条。
3. **`concurrency.group` 必须含 `${{ github.ref }}`**，不可写固定字面量（否则破坏 BC-12）。
4. **Step 的 `if` 条件**：main 路径用 `github.ref == 'refs/heads/main'`；v* 路径用 `startsWith(github.ref, 'refs/tags/v')`。两条件互斥。`Compute` 系列 step 也要带对应 `if`，否则在另一路径下 `GITHUB_REF_NAME#v` 会取到非预期值（无害但不整洁）。
5. **install 脚本只改 5 处**（§5.1 / §5.2 表格），其余一字不动。尤其**不要动**步骤 3 的 `curl -sSL -w '%{http_code}'`（保留去 `-f`）、状态码 case 分流、`grep/sed` JSON 解析——insight 明确这些是踩坑换来的正确模式。
6. **404 文案语义**：改后 404 含义是"`rolling` release 不存在"，文案要按 BC-3 写"滚动发布尚未生成"，不要再写 T-012 的"仓库尚未发布任何 release"。
7. **不要给 `actions/checkout` 加 `fetch-depth: 0`**（§6 已论证），不要改 `build.sh` / `package.sh`。
8. **不改 `.github/copilot-instructions.md`、`.harness/`、`.claude/`、`CLAUDE.md`、归档文档**（红线）。`release.yml` 不在红线清单，可改。
9. **完成前必须跑 `scripts/verify_all` 至 PASS ≥ 19**（AC-15 / 红线 6）。`bash -n install.sh`、`shellcheck`、`pwsh` 解析、`actionlint`（若 verify_all 含）须全过。
10. **若实现中发现需偏离本设计**（如 `action-gh-release` 行为与 §4.4 描述不符需换方案），在 `04_DEVELOPMENT.md` 标 `DESIGN DRIFT` 并说明，不要静默漂移（红线 1）。

---

## 15. Verdict

**READY** — 设计完整，OQ-1/OQ-2/OQ-3 已明确裁决且与 INPUT.md 决策一致；15 条完成闸门 AC（AC-1…AC-15）全部有设计覆盖；新依赖为零；风险均附缓解。可进入 Gate Review。

> 给 GR / PM 的说明：
> - 本设计未推翻 INPUT.md 任何一项 PM 预决策；OQ-1 裁决与 INPUT.md 决策 2 倾向一致，无提请事项。
> - 唯一需 Developer 在实现期落地核实的是 §4.4 的 `softprops/action-gh-release` 版本对 `clean_release_attachments` 与 tag ref 更新的支持情况——本设计已给出首选方案 + 退化方案，不构成 BLOCKED。
> - AC-16 / AC-17 为交付后人工验证、非完成闸门（RA §8.1 / PM 决策 5 已定），本设计为其提供了 workflow 与脚本层的支撑。
