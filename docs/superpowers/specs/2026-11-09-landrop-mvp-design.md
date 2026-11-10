# LanDrop v0.1 (M1 MVP) 设计文档

- 日期：2026-11-09
- 来源需求：`LandropPRD.md`（下称 PRD），本文是在其基础上的实现设计，二者冲突时以本文为准
- 状态：已完成评审，待用户确认

标记约定：**[用户定]** = 用户在讨论中明确决定；**[我的判断]** = 实现方自行决定，用户可推翻；未标记的条目直接来自 PRD。

---

## 1. 意图与动机

用户日常在 Mac 与 Windows 之间传**小文件、错误日志、代码片段**，目前没有顺手的方式（IM 绕公网且混在聊天里，SMB 太重）。LanDrop 让一端跑一个二进制，另一端只用浏览器就能收发。

典型路径（[我的判断]，用户未否认）：Mac 作为 Host 运行 LanDrop，Windows 用浏览器打开，把日志拖进去；Mac 上实时出现并下载/自动落到 `~/Downloads/LanDrop/`。

## 2. 目标 / 非目标 / 反目标

### 2.1 目标

- **范围 [用户定]**：PRD 中 **全部 P0 必须完成；P1 尽量完成**。M2 中属于 P1 的项（mDNS、代码高亮、图片预览、CLI send、自动保存）一并纳入，但优先级低于所有 P0。
- PRD §3.1 G1–G4 与 §9 非功能需求全部适用（单二进制 ≤15MB、无 CGO、5 平台交叉编译、≥10 Peer、闪断 3 秒内重连补齐等）。

### 2.2 非目标

PRD §3.2 全部；PRD §15 的"多 Host 互联"（用户确认留到后续版本）；以及所有 P2 项：F-14 新设备确认、F-34 断点续传、F-35 文件夹上传、F-54 阅后即焚、`--tls`。

### 2.3 反目标（即使功能"能用"也算失败）[我的判断，基于用户痛点推断]

- Windows 浏览器上"复制"按钮在 HTTP 下静默失效。
- 每次打开都要重新输入访问码（同一次 Host 运行内应一直免登）。
- 传几百 MB 文件时 Host 内存随文件大小上涨或页面卡死。
- 消息/日志刷屏导致页面无法浏览（见 §5 折叠与滚动）。

## 3. 技术选型

| 层 | 选择 | 标记 |
|---|---|---|
| 服务端 | Go 1.26（本机已装），标准库 `net/http`（使用 1.22+ 路由模式 `GET /api/items/{id}`） | PRD |
| 前端 | **Preact + Vite + TypeScript**（含自研轻量 i18n，见 §3.2），`vite build` 产物经 `//go:embed` 打入二进制 | [用户定] 采纳 Preact+Vite；TypeScript 为 [我的判断] |
| 实时通道 | SSE，上行用普通 POST | PRD |
| mDNS | `github.com/grandcat/zeroconf`（`RegisterProxy` 可宣告自定义主机名 + IP） | [我的判断] |
| 二维码 | Go 端 `github.com/skip2/go-qrcode` 生成矩阵；终端用半块字符渲染 ASCII；前端二维码由后端返回 SVG（`GET /api/qr?data=...`），前端不再引 QR 库 | [我的判断] |
| 代码高亮 | `highlight.js` core + 少量常用语言（bash/shell、go、js/ts、json、yaml、python、sql、log 无高亮） | [我的判断] |
| 构建 | `Makefile`：`make web`（npm ci + vite build）→ `make build`（go build）→ `make dist`（5 平台交叉编译，`CGO_ENABLED=0`），`--version` 经 ldflags 注入 | [我的判断] |

依赖原则：Go 侧第三方依赖仅限 mDNS 与 QR 两个；如需磁盘剩余空间检测，使用 `golang.org/x/sys`。

### 3.1 分发 [用户定]

本版本**只供本地自用**：在本机 `make` 构建，交叉编译出的产物手动拷到其他机器。**不做**代码签名、公证、GitHub Release 或 CI 发布流水线，这些留到决定公开分发时再做（用户有 Apple 开发者账号）。README 里写明 macOS 下被 Gatekeeper 拦截时的解除办法（`xattr -d com.apple.quarantine landrop`），以及 Windows SmartScreen 的处理方法。

### 3.2 多语言 [用户定：支持中 / 英 / 日 / 韩，默认英文]

