# LanDrop：自动发布、签名公证、Homebrew 分发 设计文档

- 日期：2026-11-10
- 状态：已与用户确认核心决策，待用户审阅全文

标记约定：**[用户定]** = 用户在讨论中明确决定；**[我的判断]** = 设计方自行决定，用户可推翻；**[留给实现方]** = 不影响最终效果，由实现方决定。

---

## 1. 意图与动机

LanDrop 的目标用户在 mac / Windows / Linux 上，习惯用命令行装软件 **[用户定]**。他们装好后的第一件事是运行 `landrop` 启动服务，然后在局域网里互传内容。

目前要用 LanDrop，只能自己 `make all` 从源码构建：需要装 Go 1.26 和 Node 22。这对"装完就用"的用户门槛太高。

目标体验：

- mac：`brew install lupguo/tap/landrop`，然后 `landrop`。
- mac / Linux：`curl -fsSL https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh | sh`。
- Windows：`irm https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.ps1 | iex`。
- 想手动下载的用户：在 GitHub Releases 页面下载对应平台的压缩包。

维护者（用户本人）第一次接触 Homebrew 发布，对 CI 不太熟 **[用户定]**。所以除了自动化本身，还要交付一份足够细的操作手册，并写清每一步的安全风险。

## 2. 目标 / 非目标 / 反目标

**目标**

1. 推送 `v*` tag（例如 `v0.1.0`）时，CI 自动构建 5 个平台的发布包，并发布到 GitHub Releases **[用户定：只在 tag 时发布]**。
2. mac 二进制在 CI 中完成 Developer ID 签名和 Apple 公证 **[用户定：这次就做]**。
3. 每次发布自动更新 `lupguo/homebrew-tap` 仓库里的安装配置 **[用户定：仓库名]**。
4. 提供 `install.sh`（mac/Linux）和 `install.ps1`（Windows）一键安装脚本。
5. 默认端口从 8848 改为 **9958** **[用户定]**。
6. README 中英文两部分都加上产品截图（`imgs/web.jpg`、`imgs/cmd.jpg`）和各平台安装方法 **[用户定]**。
7. 交付一份中文操作手册 `docs/release-setup.md`，内容包括：证书、密钥、Token 的获取与配置，第一次发版，以及出问题或泄露时怎么办。

**非目标**

- 提交到官方 homebrew-core。项目知名度不够，暂不考虑。
- Windows 代码签名（需要另买证书）。Windows 用户仍会看到 SmartScreen 提示。
- Linux 包管理（deb/rpm/apt 源）、scoop、winget、自动更新。
- 每次推送 main 都跑的常规 CI（测试/lint）。这次不做，以后可以单独加。

**反目标（技术上能跑，但算失败）**

- 用 `xattr -dr com.apple.quarantine` 之类的 hook 去掉隔离标记来绕过 Gatekeeper。既然已经签名公证，就不应该再削弱用户的系统安全。
- 证书、私钥、Token 出现在仓库、日志或 Release 附件里。
- 用户执行安装脚本后，拿到的二进制没有经过校验和验证。
- 发版流程依赖维护者本地机器（例如必须在自己的 Mac 上签名）。发版应该只需要 `git push` 一个 tag。

## 3. 约束与现状

- 仓库：`github.com/lupguo/landrop`。目前是**私有**仓库，没有 `.github/`，也没有任何 tag。第一次发版前改为**公开** **[用户定]**。匿名下载 Release、raw 脚本和 brew 都依赖公开仓库，GitHub 免费账号的 Environment 规则和 tag 保护规则也需要公开仓库才能用。
- 构建顺序：必须先构建 Web（`cd web && npm ci && npm run build`，输出到 `internal/webui/dist`，再由 `go:embed` 嵌入），然后构建 Go。见 `Makefile`。
- 版本信息通过 ldflags 注入 `internal/version.{Version,Commit,Date}`，已有 `--version` 参数。CI 产物也必须注入，而且 `Version` 要带 `v` 前缀，与 `git describe` 的格式保持一致。
- 目标平台沿用 `Makefile` 的 `TARGETS`：darwin/arm64、darwin/amd64、windows/amd64、linux/amd64、linux/arm64。全部 `CGO_ENABLED=0`，并加 `-trimpath -s -w`。
- `/dist/` 已在 `.gitignore` 中。GoReleaser 默认也输出到 `dist/`，可以直接沿用。

