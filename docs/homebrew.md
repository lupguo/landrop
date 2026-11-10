# 用 Homebrew 安装和使用 LanDrop

LanDrop 通过自建的 Homebrew tap [`lupguo/homebrew-tap`](https://github.com/lupguo/homebrew-tap) 分发，安装包类型是 **cask**，只支持 macOS（Apple Silicon 和 Intel 都支持）。Linux 和 Windows 请使用 [README](../README.md#install) 里的一键安装脚本。

- 第一部分：给使用者
- 第二部分：给维护者（tap 仓库如何运作，以后怎么加其他项目）

---

## 第一部分：使用者

### 1. 安装

```sh
brew install lupguo/tap/landrop
```

第一次执行时，brew 会自动 `brew tap lupguo/tap`，也就是把 `github.com/lupguo/homebrew-tap` 克隆到本地，然后下载与你的 CPU 匹配的 `landrop_darwin_<arch>.tar.gz`，核对 sha256 后安装。

也可以分两步做，效果一样：

```sh
brew tap lupguo/tap
brew install --cask landrop
```

### 2. 启动

```sh
landrop --version     # 输出包含版本号，例如 landrop v0.1.0 (<commit>, <date>)
landrop               # 启动服务，默认端口 9958
```

终端会显示局域网地址、4 位访问码和二维码。其他设备用浏览器打开这个地址（或扫码）就能互传文本和文件。常用参数（`--port`、`--dir`、`--no-open` 等）见 `landrop --help` 或 README。

**第一次运行时：**

- LanDrop 的 mac 版本已经用 Developer ID 签名，并经过 Apple 公证。第一次运行时，macOS 会联网向 Apple 校验一次，可能要等一两秒，这是正常的。不需要、也不要执行 `xattr -d com.apple.quarantine`。
- macOS 可能会询问"是否允许终端查找本地网络上的设备"，请选**允许**，否则其他设备连不上。之后可以在"系统设置 → 隐私与安全性 → 本地网络"里修改。

### 3. 升级

```sh
brew update            # 拉取 tap 的最新内容
brew upgrade landrop   # 或：brew upgrade --cask landrop
```

每次 LanDrop 发布正式版，发布流程都会自动更新 tap。只有正式版会同步到 brew，预发布版本（例如 `v0.2.0-rc.1`）不会。

### 4. 查看信息

```sh
brew info --cask lupguo/tap/landrop   # 当前版本、是否已安装、下载地址
brew list --cask | grep landrop       # 确认已安装
command -v landrop                    # 实际运行的是哪一份
```

brew 会把二进制放在 `$(brew --prefix)/Caskroom/landrop/<版本>/` 下，再在 `$(brew --prefix)/bin/landrop` 建一个符号链接。Apple Silicon 上 brew 前缀是 `/opt/homebrew`，Intel 上是 `/usr/local`。

### 5. 卸载

```sh
brew uninstall --cask landrop
brew untap lupguo/tap        # 可选：不再需要这个 tap 时执行
```

卸载不会删除 LanDrop 收到的文件。这些文件默认自动保存在 `~/Downloads/LanDrop`，需要的话请手动删除。

### 6. 安装指定版本

brew 的 cask 只跟踪最新版。需要固定某个旧版本时，请改用安装脚本：

```sh
curl -fsSL https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh | LANDROP_VERSION=v0.1.0 sh
```

### 7. 常见问题

| 现象 | 原因与处理 |
|---|---|
| `brew upgrade` 提示已是最新，但 GitHub 上有新版 | 先 `brew update`，让本地 tap 同步。新版是预发布（tag 带 `-`）时，brew 不会更新，这是预期行为。 |
| 安装时报 `It seems there is already a Binary at '/usr/local/bin/landrop'` | 之前用 install.sh 装过一份，占了同一个位置（Intel Mac 上常见）。先删掉旧的：`rm /usr/local/bin/landrop`，再重新 `brew install`。两种安装方式选一种即可，不要混用。 |
| `landrop: command not found` | 执行 `brew --prefix`，确认 `<前缀>/bin` 在 `PATH` 里。Apple Silicon 上需要先按 brew 的提示执行 `eval "$(/opt/homebrew/bin/brew shellenv)"`。 |
| 运行时提示"无法验证开发者"或"已损坏" | 正常情况下不会出现。请先确认版本：`brew info --cask lupguo/tap/landrop`。然后到 [Issues](https://github.com/lupguo/landrop/issues) 反馈，附上 `spctl -a -vvv -t install "$(command -v landrop)"` 的输出。**不要**通过删除隔离属性来绕过检查。 |
| 其他设备打不开地址 | 与 brew 无关。请查看 README 的 Network notes：本地网络权限、防火墙、路由器的 AP 隔离。 |
| 下载很慢或失败 | 安装包从 GitHub Releases 下载。网络受限时，可以设置代理（`export HTTPS_PROXY=...`）后重试。 |

---

## 第二部分：维护者

### 8. tap 仓库是什么

- 仓库：`github.com/lupguo/homebrew-tap`（公开）。本地路径：`/data/projects/github.com/lupguo/homebrew-tap`。
- 命名规则：GitHub 仓库名必须以 `homebrew-` 开头。用户写的 `lupguo/tap` 会被 brew 映射成 `github.com/lupguo/homebrew-tap`。
- 结构：

  ```
  homebrew-tap/
  ├── README.md          # 列出 tap 里的所有工具
  └── Casks/
      └── landrop.rb     # 由 landrop 的发布流程自动生成，不要手改
  ```

### 9. landrop.rb 是怎么更新的

1. 在 landrop 仓库推送正式版 tag（例如 `v0.1.0`）。
2. GitHub Actions 运行 GoReleaser。GoReleaser 依次构建、签名、公证、打包，并创建 GitHub Release。
3. GoReleaser 根据 `.goreleaser.yaml` 的 `homebrew_casks` 配置生成 `Casks/landrop.rb`。文件里包含新版本号、两个 mac 压缩包的下载地址和 sha256。
4. GoReleaser 使用 secret `HOMEBREW_TAP_GITHUB_TOKEN`（一个只能写 homebrew-tap 的 fine-grained PAT）把这个文件提交到 tap 仓库的 `main` 分支。

预发布 tag 不会触发第 3、4 步。Token 怎么创建、怎么配置、怎么轮换，见 [release-setup.md](release-setup.md) 第 5、6 章。第一次发版和撤回 tap 提交，见第 8 章和 9.6 节。

> ⚠️ 安全提醒
> - tap 里的 cask 决定了 brew 用户从哪里下载什么文件。谁能写这个仓库，谁就能把用户引到恶意文件。所以写入权限只给那一个 PAT，并且要设过期时间。
> - 定期查看 tap 的提交历史（`git log -p -- Casks/`）。每个提交都应该来自你的某次发版，`url` 和 `sha256` 应该与对应 Release 的 `checksums.txt` 一致。

### 10. 以后在这个 tap 里加其他项目

同一个 tap 可以放任意多个工具，每个工具一个 `Casks/<name>.rb`。新项目用 GoReleaser 时：

1. 在新项目的 `.goreleaser.yaml` 里加上下面这段，把 `<name>` 换成新项目的名字，其余参照 landrop 的配置：

   ```yaml
   homebrew_casks:
     - name: <name>
       ids: [<darwin 的 archive id>]
       directory: Casks
       repository:
         owner: lupguo
         name: homebrew-tap
         token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
       skip_upload: auto
   ```

2. Token 有两种选择：
   - **共用**：把同一个 PAT 也配到新项目的 `release` Environment。省事，但这个 PAT 泄露时，所有项目的 cask 都会受影响。
   - **分开（推荐）**：每个项目单独建一个 PAT，权限同样只给 homebrew-tap 的 Contents 读写。某个项目出问题时，只需要吊销它自己的 PAT。
3. 在 tap 的 `README.md` 表格里加一行，写上新工具的名字和安装命令。
4. 用户的安装命令是 `brew install lupguo/tap/<name>`。

### 11. 手动操作 tap（尽量避免）

只有在撤回错误版本时才需要手动改 tap，步骤见 release-setup.md 9.6 节：用 `git revert` 撤回，不要改写历史，也不要 force push。正常情况下，tap 只由发布流程写入。