- **网页界面**：所有界面文案都通过 i18n 字典取值（`web/src/i18n/{en,zh,ja,ko}.ts`），不允许硬编码。`en` 是基准语言：其他语言缺少某个 key 时回退到英文，单测会检查四种语言的 key 是否齐全。不引入 i18n 库，自己写一个轻量的 `t(key, params)`，以控制体积 [我的判断]。
- **默认语言与切换 [我的判断]**：默认显示英文。用户可在设置面板切换语言，选择存进 localStorage。不按浏览器语言自动切换，以符合"默认英文"的要求。
- **终端输出**：启动横幅、提示和错误信息同样走字典（Go 侧 `internal/i18n`，四种语言用 map 实现）。默认英文，可用 `--lang zh|en|ja|ko` 或环境变量 `LANDROP_LANG` 切换 [我的判断]。
- **无需登录的页面**（输码页、分享落地页、已失效页）同样支持多语言：优先使用 localStorage 中的选择，没有则显示英文，页面右上角可以切换语言。
- 日文和韩文文案由实现方翻译初稿，用户之后可以修订。

## 4. 架构

沿用 PRD §10.2。单进程内模块（[我的判断]，供参考的包划分）：

```
cmd/landrop/          main：参数解析、子命令 serve(默认) / send
internal/config       命令行参数与默认值
internal/store        Item 内存索引 + 临时目录文件；seq 单调递增；TTL 清理
internal/hub          SSE 广播、在线 Peer 管理、事件环形缓冲（用于 since 补齐）
internal/auth         访问码、会话、token 链接、失败限流
internal/share        分享链接（次数/过期）
internal/server       HTTP 路由、中间件（鉴权、Origin 校验、安全头）
internal/discovery    网卡枚举与推荐、mDNS 宣告
internal/termui       启动横幅、ASCII 二维码、环境提示
internal/dirbrowse    --dir 只读目录浏览
web/                  Preact 前端源码；web/dist 由 embed 引入
```

## 5. 页面与交互 [用户定：方案 B 柔和卡片风 + 两条补充]

参考 mockup：`.superpowers/brainstorm/*/content/style-b-v2.html`。

- **视觉**：亮色主调、圆角卡片、留白；强调色紫（#5b5bd6）；每条消息用彩色圆形头像 + 设备名区分来源。亮/暗主题跟随系统 `prefers-color-scheme`，设置中可手动切换。
- **布局**：整页固定高度（100dvh），顶栏 + 输入区固定，**消息区占剩余高度并独立滚动**；桌面端一屏约显示 **5 条**消息 [用户定]，按时间倒序。
- **输入框自动伸缩 [用户定]**：默认 2 行；随内容增高；上限约 8 行（180px）[我的判断：阈值]，超过后框内出现滚动条。右下角显示字数。
- **消息展开/收起 [用户定]**：文本超过 4 行 [我的判断：阈值] 默认折叠，底部渐隐，显示"展开全部（N 行）"，展开后显示"收起"。复制按钮总是复制全文。
- **文件条目**：类型图标、文件名、大小、下载/分享/删除；上传中显示进度条与实时速度；图片显示缩略图（F-33）。
- **顶栏**：Logo、访问地址、在线台数（点击展开在线设备列表）、设置。
- **断线**：顶部红条"连接已断开，正在重连…"，重连成功自动消失。
- **Host 页面与权限 [我的判断]**：与 Peer 同一个 SPA。
  - **Host 身份**：请求来源 IP 为 loopback（127.0.0.1 / ::1）即视为 Host。启动时自动打开的本机浏览器使用 `http://127.0.0.1:<port>/?t=...`，因此 Host 一定有该身份。`/api/me` 返回 `{role: "host"|"peer"}` 供前端判断。
  - **对外 URL 不依赖请求 Host 头**：分享链接、二维码、页面上展示的"访问地址"一律用服务端计算出的**推荐局域网地址**（有 mDNS 时另附 `.local` 地址）生成，所以 Host 用 localhost 访问时生成的分享链接对同事依然有效。
  - **权限矩阵**：所有已授权设备都可以发送、下载、分享、删除任意条目、清空全部（PRD 定义为单房间互信场景），也都能看到在线设备列表（设备名 / IP / UA）。**只有 Host 能踢人**。`--dir` 目录对所有已授权设备可见。
- **设备名 [我的判断]**：浏览器拿不到主机名，默认从 UA 推断（如 "Windows · Chrome"、"iPhone · Safari"），设置里可改名，存 localStorage，并随请求上报。Host 本机默认名为系统 hostname。

## 6. 功能设计要点

### 6.1 启动（F-01~F-06、F-15）