## 4. 已决定的做法

### 4.1 发布工具链 **[我的判断]**

- GitHub Actions + GoReleaser（OSS 版，v2.19 及以上）。
- 工作流 `.github/workflows/release.yml`：
  - 触发条件：`push: tags: ['v*']`。
  - 运行环境：`ubuntu-latest`。GoReleaser 的跨平台 `notarize.macos` 不需要 macOS runner。
  - 步骤：checkout（`fetch-depth: 0`）→ setup-node 22 → setup-go（版本取自 `go.mod`）→ `go test ./...` 和 web 单元测试（测试失败则不发布）→ 构建 Web → `goreleaser release --clean`。
  - 顶层权限：`permissions: contents: write`，不给其他权限。
  - job 绑定 GitHub Environment `release`，所有密钥都放在这个 Environment 里，不放仓库级 secrets（见 §5）。
- `.goreleaser.yaml`：
  - `before.hooks` 不做 Web 构建，Web 由工作流的前一步完成，GoReleaser 只负责 Go 部分。**[留给实现方]** 也可以改放到 hooks 里，只要保证构建顺序正确。
  - 压缩包命名**不带版本号**：`landrop_<os>_<arch>.tar.gz`，Windows 为 `.zip`。这样安装脚本可以直接用 `https://github.com/lupguo/landrop/releases/latest/download/<文件名>` 下载最新版，不需要调 GitHub API。**[我的判断]**
  - 压缩包内包含 `landrop`（Windows 为 `landrop.exe`）、`README.md`，以及 LICENSE（如果有）。
  - 生成 `checksums.txt`（SHA-256）。
  - 预发布 tag（含 `-`，例如 `v0.1.0-rc.1`）：`release.prerelease: auto`，cask 设 `skip_upload: auto`。这样预发布版本不会成为 latest，也不会更新 tap，适合用来演练。第一个正式版本建议是 `v0.1.0`。手册里写明"先推 rc 演练，再推正式版"。**[我的判断]**
  - Release 直接发布，不用 draft。**[我的判断]**
  - Release 说明由 GoReleaser 根据提交记录自动生成。**[留给实现方]** 决定是否按 `feat`/`fix` 分组。

### 4.2 mac 签名与公证 **[用户定：做；具体方式为我的判断]**

- 使用 GoReleaser 的 `notarize.macos`，只作用于两个 darwin 构建：
  - `sign.certificate`：`{{ .Env.MACOS_SIGN_P12 }}`，值为 base64 编码的 .p12。
  - `sign.password`：`{{ .Env.MACOS_SIGN_PASSWORD }}`。
  - `notarize.issuer_id`、`notarize.key_id`、`notarize.key`：值为 base64 编码的 .p8。
  - `wait: true`，`timeout: 20m`。
  - `enabled: '{{ isEnvSet "MACOS_SIGN_P12" }}'`：没配置密钥时跳过签名，fork 和本地 `goreleaser release --snapshot` 也能跑通。
- 已知限制：单个命令行二进制无法 staple 公证票据。用户第一次运行时，macOS 会联网向 Apple 校验，这是正常行为。手册里要写明。
- 签名完成后再打包和计算校验和，保证 `checksums.txt` 对应的是签名后的文件。实现方需要验证 GoReleaser 的执行顺序确实如此。

### 4.3 Homebrew **[用户定：tap 仓库 `lupguo/homebrew-tap`，可复用给以后的项目]**

- 使用 GoReleaser 的 `homebrew_casks`（新版推荐方式，旧的 formula 方式已不推荐）。生成 `Casks/landrop.rb`，推送到 `lupguo/homebrew-tap` 的默认分支。
- 不加移除隔离标记的 hook（见反目标）。
- 用户安装命令：`brew install lupguo/tap/landrop`，第一次会自动 tap。升级：`brew upgrade landrop`。
- cask 只覆盖 macOS。Linux 用户使用 install.sh，README 中不推荐 Linuxbrew。**[我的判断]**
- 写入 tap 仓库使用一个单独的 fine-grained PAT（`HOMEBREW_TAP_GITHUB_TOKEN`），权限只限 `lupguo/homebrew-tap` 这一个仓库的 Contents 读写（见 §5）。
- 以后的其他项目也往同一个 tap 推送各自的 `Casks/<name>.rb`，可以共用这个 Token，也可以按项目各建一个。手册中说明。

