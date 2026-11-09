# LanDrop

[中文说明](#中文说明)

LanDrop is a single-binary tool for moving text and files between devices on the same LAN. Run it on one machine, open the address (or scan the QR code) from any other device, and paste text or drop files in a browser. No install, no accounts, no cloud. The web UI is embedded in the binary.

## Quick start

```sh
make all          # builds the web UI, then bin/landrop
./bin/landrop
```

The terminal prints the LAN addresses, a 4-digit access code and a QR code. Open the address on another device, enter the code, and paste text or drop files. A device that scans the QR code (or opens the `?t=` link) joins without typing the code.

Pre-built binaries for other platforms: `make dist` writes darwin/arm64, darwin/amd64, windows/amd64, linux/amd64 and linux/arm64 binaries to `dist/`.

## Flags

```
Usage:
  landrop [flags]           start the LanDrop server
  landrop send <file>|-     send a file (or stdin with -) to the running server

Flags:
  --dir string       directory to share read-only for browsing
  --keep             keep temporary files on exit
  --lang string      terminal language: en|zh|ja|ko (env LANDROP_LANG) (default "en")
  --max-size string  maximum file size, e.g. 512MB or 4GB (default "4GB")
  --name string      mDNS name, reachable as <name>.local (default "drop")
  --no-auth          disable the access code (anyone on the network can join)
  --no-open          do not open the browser on start
  --no-save          do not autosave received files to ~/Downloads/LanDrop
  --port int         listen port; tries the next one if busy, up to 10 times (default 8848)
  --ttl duration     how long items are kept (default 24h0m0s)
  --version          print the version and exit
  -h, --help         show this help
```

## Sending from the command line

With LanDrop running on this machine:

```sh
landrop send ./report.pdf          # send a file
cat app.log | landrop send -       # send stdin (text up to 1 MiB becomes a message, larger input a file)
```

## Running an unsigned binary

The release binaries are not code-signed.

- **macOS**: if Gatekeeper blocks it, remove the quarantine flag: `xattr -d com.apple.quarantine landrop`
- **Windows**: SmartScreen may warn. Click "More info", then "Run anyway".

## Network notes

- **Windows firewall**: allow inbound TCP on the port (run in an elevated prompt):
  `netsh advfirewall firewall add rule name="LanDrop" dir=in action=allow protocol=TCP localport=8848`
- **macOS Local Network permission**: on first run macOS may ask whether the terminal app may find devices on your local network. Allow it, or other devices (and `<name>.local`) cannot reach LanDrop. It can be changed in System Settings > Privacy & Security > Local Network.
- **AP / client isolation**: if other devices cannot connect, check that the router or guest Wi-Fi does not have "AP isolation" / "client isolation" enabled; it blocks device-to-device traffic.
- `drop.local` (mDNS) may not resolve on every network; use the IP address printed in the banner.

## Security notes

- LanDrop speaks plain HTTP on your LAN. Anyone who can sniff the network can see traffic. Use it only on networks you trust.
- Access is gated by a 4-digit code. Five consecutive wrong attempts lock that IP for 60 seconds, and more than 30 failures overall within a minute lock all code attempts for 60 seconds. Existing sessions and QR/token links are unaffected.
- Share links are one-time by default (1 download, 10 minutes); invalid or used links return HTTP 410.
- `--no-auth` disables the code entirely: anyone on the network can view, send and download. Use it only on networks you fully trust.
- Only the host machine (loopback) can remove other devices.

## Build from source

Requires Go 1.26 and Node 22.

```sh
make all     # web UI + bin/landrop
make test    # Go and web unit tests
make e2e     # Playwright smoke test (needs: cd web && npx playwright install chromium)
make dist    # cross-compile all release targets into dist/
```

---

## 中文说明

LanDrop 是一个单文件工具，用于在同一局域网内的设备之间传文本和文件。在一台机器上运行，在其他设备的浏览器里打开地址（或扫码），即可粘贴文本、拖入文件。无需安装、无账号、不经过云端；Web 界面已内嵌在二进制中。

### 快速开始

```sh
make all
./bin/landrop            # 加 --lang zh 可让终端输出中文
```

终端会显示局域网地址、4 位访问码和二维码。其他设备打开地址并输入访问码即可；扫码或打开带 `?t=` 的链接则无需输入访问码。

### 命令行发送

```sh
landrop send ./report.pdf
cat app.log | landrop send -
```

参数列表见上方 Flags（`landrop --help --lang zh` 可看中文说明）。

### 未签名二进制

- **macOS**：若被 Gatekeeper 拦截，执行 `xattr -d com.apple.quarantine landrop`。
- **Windows**：SmartScreen 提示时点击"更多信息"，再点"仍要运行"。

### 网络提示

- **Windows 防火墙**：以管理员身份执行 `netsh advfirewall firewall add rule name="LanDrop" dir=in action=allow protocol=TCP localport=8848`。
- **macOS 本地网络权限**：首次运行时请允许终端访问本地网络，否则其他设备无法连接。
- **AP / 客户端隔离**：如果其他设备连不上，请检查路由器或访客 Wi-Fi 是否开启了"AP 隔离"。

### 安全说明

- 局域网内使用明文 HTTP，请只在可信网络中使用。
- 访问码为 4 位数字；同一 IP 连续输错 5 次锁定 60 秒，全局 1 分钟内失败超过 30 次则暂停所有输码尝试 60 秒。
- 分享链接默认一次有效（1 次下载、10 分钟），失效返回 HTTP 410。
- `--no-auth` 会关闭访问码，网络内任何人都能查看和发送，请谨慎使用。

### 从源码构建

需要 Go 1.26 和 Node 22：`make all`；测试 `make test`；冒烟测试 `make e2e`；交叉编译 `make dist`。