- 默认 `0.0.0.0:9958`，占用则递增，最多 10 次，失败报错并提示 `--port`。
- 参数：`--port` `--name`（mDNS 名，默认 `drop`）`--dir` `--no-auth` `--ttl`（默认 24h）`--keep` `--max-size`（默认 4GB）`--no-save`（关闭自动保存，[我的判断] 新增）`--no-open`（[我的判断] 新增）`--version`。
- 终端打印：所有可用地址（排除 loopback、docker/veth/utun/vboxnet/vmnet 等虚拟网卡）并标注推荐地址（优先私有网段、有默认路由的网卡）、`http://<name>.local:<port>`、4 位访问码、带 token 的链接及其 ASCII 二维码；以及 AP 隔离提示、Windows 防火墙 `netsh` 放行命令（仅 Windows）、macOS 本地网络权限提示。
- 启动后用系统命令打开本机浏览器（open / xdg-open / rundll32），URL 带 token 使 Host 免输入码。
- `--no-auth`：终端醒目黄色警告，页面顶部常驻提示。

### 6.2 访问控制（F-10~F-13、PRD §11）

- 访问码 [用户定]：不设账户和登录流程，每次启动随机生成 **4 位数字码**（crypto/rand，允许前导 0）；任何设备输入此码即进入会话。另生成 128-bit 随机 **token** 用于链接/二维码 `?t=`。
- 进入方式：带有效 `?t=` 访问 → 签发会话 Cookie 并 302 去掉查询参数；否则显示输入码页 → `POST /api/auth`。
- 会话 Cookie：随机 ID，`HttpOnly; SameSite=Strict; Path=/; Max-Age=30 天` [我的判断]。设置较长的 Max-Age 是为了让关掉浏览器（尤其是 iOS Safari）后不必重新输码。会话本身只保存在 Host 进程内存中，进程重启后全部失效，旧 Cookie 自然作废，这样就满足 F-12 "与 Host 运行周期一致"。
- Origin 校验：写接口要求 `Origin` 的 host:port 与请求的 `Host` 头一致（因此 localhost 和局域网 IP 两种访问方式都能通过）。
- 失败限流：每 IP 连续失败 5 次锁 60 秒；另加全局限流 [我的判断，因 4 位码仅 1 万种组合]：任意 60 秒窗口内全局失败超过 30 次，则所有输码尝试暂停 60 秒（已登录会话与 token 链接不受影响）。
- 踢出：吊销该会话并断开其 SSE；被踢设备仍可凭访问码重新进入 [我的判断，P2 的新设备确认不做]。

### 6.3 消息流与实时（F-20~F-26、F-50~F-52）

- Item 结构沿用 PRD §10.3；`seq` 全局递增。
- SSE `/events`：事件 `item.created` / `item.deleted` / `items.cleared` / `peer.joined` / `peer.left`；每条带 `id: <seq>`。重连时浏览器自动带 `Last-Event-ID`，前端同时调用 `GET /api/items?since=<seq>` 补齐；15 秒心跳注释防代理断连。前端指数退避（0.5s 起，上限 3s 以满足"3 秒内重连"）。
- 文本：上限 1MB，超出前端提示"请作为文件发送"并提供一键转为 .txt 文件上传。
- 代码识别：启发式（多行且含缩进/括号/分号/常见关键字或 shell 提示符）→ 等宽 + highlight.js 自动语言检测；否则按普通文本（仍保留换行）。URL 自动链接（`rel="noopener noreferrer" target="_blank"`）。所有文本一律转义渲染，禁止 `innerHTML` 拼接用户内容。
- 全局粘贴：页面聚焦且焦点不在输入框时，粘贴文本直接发送、粘贴图片/文件直接上传。
- 复制：优先 `navigator.clipboard.writeText`，不可用（非安全上下文）时用隐藏 textarea + `document.execCommand('copy')` 兜底；成功/失败均 toast。**需在 Win Chrome/Edge、Mac Safari/Chrome、Firefox 上实测**。
- TTL：默认 24h，后台每分钟清理过期条目及其文件，并广播 `item.deleted`。

### 6.4 文件（F-30~F-33、F-36、F-62）

