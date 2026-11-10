# LanDrop 发布配置手册（维护者用）

这份手册写给第一次做下面这些事的维护者：

- 用 GitHub Actions 自动发版；
- 用 Apple Developer ID 给 mac 二进制签名，并提交 Apple 公证；
- 用自己的 Homebrew tap（`lupguo/homebrew-tap`）分发。

每一步都写清三件事：在哪里点什么，得到什么，存到哪里。凡是创建、保存、传输或使用密钥的地方，都有 `> ⚠️ 安全提醒`。第 10 章把所有提醒和泄露后的吊销步骤汇总在一起。

> 截至 2026-11，界面文字已对照 Apple 和 GitHub 官方文档核对过。两家都会改界面文字。如果你看到的按钮名和本文略有不同，按意思找同一个位置即可，路径和顺序不会变。

## 目录

- [0. 把仓库改为公开前的检查](#0-把仓库改为公开前的检查)
- [1. 总览](#1-总览)
- [2. Developer ID Application 证书](#2-developer-id-application-证书)
- [3. App Store Connect API Key](#3-app-store-connect-api-key)
- [4. 创建 lupguo/homebrew-tap 仓库](#4-创建-lupguohomebrew-tap-仓库)
- [5. Fine-grained PAT](#5-fine-grained-pat)
- [6. 在 landrop 仓库配置 Environment release](#6-在-landrop-仓库配置-environment-release)
- [7. 建议开启的保护](#7-建议开启的保护)
- [8. 第一次发版](#8-第一次发版)
- [9. 常见失败及排查](#9-常见失败及排查)
- [10. 安全风险与应急](#10-安全风险与应急)

**建议顺序：** 0 → 2 → 3 → 4 → 5 → 6 → 7 → 8。第 6、7 章依赖仓库已经公开。GitHub 免费账号只能在公开仓库上使用 Environment 和 tag 规则集。

---

## 0. 把仓库改为公开前的检查

公开之后，全部提交历史都会对外可见，包括已经删除的文件和早期提交。

> ⚠️ 安全提醒
> 仓库一旦公开，就可能在几分钟内被别人克隆、fork，或被搜索引擎和代码归档服务缓存。事后改回私有，这些副本也收不回来。所以公开这一步**在效果上不可撤销**。如果历史里有密钥，唯一安全的处理方法是先**吊销那个密钥**，再考虑改写历史。只删文件或改写历史是不够的。

### 0.1 用 gitleaks 扫描全部提交历史

```sh
cd /path/to/landrop
brew install gitleaks && gitleaks git -v .
```

- `gitleaks git` 会扫描 git 历史中的每一个提交，不只是当前文件。`-v` 会打印每条发现的详情。
- 上面的命令只扫当前分支能看到的历史。要连同所有本地分支和 tag 一起扫，运行：

  ```sh
  gitleaks git -v --log-opts="--all" .
  ```

- 结果是 `no leaks found` 才算通过。如果有发现，逐条看：是真密钥就先去对应平台吊销，再处理历史；是误报（例如测试里的假数据）就记下来。

### 0.2 用关键词再人工搜一遍

gitleaks 认不出所有东西，例如私人路径和内网地址。再补一遍关键词搜索：

```sh
# 历史中出现过的所有文件路径：确认没有 .superpowers/、.idea/、证书、密钥文件
git log --all --name-only --format= | sort -u
git log --all --name-only --format= | sort -u | grep -E '^\.(superpowers|idea)/'   # 期望：没有输出
git log --all --name-only --format= | sort -u | grep -iE '\.(p12|p8|cer|pem|key|mobileprovision)$'   # 期望：没有输出

# 在所有提交的改动内容中搜敏感关键词
git log --all -p | grep -inE 'BEGIN [A-Z ]*PRIVATE KEY|ghp_|github_pat_|AuthKey_|password *=|/Users/[a-z]' | head -50
```

还要检查下面几项：

- `git log --all --format='%an <%ae>' | sort -u`：这些作者名和邮箱公开后所有人都能看到。确认你愿意公开它们。
- `LandropPRD.md`、`docs/` 下的设计文档、`README.md`：确认没有不想公开的内容，例如私人计划、客户名、内网 IP。
- `imgs/` 里的截图（如果要提交）：确认截图里没有私人信息，例如真实文件名、内网地址、二维码里的地址。

### 0.3 改为公开

全部确认后，有两种做法。

**网页：** 打开 `https://github.com/lupguo/landrop` → **Settings** → **General**（默认就在这一页）→ 拉到最底部的 **Danger Zone** → **Change visibility** → **Change to public**，然后按提示输入仓库名确认。

**命令行（gh）：**

```sh
gh repo edit lupguo/landrop --visibility public --accept-visibility-change-consequences
```

---

## 1. 总览

### 1.1 你需要准备

- Apple Developer Program 会员资格（付费个人账号）。本手册假设你本人就是 **Account Holder**，个人账号默认如此。
- 一台 Mac，用来生成证书和本地验证签名。
- GitHub CLI：`brew install gh`，然后运行 `gh auth login`。
- 一个密码管理器（1Password、Bitwarden、macOS「密码」App 等），用来长期保存证书、密钥和密码的原件。

### 1.2 6 个 secret

全部放在 landrop 仓库的 GitHub Environment **`release`** 里，不放仓库级 secrets。名字必须一字不差，`.goreleaser.yaml` 和 `.github/workflows/release.yml` 按这些名字读取。

| Secret | 内容 | 从哪里来 | 用途 | 泄露后果 |
|---|---|---|---|---|
| `MACOS_SIGN_P12` | base64 编码的 Developer ID Application `.p12`（证书和私钥） | 第 2 章：钥匙串导出 | 签名 mac 二进制 | 与密码一起泄露时，别人可以**以你的名义签名恶意软件** |
| `MACOS_SIGN_PASSWORD` | `.p12` 的导出密码 | 第 2 章：你自己设定 | 打开 `.p12` | 与 `.p12` 一起泄露，后果同上 |
| `MACOS_NOTARY_ISSUER_ID` | App Store Connect Issuer ID（UUID） | 第 3 章：Team Keys 页面顶部 | 公证 | 单独泄露风险低，但它是 API Key 的组成部分 |
| `MACOS_NOTARY_KEY_ID` | API Key ID | 第 3 章：Team Keys 列表 | 公证 | 单独泄露风险低 |
| `MACOS_NOTARY_KEY` | base64 编码的 `.p8` 私钥 | 第 3 章：生成后只能下载一次 | 公证 | 与上面两个 ID 一起，别人可以按该 Key 的角色调用你的 App Store Connect API，包括用你的账号提交公证 |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Fine-grained PAT，只能写 `lupguo/homebrew-tap` | 第 5 章：GitHub Developer settings | 发版时把 `Casks/landrop.rb` 推到 tap | 别人可以篡改 tap 里的安装配置，把 brew 用户引到恶意下载地址 |

`GITHUB_TOKEN` 由 Actions 自动提供，不需要你创建。工作流用它创建 Release，权限只有 `contents: write`。

### 1.3 发版流程

1. 你推送一个 `v*` tag，例如 `v0.1.0`。
2. `.github/workflows/release.yml` 在 `ubuntu-latest` 上运行，job 绑定 Environment `release`，只有这时才能读到上面的 secret。
3. 依次运行 Go 测试、Web 测试和构建。任何测试失败，都不会发布。
4. GoReleaser 构建 5 个平台的二进制，再对两个 darwin 二进制做签名和公证。这一步在 Linux 上跨平台完成，不需要 Mac runner。
5. 打包：`landrop_darwin_amd64.tar.gz`、`landrop_darwin_arm64.tar.gz`、`landrop_linux_amd64.tar.gz`、`landrop_linux_arm64.tar.gz`、`landrop_windows_amd64.zip`，然后生成 `checksums.txt`（SHA-256，对应签名后的文件）。
6. 创建 GitHub Release（`release.prerelease: auto`）。
7. 生成 `Casks/landrop.rb`，推到 `lupguo/homebrew-tap`（`skip_upload: auto`）。

**两个自动规则：**

- tag 里含 `-` 的是预发布，例如 `v0.1.0-rc.1`。它会被标为 Pre-release，**不会成为 latest**，**也不会更新 tap**。
- 没有配置 `MACOS_SIGN_P12` 时，签名公证会被跳过，其余步骤照常进行。这适用于 fork、本地 `goreleaser release --snapshot --clean`，或者你还没配好密钥的时候。GoReleaser 的 `isEnvSet` 把空值也当作未设置。

---

## 2. Developer ID Application 证书

**角色要求：** Apple 当前文档规定只有 **Account Holder** 能创建 Developer ID 证书。Admin 只有在被额外授予 cloud-managed Developer ID 证书权限时才能创建，而云端托管证书的私钥留在 Apple 服务器上，导不出本流程需要的 `.p12`。个人开发者账号的本人就是 Account Holder。如果你的账号不是，请 Account Holder 按本章步骤代为创建，然后把 `.p12` 和密码**通过密码管理器的共享功能**交给你，不要用聊天工具或邮件。

**数量和有效期：** 每个账号最多同时持有 5 张 Developer ID Application 证书。Apple 当前文档写明，从 G2 中间 CA 签发的 Developer ID 证书**有效期一年，需要每年续**。实际到期日以证书上显示的为准。已经签名并公证的旧版本，在证书过期后仍可运行；但发新版之前必须换新证书。

### 2.1 在钥匙串中生成 CSR（证书签名请求）

1. 打开「钥匙串访问」（Keychain Access）。较新的 macOS 不在「实用工具」文件夹里，用 Spotlight 搜索 `Keychain Access`，或运行 `open -a "Keychain Access"`。
2. 菜单栏选 **Keychain Access（钥匙串访问）→ Certificate Assistant（证书助理）→ Request a Certificate From a Certificate Authority（从证书颁发机构请求证书）**。
3. 填写：
   - **User Email Address**：你的 Apple 账号邮箱；
   - **Common Name**：便于识别的名字，例如 `LanDrop Developer ID`；
   - **CA Email Address**：留空；
   - 选 **Saved to disk（存储到磁盘）**，点 **Continue**。
4. 得到一个 `CertificateSigningRequest.certSigningRequest` 文件。同时，钥匙串里已经生成了配套的**私钥**。这个私钥只存在于这台 Mac 上。

> ⚠️ 安全提醒
> 私钥此刻只在你这台 Mac 的登录钥匙串里，它就是签名能力本身。不要在别人的电脑或公用电脑上做这一步。

### 2.2 在 developer.apple.com 创建证书

1. 登录 `https://developer.apple.com/account` → **Certificates, Identifiers & Profiles** → 左侧栏 **Certificates** → 左上角 **+**。
2. 在 **Software** 下选 **Developer ID**，点 **Continue**。
3. 选 **Developer ID Application**（用来签 Mac 应用和二进制；**不要**选 Developer ID Installer，那是给 `.pkg` 安装包用的）。
4. 如果页面要求选择 Developer ID Certificate Intermediary，选 **G2 Sub-CA (Xcode 11.4.1 or later)**。选别的可能拿到旧 CA 签发的证书，旧 CA 将于 2027-02-01 失效。
5. **Choose File** 选上一步的 `.certSigningRequest` 文件 → **Continue** → **Download**，得到一个 `.cer` 文件，在「下载」文件夹里。
6. 双击 `.cer`，把它装进**登录**钥匙串。

### 2.3 导出 .p12

1. 在钥匙串访问里选 **login（登录）** 钥匙串，切到 **My Certificates（我的证书）** 分类。
2. 找到 `Developer ID Application: 你的名字 (TEAMID)`，点左边的小三角，确认下面挂着一把私钥。没有私钥说明 CSR 不是在这台 Mac 上生成的，导出的 `.p12` 无法签名。
3. 右键这张证书（不是私钥）→ **Export（导出）"Developer ID Application: …"**。
4. 文件格式选 **Personal Information Exchange (.p12)**，文件名用 `cert.p12`，保存到一个临时工作目录（见下面的提醒）。
5. 设置导出密码：用密码管理器生成一个**强随机密码**（至少 20 位），先存进密码管理器，再粘贴到对话框。这个密码就是 `MACOS_SIGN_PASSWORD`。
6. 系统会要求输入 Mac 登录密码，以允许导出私钥。

建议的临时工作目录（不在 iCloud 同步范围内，只有你能读）：

```sh
mkdir -p ~/landrop-secrets-tmp && chmod 700 ~/landrop-secrets-tmp
```

> ⚠️ 安全提醒
> - `.p12` 加上密码，就等于你的签名身份。不要放进 iCloud 同步的「桌面」或「文稿」文件夹，不要放进任何 git 仓库目录，不要通过微信、邮件或 Slack 发送。
> - 原件（`.p12` 文件和密码）长期保存在密码管理器里。密码管理器可以存附件。本地文件在第 6 章用完后删除。
> - 密码不要用你在别处用过的密码，也不要写在备忘录里。

### 2.4 转成 base64

GoReleaser 读的是 base64 文本。macOS 的 `base64` 默认输出单行，正好符合要求：

```sh
cd ~/landrop-secrets-tmp
base64 -i cert.p12 | pbcopy
```

这样剪贴板里就是 `MACOS_SIGN_P12` 的值，可以直接粘贴到第 6 章的网页里。更推荐第 6 章的 `gh secret set` 管道写法，它完全不经过剪贴板。

> ⚠️ 安全提醒
> - 剪贴板会被其他 App 读到。开了「通用剪贴板」（Handoff）时，还会同步到你的 iPhone 和 iPad。粘贴完立刻清空：`pbcopy < /dev/null`。如果你用了剪贴板历史工具（Raycast、Paste、Alfred 等），去它的历史里删掉这一条。
> - 不要把 base64 结果另存成文本文件。它和 `.p12` 一样敏感。

---

## 3. App Store Connect API Key

公证需要一把 App Store Connect API **Team Key**。它包括三样东西：Issuer ID、Key ID 和 `.p8` 私钥文件。

**角色要求：** 只有 Account Holder 或 Admin 能生成 Team Key。第一次使用 App Store Connect API 时，Account Holder 需要先申请开通：**Users and Access → Integrations → Request Access**，勾选同意条款后点 **Submit**。Apple 会逐个审核，通常很快。

### 3.1 生成 Key

1. 登录 `https://appstoreconnect.apple.com` → **Users and Access（用户和访问）** → 顶部 **Integrations（集成）** 标签。默认选中的就是左侧的 **App Store Connect API**。
2. 点 **Team Keys**（团队密钥）。注意不要选 Individual Keys，个人密钥不能用于 notarytool。
3. 点 **Generate API Key**。如果已经有其他 Key，就点 **+**。
4. **Name**：写一个能看出用途的名字，例如 `landrop-notary-ci`。
5. **Access**：选 **Developer**。Developer 是能完成公证的最低角色（社区多次验证 notarytool 用 Developer 角色可以工作），不要选 Admin 或 App Manager。Team Key 对整个团队的所有 App 生效，不能限定到单个 App；Key 生成后，名字和角色**都不能修改**，要改只能吊销后重建。
6. 点 **Generate**。

### 3.2 下载 .p8 并记录 ID

1. 在新 Key 那一行点 **Download**，得到 `AuthKey_XXXXXXXXXX.p8`。**`.p8` 只能下载这一次。** 刷新页面后，下载按钮就没有了。把它放进 `~/landrop-secrets-tmp`，并立刻存一份到密码管理器。
2. **Key ID**：列表中这一行的 Key ID 列，也就是文件名 `AuthKey_` 后面那一串。它就是 `MACOS_NOTARY_KEY_ID`。
3. **Issuer ID**：Team Keys 页面顶部显示的一个 UUID，同一团队的所有 Key 共用。它就是 `MACOS_NOTARY_ISSUER_ID`。
4. 转成 base64：

   ```sh
   cd ~/landrop-secrets-tmp
   base64 -i AuthKey_XXXX.p8 | pbcopy     # XXXX 换成你的 Key ID
   ```

   这就是 `MACOS_NOTARY_KEY` 的值。

> ⚠️ 安全提醒
> - `.p8` 加上 Key ID 和 Issuer ID，就能以这把 Key 的角色调用你的 App Store Connect API。所以角色要选最低的 Developer。
> - `.p8` 只能下载一次。丢了不能找回，只能吊销后重建。原件存进密码管理器。
> - 用完剪贴板就清空（`pbcopy < /dev/null`），见 2.4 的提醒。
> - 不要把 `.p8` 放进任何仓库目录。

---

## 4. 创建 lupguo/homebrew-tap 仓库

Homebrew 的规则是：`brew install lupguo/tap/landrop` 会自动去找 `github.com/lupguo/homebrew-tap` 这个仓库。

1. 打开 `https://github.com/new`。
2. **Owner**：`lupguo`；**Repository name**：`homebrew-tap`（必须以 `homebrew-` 开头）。
3. 选 **Public**。brew 用户需要匿名读取它。
4. 勾选添加 **README** 文件。仓库必须有默认分支和至少一个提交，GoReleaser 才能往里推送。
5. 点 **Create repository**。

不需要手动创建 `Casks/` 目录。第一次正式发版时，GoReleaser 会生成 `Casks/landrop.rb` 并提交到默认分支。

**给以后的项目复用：** 其他项目也可以往这个 tap 推各自的 `Casks/<name>.rb`。可以共用第 5 章的 Token，也可以每个项目单独建一个。单独建的好处是：某个项目的 Token 泄露或过期，不影响其他项目。

---

## 5. Fine-grained PAT

GoReleaser 往 tap 仓库推送时，用的是一个**只能写 homebrew-tap** 的 Token。Actions 自带的 `GITHUB_TOKEN` 只能访问 landrop 本仓库，推不了别的仓库。

1. GitHub 右上角头像 → **Settings** → 左侧栏最下面 **Developer settings** → **Personal access tokens** → **Fine-grained tokens** → **Generate new token**。
2. **Token name**：`landrop-homebrew-tap`。**Description**（可选）：`GoReleaser pushes Casks/landrop.rb`。
3. **Resource owner**：`lupguo`（你的个人账号）。
4. **Expiration**：选自定义日期，**建议 1 年**。不要选 No expiration。然后在日历里加一个「到期前 2 周续期」的提醒。GitHub 也会在到期前发邮件提醒。
5. **Repository access**：选 **Only select repositories**，然后在下拉框里**只选 `lupguo/homebrew-tap`**。不要选 All repositories。
6. **Permissions**：在仓库权限（Repository permissions）里找到 **Contents**，设为 **Read and write**。新版界面需要先点 **Add permissions** 再选 Contents。**Metadata: Read-only** 会自动加上，而且是必需的，保留即可。其他权限都不要加。
7. 点 **Generate token**，页面会显示一个以 `github_pat_` 开头的字符串。**它只显示这一次**，马上复制。这个值就是 `HOMEBREW_TAP_GITHUB_TOKEN`。

> ⚠️ 安全提醒
> - 这个 Token 能改 tap 里的安装配置。泄露后，别人可以把 cask 的下载地址改成恶意文件。所以范围要严格限定为 homebrew-tap 这一个仓库，权限只给 Contents，并且一定要设过期时间。
> - 复制后先存进密码管理器，再到第 6 章配置。用完剪贴板就清空。
> - 不要为了省事用 classic token（`ghp_` 开头）。它无法限定到单个仓库。

---

## 6. 在 landrop 仓库配置 Environment release

前提：landrop 仓库已经公开（第 0 章）。GitHub 免费账号只能在公开仓库上配置 Environment。如果仓库后来又改回私有，Environment 的保护规则和 secret 会被忽略。

### 6.1 创建 Environment

1. 打开 `https://github.com/lupguo/landrop` → **Settings** → 左侧栏 **Environments** → **New environment**。
2. **Name**：`release`（工作流里写的是 `environment: release`，名字必须一致）→ **Configure environment**。

### 6.2 限定只有 v* tag 能用

1. 在 **Deployment branches and tags** 下拉框里，选 **Selected branches and tags**（默认是 No restriction）。
2. 点 **Add deployment branch or tag rule** → **Ref type** 选 **Tag** → 名称规则填 `v*` → **Add rule**。
3. 确认列表里只有这一条 Tag 规则，没有任何 Branch 规则。

这样，只有从 `v*` tag 触发的运行才能进入这个 Environment、读到其中的 secret。从分支或 PR 触发的运行都读不到。

可选：**Required reviewers** 可以把自己加为审批人。之后每次发版，job 会先停在 Waiting，等你在 Actions 页面点批准再继续。这多了一道防误推的确认。注意**不要**勾选 Prevent self-review，否则你自己推的 tag 无法由你自己批准。

### 6.3 添加 6 个 secret

在同一页面的 **Environment secrets** 区域点 **Add secret**（添加 secret），**Name** 填名字，**Value** 粘贴值，然后点 **Add secret**。按下表逐个添加：

| Name | Value |
|---|---|
| `MACOS_SIGN_P12` | `base64 -i cert.p12` 的输出（2.4） |
| `MACOS_SIGN_PASSWORD` | `.p12` 导出密码（2.3） |
| `MACOS_NOTARY_ISSUER_ID` | Issuer ID（3.2） |
| `MACOS_NOTARY_KEY_ID` | Key ID（3.2） |
| `MACOS_NOTARY_KEY` | `base64 -i AuthKey_XXXX.p8` 的输出（3.2） |
| `HOMEBREW_TAP_GITHUB_TOKEN` | `github_pat_...`（第 5 章） |

注意：要加在 **Environment secrets** 里，**不要**加到 Settings → Secrets and variables → Actions 的仓库级 Repository secrets 里。

**也可以用命令行，并且更推荐。** 值直接从管道或隐藏输入进入 gh，不经过剪贴板，也不会进 shell 历史。Environment `release` 必须先在网页上建好（6.1）。

```sh
cd ~/landrop-secrets-tmp
R=lupguo/landrop

# 文件类：base64 结果直接通过管道传给 gh
base64 -i cert.p12          | gh secret set MACOS_SIGN_P12   --env release --repo $R
base64 -i AuthKey_XXXX.p8   | gh secret set MACOS_NOTARY_KEY --env release --repo $R

# 文本类：不带 --body 时，gh 会提示 "Paste your secret"，输入内容不会回显
gh secret set MACOS_SIGN_PASSWORD       --env release --repo $R
gh secret set MACOS_NOTARY_ISSUER_ID    --env release --repo $R
gh secret set MACOS_NOTARY_KEY_ID       --env release --repo $R
gh secret set HOMEBREW_TAP_GITHUB_TOKEN --env release --repo $R

# 确认名字齐全（只列名字，不显示值）
gh secret list --env release --repo $R
```

> ⚠️ 安全提醒
> - **不要**写成 `gh secret set NAME --body "真实密钥"`。那样明文会留在 `~/.zsh_history` 里，也可能被终端录屏或日志工具记下来。
> - secret 一旦保存，网页和 API 都无法再读出原值，只能覆盖。所以原件要在密码管理器里。
> - 5 个 `MACOS_*` secret 要么全配，要么全不配。只配了 `MACOS_SIGN_P12` 而缺少公证的三个值时，GoReleaser 会签名，但会跳过公证（日志显示 `will not try to notarize`），用户在 mac 上会被 Gatekeeper 拦下。
> - GitHub 会在 Actions 日志里把 secret 的原值打码成 `***`，但不会给它的变形（例如 base64 解码后的内容）打码。不要在工作流里加 `echo`、`cat` 或调试步骤去输出这些变量。

### 6.4 删除本地明文文件

确认 `gh secret list --env release` 列出了全部 6 个名字，并且原件都已经在密码管理器里，然后删除本地文件：

```sh
rm -rf ~/landrop-secrets-tmp
pbcopy < /dev/null
```

还要检查「下载」文件夹：浏览器下载的 `.cer` 和 `AuthKey_*.p8` 默认在那里，一并删除，并清空废纸篓。`.cer` 本身不含私钥，但没必要留着。

> ⚠️ 安全提醒
> 签名证书的私钥仍然在你 Mac 的登录钥匙串里，这是正常的，以后续期或重新导出都要用它。你的 Mac 本身要有登录密码，并开启 FileVault。

---

## 7. 建议开启的保护

### 7.1 给 v* tag 加规则集（tag ruleset）

GitHub 旧的「Tag protection rules」已经被**仓库规则集（Rulesets）**取代。作用是只有你能创建、移动或删除 `v*` tag，防止有写权限的协作者，或者泄露的其他凭据，推一个 tag 触发发版。

1. landrop 仓库 → **Settings** → 左侧栏 **Rules** → **Rulesets** → **New ruleset** → **New tag ruleset**。
2. **Ruleset name**：`release-tags`。
3. **Enforcement status**：选 **Active**。
4. **Bypass list**：点 **Add bypass**，加上 **Repository admin** 角色。你是仓库所有者，属于 admin，所以你自己可以绕过规则，正常推 tag 和删 tag。
5. **Target tags**：点 **Add target** → **Include by pattern** → 填 `v*`。
6. 规则勾选：**Restrict creations**、**Restrict updates**、**Restrict deletions**、**Block force pushes**。
7. 点 **Create**。

之后你推 tag 时，终端里可能出现一行提示，说你绕过了规则集（bypassed rule violations）。这是预期的，说明规则在生效，只是你有绕过权限。

### 7.2 开启账号两步验证

- **GitHub**：右上角头像 → **Settings** → **Password and authentication** → **Two-factor authentication**。推荐用通行密钥（passkey）或验证器 App，不要只用短信。
- **Apple 账号**：开发者账号本来就要求两步验证。确认你的受信任设备和电话号码是最新的。

> ⚠️ 安全提醒
> 能登录你 GitHub 账号的人，可以直接改 Environment secret、删掉规则集、推 tag 发版。账号本身是所有保护的根基。

### 7.3 其他

- 不要新增由 `pull_request` 或 `pull_request_target` 触发、又使用 `environment: release` 的工作流。Environment 只对 `v*` tag 开放，是这套安全设计的前提。
- `release.yml` 里的第三方 Action 都固定到了 commit SHA（例如 `actions/checkout@3d3c42e…`，后面注释版本号）。以后升级时，同样要写完整 SHA，不要改回 `@v7` 这种可以被移动的标签。

---

## 8. 第一次发版

先推 rc 演练，再推正式版。

### 8.0 发版前

- `release-distribution` 分支已经合并到 `main`，并且推到了 GitHub。安装脚本通过 `raw.githubusercontent.com/lupguo/landrop/main/...` 引用，所以必须在 main 上。
- 第 0 到 7 章都已完成。
- 在 main 的最新提交上打 tag：

  ```sh
  git switch main
  git pull --ff-only
  git log -1 --oneline    # 确认是你要发布的提交
  ```

### 8.1 rc 演练：v0.1.0-rc.1

这是**第一次真正跑签名和公证**。之前开发时没有密钥，只验证过「没有密钥时跳过签名」这条路径。

```sh
git tag v0.1.0-rc.1
git push origin v0.1.0-rc.1
```

rc 是预发布：Release 会标为 Pre-release，不会成为 latest，**也不会更新 tap**。所以 rc 演练是安全的，但它**测不到 tap 推送**。tap 推送和 `HOMEBREW_TAP_GITHUB_TOKEN` 是否可用，要到 8.3 的正式版才验证得到。

**看进度：** 仓库 → **Actions** → 左侧 **release** 工作流 → 点这次运行。如果开了 Required reviewers，先在页面上点批准。

**成功时的 Actions 日志：** 展开 goreleaser 那一步，应该能看到：

- 一个 `sign & notarize macOS binaries` 段落；
- 两个 darwin 二进制（`dist/darwin_darwin_amd64…/landrop` 和 `…arm64…/landrop`），各自依次出现 `signing` → `notarizing and waiting - this might take a while` → `notarized`；
- 公证通常要几分钟，配置的上限是 20 分钟；
- homebrew cask 那一段显示跳过（因为是预发布），这是正常的；
- 最后是 release 发布成功，整个 job 显示绿色对勾。

**不正常的信号：**

| 日志 | 含义 |
|---|---|
| `sign & notarize macOS binaries` 显示 skipped / `disabled` | `MACOS_SIGN_P12` 没读到。检查 secret 是否建在 **release** Environment 里、名字是否正确 |
| `will not try to notarize` | 签名了，但三个 `MACOS_NOTARY_*` 中有空的 |
| `notarize timeout` 或 `notarize still pending` | 公证没在 20 分钟内完成。**job 不会因此失败**，但二进制可能还没被 Apple 接受。见 9.3 |
| `rejected` / `invalid` / 401 / 403 | 见 9.3 |

**检查 Release：** 仓库 → **Releases** → `v0.1.0-rc.1`：

- 带有 **Pre-release** 标记，不是 Latest；
- 附件有 6 个：`landrop_darwin_amd64.tar.gz`、`landrop_darwin_arm64.tar.gz`、`landrop_linux_amd64.tar.gz`、`landrop_linux_arm64.tar.gz`、`landrop_windows_amd64.zip`、`checksums.txt`。GitHub 自动附带的 Source code 不算。
- 附件里不应该出现任何 `.p12`、`.p8` 或其他密钥文件。

**检查 tap 没有变化：** `https://github.com/lupguo/homebrew-tap` 里没有新提交，也没有出现 `Casks/landrop.rb`。

**在 Mac 上验证签名和公证：**

```sh
mkdir -p ~/landrop-verify && cd ~/landrop-verify
gh release download v0.1.0-rc.1 --repo lupguo/landrop -p 'landrop_darwin_arm64.tar.gz' -p 'checksums.txt'
grep ' landrop_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c -     # 期望：OK
tar -xzf landrop_darwin_arm64.tar.gz

codesign -dv --verbose=4 landrop
spctl -a -vvv -t install landrop
./landrop --version
```

Intel Mac 把 `arm64` 换成 `amd64`。

期望结果：

- `codesign -dv --verbose=4`：
  - `Authority=Developer ID Application: 你的名字 (TEAMID)`
  - `Authority=Developer ID Certification Authority`
  - `Authority=Apple Root CA`
  - 有 `Timestamp=` 一行，和正确的 `TeamIdentifier=`
- `spctl -a -vvv -t install`：
  - `landrop: accepted`
  - `source=Notarized Developer ID`
  - `origin=Developer ID Application: 你的名字 (TEAMID)`
- `--version` 输出 `v0.1.0-rc.1`。

关于联网校验：单个命令行二进制**无法 staple**（不能把公证票据钉在文件上），所以用户第一次运行时，macOS 会联网向 Apple 查询公证状态，这是正常的。离线环境下第一次运行可能被 Gatekeeper 拦下。`spctl` 检查同样需要联网。

**可选：用 rc 试 install.sh。** latest 还不存在（rc 不算 latest），所以要指定版本：

```sh
curl -fsSL https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh | LANDROP_VERSION=v0.1.0-rc.1 sh
```

rc 有任何问题：先按第 9 章修，再推 `v0.1.0-rc.2`，不要重用已经推过的 tag 名。rc 全部通过后，再进入 8.3。

### 8.2 关于 Windows 与 Linux

本次开发的验证情况：install.sh 只在 macOS 上实际跑过，Linux 上**没有实测**（当时没有 Docker）。install.ps1 只做过语法解析检查，在 macOS 的 pwsh 上做过部分冒烟测试，**没有在真实 Windows 上跑过**。所以 8.4 的首发清单必须包含真机测试。

### 8.3 正式版：v0.1.0

```sh
git switch main && git pull --ff-only
git tag v0.1.0
git push origin v0.1.0
```

和 rc 一样看 Actions 日志，签名和公证这一段应该和 8.1 一样。另外，这次 homebrew cask 那一段**不再跳过**，会推送到 tap。

### 8.4 首发清单（v0.1.0 之后逐项打勾）

**Release**

- [ ] `v0.1.0` 标为 **Latest**，没有 Pre-release 标记。
- [ ] 6 个附件齐全（5 个压缩包和 `checksums.txt`）。
- [ ] 按 8.1 的命令下载 `v0.1.0` 的 darwin 包，`codesign -dv --verbose=4` 和 `spctl -a -vvv -t install` 结果符合预期。

**Tap（rc 测不到，第一次在这里验证）**

- [ ] `lupguo/homebrew-tap` 出现一个新提交，里面有 `Casks/landrop.rb`。
- [ ] 打开 `Casks/landrop.rb` 检查：`version` 是 `0.1.0`；`url` 指向 `https://github.com/lupguo/landrop/releases/download/v0.1.0/landrop_darwin_….tar.gz`；两个 `sha256` 与 `checksums.txt` 里对应的 darwin 条目一致；**没有**任何调用 `xattr` 去掉隔离标记的 hook。

**macOS**

- [ ] `brew install lupguo/tap/landrop`。最好在一台干净的 Mac 上，或者用新建的 macOS 用户。
- [ ] `landrop --version` 输出 `v0.1.0`，第一次运行没有弹出 Gatekeeper 警告。
- [ ] `curl -fsSL https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh | sh` 输出 `Checksum OK.`，`landrop --version` 输出 `v0.1.0`。

**Linux（必须真机或真虚拟机，之前未实测）**

- [ ] 在一台 Linux amd64 机器上，以**普通用户**（非 root）运行：

  ```sh
  curl -fsSL https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh | sh
  ```

  期望：输出 `Checksum OK.`。如果 `/usr/local/bin` 不可写，会装到 `~/.local/bin` 并提示 PATH。不会要求 sudo。
- [ ] `landrop --version` 输出 `v0.1.0`。如果提示了 PATH，先按提示 `export PATH=...`。
- [ ] 再运行一次安装命令（模拟升级），仍然成功。
- [ ] 如果手边有 Linux arm64（例如树莓派 64 位），也跑一次。

**Windows（必须真机，之前只做过语法解析检查）**

- [ ] 在 Windows 10/11 x64 上打开 PowerShell（不需要管理员权限），运行：

  ```powershell
  irm https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.ps1 | iex
  ```

  期望：输出 `Checksum OK.`，`Installed to C:\Users\<你>\AppData\Local\Programs\landrop\landrop.exe`，然后打印版本号。
- [ ] **同一条命令再运行一次**（模拟升级），仍然成功。
- [ ] 确认用户级 PATH 里 landrop 目录**只出现一次**：

  ```powershell
  [Environment]::GetEnvironmentVariable('Path', 'User') -split ';' | Where-Object { $_ -like '*Programs\landrop*' }
  ```

  期望：正好输出 **一行**。输出两行就说明有重复，要报 bug。
- [ ] **新开**一个 PowerShell 窗口，运行 `landrop --version`，输出 `v0.1.0`。
- [ ] 如果有条件，在 Windows PowerShell 5.1 和 PowerShell 7 上各跑一次。

---

## 9. 常见失败及排查

### 9.1 推了 tag，Actions 没有运行

- tag 必须以 `v` 开头（触发条件是 `tags: ['v*']`）。
- 确认 tag 推到了 GitHub：`git ls-remote --tags origin | grep v0.1.0`。
- 仓库 → **Settings** → **Actions** → **General**：确认 Actions 没有被禁用。

### 9.2 job 卡住或无法进入 Environment

- 显示 Waiting：你开了 Required reviewers，去运行页面点批准。
- 报错说这个 tag 不允许部署到 `release`：检查 6.2 的规则，Ref type 必须是 **Tag**，规则是 `v*`。

### 9.3 签名或公证失败

- **Go 或 Web 测试失败：** 这是有意的设计，测试不过就不发布。先在本地修好，删掉这个 tag（9.5），再推一个新 tag。
- **`.p12` 解析失败、密码错误：** 通常是 `MACOS_SIGN_P12` 粘贴得不完整，或者被插入了换行，也可能 `MACOS_SIGN_PASSWORD` 不对。从密码管理器取出原件，按 6.3 的 `base64 -i cert.p12 | gh secret set ...` 重新设置。
- **401 / unauthorized：** Key ID 或 Issuer ID 不对，或者 Key 已被吊销，或者用了 Individual Key 而不是 Team Key。
- **403 或提示协议问题：** Apple 有新的开发者协议需要接受。登录 developer.apple.com 和 App Store Connect，按页面提示，由 Account Holder 接受协议。
- **`rejected` / `invalid`：** Apple 拒绝了公证。在你的 Mac 上查看原因（需要 Xcode Command Line Tools，并准备好 `.p8` 原件）：

  ```sh
  xcrun notarytool history --key AuthKey_XXXX.p8 --key-id <KEY_ID> --issuer <ISSUER_ID>
  xcrun notarytool log <submission-id> --key AuthKey_XXXX.p8 --key-id <KEY_ID> --issuer <ISSUER_ID>
  ```

  > ⚠️ 安全提醒
  > 这里要临时把 `.p8` 从密码管理器取出到本地。查完立即删除。`--key-id` 和 `--issuer` 不是机密，可以写在命令行上，但 `.p8` 文件本身是机密。

- **`notarize timeout` / `notarize still pending`：** job 不会失败，Release 也会照常发布，但这两个二进制当时还没被 Apple 接受。过一段时间用上面的 `notarytool history` 查看：
  - 变成 **Accepted**：用户第一次运行时会联网校验，通过即可，不需要重发。
  - 变成 **Invalid**：删除这个 Release 和 tag（9.5），修好后发新版本。

### 9.4 tap 推送失败（403 / 404）

只会发生在正式版上。

- **403：** Token 过期了；或者 Contents 只给了 Read；或者 Repository access 没选 `homebrew-tap`。按第 5 章重新生成 Token，再按 6.3 覆盖 `HOMEBREW_TAP_GITHUB_TOKEN`。
- **404：** `lupguo/homebrew-tap` 不存在、名字拼错，或者仓库是空的、没有默认分支（第 4 章要求勾选 README）。
- **补救：** 这时 GitHub Release 往往已经建好，只是 tap 没更新。修好 Token 后，到 Releases 页面**只删除这个 Release、保留 tag**，再到 Actions 里这次失败的运行页面点 **Re-run jobs**（重新运行），用同一个 tag 重新完整发布一次。

### 9.5 tag 推错了：删除并重发

```sh
# 1. 删除 GitHub Release（网页：Releases → 该版本 → 删除图标；或命令行：）
gh release delete v0.1.0 --repo lupguo/landrop --yes

# 2. 删除远端和本地 tag
git push --delete origin v0.1.0
git tag -d v0.1.0
```

`gh release delete --cleanup-tag` 可以把第 1、2 步的远端部分合成一步。开了 7.1 的规则集时，你作为 admin 可以绕过它删除 tag。

**重发：** 改好后，最好**用新版本号**（例如 `v0.1.1`），不要重新推同一个 tag。已经有人下载过、brew 缓存过、或者镜像站抓取过的旧 tag，再指向不同的内容会造成混乱，校验和对不上。只有确认没人用过时（例如刚推几分钟的 rc），才重用同一个名字。

### 9.6 撤回 tap 里的一次提交

如果某个正式版有问题，而 tap 已经指向它，就要撤回 tap 的那次提交：

```sh
git clone https://github.com/lupguo/homebrew-tap.git
cd homebrew-tap
git log --oneline -- Casks/landrop.rb     # 找到要撤回的那次提交
git revert <commit-sha>
git push origin HEAD
```

`git revert` 会新增一个反向提交，不改写历史。撤回后，`Casks/landrop.rb` 会回到上一个版本；如果这是第一个版本，文件会被删除。已经安装的用户执行 `brew update && brew upgrade landrop`，就会回到 tap 当前指向的版本。之后修好问题，发一个新的正式版，tap 会自动更新。

---

## 10. 安全风险与应急

### 10.1 全部安全提醒汇总

1. **公开仓库不可撤销**（第 0 章）：公开前用 gitleaks 扫描全部历史。历史里的密钥只能吊销，删文件或改写历史都不够。
2. **证书私钥只在你的 Mac 上**（2.1）：不要在公用电脑上生成 CSR。Mac 要有登录密码，并开启 FileVault。
3. **`.p12` 加密码就是你的签名身份**（2.3）：强随机密码；原件只放在密码管理器；不要放进 iCloud 同步目录或 git 仓库，不要用聊天工具发送。
4. **剪贴板会泄露**（2.4、3.2、第 5 章）：粘贴后执行 `pbcopy < /dev/null`；注意通用剪贴板和剪贴板历史工具；更推荐用 `gh secret set` 的管道写法。
5. **`.p8` 只能下载一次**（3.2）：角色选最低的 Developer，原件存进密码管理器。
6. **PAT 范围最小化**（第 5 章）：Only select repositories 只选 homebrew-tap，只给 Contents: Read and write，一定设过期时间，不用 classic token。
7. **secret 只放 Environment `release`**（6.3）：不放 repository secrets；不要用 `gh secret set --body "<secret>"`；5 个 MACOS_* 要全配；不要在工作流里输出 secret。
8. **用完删除本地明文**（6.4）：删除 `~/landrop-secrets-tmp`、「下载」里的 `.cer` 和 `.p8`，清空废纸篓和剪贴板。
9. **Environment 只对 `v*` tag 开放**（6.2、7.3）：不要新增 PR 触发、又使用 `release` Environment 的工作流。
10. **第三方 Action 固定到 commit SHA**（7.3）：升级时同样写完整 SHA。
11. **账号是根基**（7.2）：GitHub 和 Apple 账号都开两步验证；用 tag 规则集限制谁能推 `v*`。
12. **查公证日志时临时取出的 `.p8`**（9.3）：查完立即删除。

### 10.2 泄露后怎么办

**`.p12` 和密码泄露（最严重）**

别人可以用你的 Developer ID 签名恶意软件，用户会以为那是你发布的。

1. Developer ID 证书**不能**在 developer.apple.com 上自助吊销。Apple 当前文档要求发邮件到 **product-security@apple.com** 申请吊销。邮件里写明你的 Team ID、证书序列号（在钥匙串里双击证书可以看到），并说明私钥已泄露。
2. 你要清楚后果：**一旦吊销，所有用这张证书签名的旧版本都会无法安装或运行**，用户需要升级到用新证书签名的版本。
3. 所以顺序是：先按第 2 章创建一张**新**证书（一个账号最多同时有 5 张），导出新 `.p12`，用新的强密码；再按 6.3 覆盖 `MACOS_SIGN_P12` 和 `MACOS_SIGN_PASSWORD`；发一个新版本（例如 `v0.1.1`）；同时发邮件申请吊销旧证书。
4. 在 README 或 Release 说明里告知用户升级。

**`.p8` 泄露**

1. App Store Connect → **Users and Access** → **Integrations** → **Team Keys** → **Edit** → 勾选这把 Key → **Revoke Key** → 在弹窗里点 **Revoke**。吊销立即生效，并且不能恢复。
2. 按第 3 章生成一把新 Key，按 6.3 覆盖 `MACOS_NOTARY_KEY` 和 `MACOS_NOTARY_KEY_ID`。Issuer ID 一般不变。
3. 用 `xcrun notarytool history ...`（用新 Key）检查有没有不是你发起的公证提交。

**PAT 泄露**

1. GitHub 头像 → **Settings** → **Developer settings** → **Personal access tokens** → **Fine-grained tokens** → 找到 `landrop-homebrew-tap` → **Delete**。
2. 检查 `lupguo/homebrew-tap` 的提交历史：`git log -p -- Casks/`。确认每次提交都是 GoReleaser 在你发版时做的，`url` 和 `sha256` 都与对应 Release 的 `checksums.txt` 一致。发现可疑提交，就按 9.6 撤回。
3. 按第 5 章生成新 Token，按 6.3 覆盖 `HOMEBREW_TAP_GITHUB_TOKEN`。
4. 因为 Token 的范围只限 tap 仓库，影响只在 tap 里的安装配置，不涉及 landrop 源码和你的其他仓库。

**怀疑 GitHub 账号被入侵**

1. 立即修改密码，检查 **Settings → Password and authentication** 里的两步验证设置，以及 **Sessions**，登出不认识的会话。
2. 吊销所有 PAT 和 SSH key，然后重建。
3. 检查 landrop 仓库的 Environment secret、规则集和工作流文件有没有被改动，检查有没有陌生的 tag 或 Release。
4. 把 6 个 secret 当作全部可能泄露，按上面的步骤逐个处理。

### 10.3 定期维护

- **每年：** PAT 到期前续期（第 5 章），Developer ID 证书到期前创建新证书并更新 secret（第 2 章，按证书上的到期日）。两者都在日历里加提醒。
- **每次升级工作流中的 Action：** 固定到完整的 commit SHA，后面注释版本号。
- **增加协作者前：** 确认 7.1 的规则集是 Active，并且 Bypass list 里只有 Repository admin。
