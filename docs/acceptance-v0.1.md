# LanDrop v0.1 acceptance checklist

Spec section 9, manual acceptance. "Verified" rows were run on the Mac build host (macOS, arm64) on 2026-11-10. Everything that needs Windows, a phone or a second machine is pending.

| Item | How to check | Result |
|---|---|---|
| Automated smoke (login, text across two browsers, copy, collapse, 2MB upload, one-time share) | `make e2e` | PASS: 5/5 (Playwright, Chromium, against a real server) |
| Binary size <= 15MB, all targets | `make all dist && ls -l dist/` | PASS: darwin-arm64 7.55MB, darwin-amd64 8.08MB, linux-amd64 7.95MB, linux-arm64 7.44MB, windows-amd64 8.21MB |
| Idle RSS <= 30MB | start `bin/landrop --no-open` with temp HOME, wait 5s, `ps -o rss= -p <pid>` | PASS: 14800 KB |
| Unit and race tests | `go vet ./... && go test -race -short ./... && (cd web && npm test)` | PASS |
| mDNS name resolves on the host | run with `--name ldtest`, then `dscacheutil -q host -a name ldtest.local` and `ping -c1 ldtest.local` | PASS: resolves to the LAN IP, ping replies (self-resolution on the Mac only) |
| PRD 7.1: Mac host sends code to Windows Chrome/Edge | Open banner address from Windows, enter code, paste text on Mac, copy on Windows | pending (needs user) |
| PRD 7.2: Windows Chrome/Edge sends a file to Mac | Drop a file on Windows, check it appears and is autosaved in `~/Downloads/LanDrop/` | pending (needs user) |
| PRD 7.3: one-time share to a colleague | Share an item, open the link on another machine, download once, reload shows "expired" | pending (needs user; same flow passes in `make e2e` on one machine) |
| Phone QR join | Scan the banner QR with a phone; it joins without entering the code | pending (needs user) |
| Copy fallback on Windows Chrome | Open `http://<LAN IP>:8848`, click Copy, paste elsewhere | pending (needs user) |
| Copy fallback on Windows Edge | same | pending (needs user) |
| Copy fallback on Mac Safari | same | pending (needs user) |
| Copy fallback on Mac Chrome | same | pending (needs user) |
| Copy fallback on Firefox over `http://<LAN IP>` | same | pending (needs user) |
| 500MB transfer >= 80MB/s on gigabit | Upload and download a 500MB file between two wired machines, time it | pending (needs user) |
| `drop.local` resolves from another Mac | `ping drop.local` from a second Mac on the LAN | pending (needs user) |
| `drop.local` resolves from iPhone | Open `http://drop.local:8848` in Safari on iPhone | pending (needs user) |
| Windows firewall prompt / netsh rule | Run the .exe on Windows, check the prompt or apply the `netsh` rule from the README, connect from another device | pending (needs user) |