- 上传：`POST /api/upload`，每个文件一个请求（便于逐个进度与失败）；前端用 `XMLHttpRequest` 以获得 `upload.onprogress`，计算速度。多文件并发上限 3 [我的判断]。
- 服务端使用 `r.MultipartReader()` 流式写入临时文件，超限立即中止（`http.MaxBytesReader`）；上传前依据 `Content-Length` 检查磁盘剩余空间，不足返回 507 并提示。
- 存储：`os.TempDir()/landrop-<pid>/<item-id>`，绝不用用户文件名拼路径；原始文件名只作为元数据，下载时经 `Content-Disposition: attachment; filename*=UTF-8''...`。
- 下载：`http.ServeContent` 流式（自然支持 Range）；统一 `X-Content-Type-Options: nosniff`。图片预览接口 `GET /api/items/{id}/raw?inline=1` 仅对 `image/png|jpeg|gif|webp` 允许 inline，SVG 一律 attachment。
- 自动保存 F-62 [我的判断]：默认开启，上传完成后复制到 `~/Downloads/LanDrop/`（重名追加 ` (1)`）。该目录是持久副本，**不受 F-53 退出清理影响**；`--no-save` 关闭。
- 退出（SIGINT/SIGTERM）：停止监听、撤销 mDNS、删除临时目录（`--keep` 时保留并打印路径）。

### 6.5 分享链接（F-40~F-43）

- `POST /api/items/{id}/share {max_downloads=1, ttl_seconds=600}` → 128-bit 随机 ID。
- `GET /s/{id}`：无需登录的落地页，显示文件名/大小 + "下载"按钮，或文本内容 + "复制"按钮。**计次发生在 `GET /s/{id}/raw` 实际下载时**（文本则在落地页渲染时），以避免 IM 链接预览爬虫提前消耗 [我的判断]。
- 失效（次数用尽/过期/原条目已删除）→ 友好"已失效"页，HTTP 410。
- 生成后弹窗展示链接 + 复制 + 二维码（F-43）。

### 6.6 `--dir` 只读浏览（F-60）

- `GET /api/dir?path=` 列目录、`GET /api/dir/raw?path=` 下载。路径用 `filepath.Clean` + 拼接根后 `filepath.EvalSymlinks`，再校验仍在根目录内，否则 403。不提供写操作。前端为单独的"目录"标签页。

### 6.7 CLI 发送（F-61）[实现方式为我的判断]

- `landrop send <file>` / `landrop send -`（读 stdin 作为文本，超 1MB 自动转文件）。
- 运行中的实例启动时写 `~/.landrop/instance.json`（权限 0600，含端口与本地 secret），退出时删除。`send` 读取它，向 `http://127.0.0.1:<port>/api/local/...` 发请求并以 secret 鉴权；该接口只接受来自 loopback 的连接。找不到实例则报错"LanDrop 未在运行"。

## 7. 安全

全部沿用 PRD §11，补充实现细节见 §6.2/§6.4/§6.6。另：所有响应加 `Referrer-Policy: no-referrer`；前端页面加 CSP `default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'`。

## 8. 异常兜底

沿用 PRD §12。mDNS 注册失败只打印一行灰色提示，不影响启动。

## 9. 测试与验收

- **Go 单测**：store（seq、删除、清空、TTL）、auth（码校验、限流、token 换会话）、share（次数、过期、删除联动）、dirbrowse（路径穿越含 symlink 用例）、Origin 校验。
- **Go 集成测**（`httptest`）：鉴权全流程、文本发送→SSE 收到、`since` 补齐、大文件上传下载（生成 1GB 稀疏/流式数据，断言内存不线性增长）、分享链接 1 次后 410。
- **前端**：关键纯函数（代码识别、设备名推断、文件大小格式化）用 Vitest；交互用一组 Playwright 冒烟用例（登录、发文本、复制、上传、展开/收起）[我的判断]。
- **手工验收清单**：PRD §7 三条流程在 Mac(Host) + Windows Chrome/Edge 实测；手机扫码进入；复制兜底在各浏览器实测；二进制体积 ≤15MB；空闲内存 ≤30MB。

## 10. 留给实现方的事项

- 包结构、具体 Go 类型设计、前端组件拆分与状态管理（Preact signals 或 hooks 均可）。
- 网卡"推荐地址"的具体打分规则。
- 代码识别启发式的细节阈值。
- `web/dist` 是否入库（若不入库，需保证 `go build` 在缺少 dist 时给出清晰错误或占位页）。
- 设置面板的具体内容（至少：设备名、主题、自动保存提示）。

## 11. 开放问题处理

| PRD 编号 | 处理 |
|---|---|
| Q1 端口 | 9958 [用户定 2026-11-10] |
| Q2 文本落盘 | 不落盘，保持临时定位 |
| Q3 HTTPS/剪贴板 | 不内置 HTTPS；`execCommand` 兜底并实测（§6.3） |
| Q4 / Q5 命名、开源 | 不影响实现，暂不处理 |