### 4.4 安装脚本 **[我的判断]**

放在 `scripts/install.sh` 和 `scripts/install.ps1`，通过 `raw.githubusercontent.com/lupguo/landrop/main/...` 引用。

`install.sh`（POSIX sh，兼容 mac 自带 sh 和常见 Linux）：

- 用 `uname -s` / `uname -m` 识别平台，`x86_64` 映射为 amd64，`arm64`/`aarch64` 映射为 arm64。遇到不支持的平台，给出清楚的报错并退出。
- 下载对应压缩包和 `checksums.txt`，用 `sha256sum` 或 `shasum -a 256` 校验。校验失败立即退出，不安装。
- 安装目录：优先用 `LANDROP_INSTALL_DIR`；否则 `/usr/local/bin` 可写就装到那里；不可写时装到 `~/.local/bin`，并提示用户把它加入 PATH。不自动 sudo，需要提权时提示用户。
- 指定版本：`LANDROP_VERSION=v0.1.0`，此时从 `releases/download/<tag>/` 下载；默认下载 latest。
- 用 curl 下载，没有 curl 时回退到 wget。临时目录在退出时清理。
- 安装完成后打印 `landrop --version` 的结果和下一步提示（"运行 `landrop` 启动"）。

`install.ps1`（Windows PowerShell 5.1 及以上）：

- 下载 `landrop_windows_amd64.zip` 和 `checksums.txt`，用 `Get-FileHash` 校验。
- 解压到 `$env:LOCALAPPDATA\Programs\landrop`，并把该目录加入**用户级** PATH（不需要管理员权限）。提示用户重开终端。
- 同样支持 `LANDROP_VERSION`。

README 中对 `curl | sh` 和 `irm | iex` 加一句安全提示：可以先下载脚本查看内容再执行。

### 4.5 默认端口 9958 **[用户定]**

- `internal/config/config.go` 中的默认值改为 9958。
- 断言默认值的测试同步修改：`internal/config/config_test.go` 中检查 `c.Port` 默认值和 help 输出包含端口号的两处。
- `web/vite.config.ts` 中的开发代理目标改为 9958。
- README（中英文的 Flags 和 Windows 防火墙命令）、`LandropPRD.md` 中的 F-01、示意地址，以及把 Q1（默认端口）标为已定 9958。**[我的判断]**
- 其他测试和文档里作为示例出现的 8848（host_test、banner_test、qr_test、server_test、share_handlers_test、testutil_test、instance_test、`server.go` 注释、`LandropPRD.md` 中的示意地址、已有 specs 中的示例等）**也全部改为 9958** **[用户定]**，保证仓库里不再出现旧端口。e2e 专用端口 18848 是另一个独立端口，不改 **[我的判断]**。

### 4.6 README **[用户定：截图 + 各平台安装方法]**

- 英文标题下方并排放两张截图，Web 在左、终端在右，用 HTML `<img width=...>` 控制大小。中文部分开头再放一次（或引用同一组图）。**[留给实现方]** 决定具体宽度。
- 新增 **Install / 安装** 一节，放在 Quick start 之前，按顺序列出：
  1. macOS（Homebrew，推荐）
  2. macOS / Linux（install.sh）
  3. Windows（PowerShell）
  4. 手动下载（Releases 页面，附校验方法）
- Quick start 改为"安装后运行 `landrop`"。原来的 `make all` 内容移到 "Build from source"。
- "Running an unsigned binary" 改为只讲 Windows SmartScreen，并说明 mac 版本已签名公证，第一次运行需要联网校验。
- 所有端口示例改为 9958。

### 4.7 操作手册 `docs/release-setup.md`（中文） **[用户定：讲清楚、提醒安全风险]**

面向第一次做这件事的人，每一步写清"在哪里点什么、得到什么、存到哪里"。章节：

0. **把仓库改为公开前的检查**：用 gitleaks（或 `git log -p` 加关键词搜索）扫描全部提交历史，确认没有密钥、Token、私人路径或文件；确认 `.superpowers/`、`.idea/` 从未被提交；确认 `LandropPRD.md` 等文档可以公开。全部确认后，再到 Settings → General → Danger Zone → Change visibility 改为 Public。提醒用户：仓库一旦公开，历史就可能已被他人克隆或缓存，事后改回私有也收不回。
1. **总览**：一张表列出 6 个 secret 的名称、来源、用途，以及泄露后果。
2. **Developer ID Application 证书**：需要 Account Holder 角色。用户有个人开发者账号，默认本人就是 Account Holder **[我的判断]**；如果不是，手册要说明需请 Account Holder 代为创建。在 Keychain 中生成 CSR，到 developer.apple.com 创建证书；导出 .p12 并设置强密码；用 `base64 -i cert.p12 | pbcopy` 转码。提醒：每个账号的 Developer ID 证书数量有限，有效期 5 年。
3. **App Store Connect API Key**：Users and Access → Integrations → Team Keys；角色选 Developer，用最小权限；.p8 **只能下载一次**；记录 Issuer ID 和 Key ID。
4. **创建 `lupguo/homebrew-tap` 仓库**：公开仓库，带一个 README。
5. **Fine-grained PAT**：Only select repositories 只选 homebrew-tap；Contents 读写；设置过期时间（建议 1 年）并加日历提醒续期。
6. **在 landrop 仓库配置 Environment `release`**：Deployment branches and tags 限定为 `v*` tag；把 6 个 secret 加进去。
7. **建议开启**：给 `v*` 加 tag 保护规则（只有自己能推）；开启账号两步验证。
8. **第一次发版**：`git tag v0.1.0 && git push origin v0.1.0` → 在 Actions 页面看进度 → 检查 Release 附件 → 本地验证：`codesign -dv --verbose=4`、`spctl -a -vvv -t install`、`brew install lupguo/tap/landrop`。
9. **常见失败及排查**：公证超时或被拒、tap 推送 403、tag 推错了怎么删和重发。
10. **安全风险与应急**：
    - .p12 加密码泄露：别人可以用你的名义签恶意软件。应立即到 developer.apple.com 吊销证书，然后重新生成。
    - .p8 泄露：在 App Store Connect 吊销该 Key。
    - PAT 泄露：在 GitHub 设置中删除。因为权限限定在 tap 仓库，影响范围是 tap 里的安装配置可能被篡改，应检查 tap 仓库的提交历史。
    - 本地明文文件（.p12、.p8、base64 文本）用完后从下载目录删除，原件保存在密码管理器里。
    - 不要在 PR 触发的 workflow 中使用这些 secret；Environment 只对 tag 开放。
    - 引入第三方 Action 时固定到具体版本（建议固定 commit SHA）。

## 5. 密钥一览（实现与手册共用的名称）

| Secret | 内容 | 用途 |
|---|---|---|
| `MACOS_SIGN_P12` | base64 编码的 Developer ID Application .p12 | 签名 |
| `MACOS_SIGN_PASSWORD` | .p12 的导出密码 | 签名 |
| `MACOS_NOTARY_ISSUER_ID` | App Store Connect Issuer ID | 公证 |
| `MACOS_NOTARY_KEY_ID` | API Key ID | 公证 |
| `MACOS_NOTARY_KEY` | base64 编码的 .p8 | 公证 |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Fine-grained PAT，只能写 homebrew-tap | 更新 cask |

`GITHUB_TOKEN` 由 Actions 自动提供，用于创建 Release。

## 6. 验收

- 推送 `v0.1.0-rc.1` 后，Release 标为 pre-release，tap 没有变化；推送 `v0.1.0` 后，Release 中有 5 个压缩包和 `checksums.txt`；darwin 二进制 `codesign -dv` 显示 Developer ID，`spctl` 显示 accepted / Notarized Developer ID。
- `homebrew-tap` 中出现 `Casks/landrop.rb`；在一台干净的 Mac 上执行 `brew install lupguo/tap/landrop && landrop --version`，不弹 Gatekeeper 警告。
- install.sh 在 mac（arm64）和 Linux（amd64）上能装好；install.ps1 在 Windows 上能装好，重开终端后 `landrop --version` 可用。
- 没有任何参数时，`landrop` 监听 9958；`go test ./...` 和 web 测试通过。
- 没有配置密钥时，`goreleaser release --snapshot --clean` 在本地能跑通，只是跳过签名。

## 7. 留给实现方

- Action 和 GoReleaser 的具体版本号（满足 §4.1 的最低版本即可，固定版本）。
- `.goreleaser.yaml` 的细节写法（ldflags 模板、changelog 分组、archive 中附带的文件）。
- 是否在 Makefile 中加 `make release-snapshot` 方便本地试跑。
- 安装脚本的输出文案与颜色。
- README 截图宽度和排版。
