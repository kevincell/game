# Sour LAN deployment

Prepared from the upstream Sour v0.2.5 Linux release and source tag. This package is for the separate Linux server PC; it does not configure that machine's NIC or firewall from this development computer. The bundled container includes the prebuilt game engine and assets, plus the modified Go service and React web client. No Emscripten/game-engine build is run.

The browser WASM and its JavaScript loaders were extracted from the verified upstream release binary, not rebuilt; their hashes are in `client/prebuilt-game/SHA256SUMS`.

## Player URL

**`http://<SERVER_LAN_IP>:1337`**

Replace `<SERVER_LAN_IP>` with the address assigned to the server's wired LAN interface. Reserve that address with college IT or configure it statically on the server before inviting players. The development machine's `10.68.200.80` address is not the target server address.

## Player instructions

1. Open `http://<SERVER_LAN_IP>:1337` in a recent desktop browser.
2. Click **Create Private Lobby** and choose a map and mode.
3. Read the five-character code to friends.
4. Friends open the same URL and use **Join with Code**.
5. Click the game view and play; **Join Public FFA** enters the public arena.

## Install on the Linux server

Copy/extract this deployment directory to `/opt/sour-lan`. The release is for Linux `x86_64`/`amd64`; confirm the target with `uname -m` before loading it. The package requires Docker Engine and the Docker Compose v2 plugin on the server. No Node, Go, npm, or internet access is needed at runtime.

The local smoke script uses `curl`; install it from the target distro's package manager if it is missing.

```sh
uname -m                 # must report x86_64 for this image
docker --version
docker compose version
```

To rebuild the image on an internet-connected build machine after changing source or config:

```sh
docker compose -f docker-compose.build.yml build
docker save sour-lan:0.2.5 -o sour-lan-image.tar
```

These build commands require the full Sour source checkout (including `Dockerfile`, `docker-compose.build.yml`, `client/prebuilt-game`, and `release/assets`). The offline deployment archive contains only the ready-to-load image and runtime files.

From the extracted deployment directory:

```sh
sudo mkdir -p /opt/sour-lan
sudo cp sour-lan-image.tar /opt/sour-lan/
sudo cp docker-compose.yml DEPLOYMENT.md /opt/sour-lan/
sudo cp -r systemd scripts /opt/sour-lan/
sudo chown -R root:root /opt/sour-lan
cd /opt/sour-lan
sudo docker load -i sour-lan-image.tar
sudo install -m 0644 systemd/sour-lan.service /etc/systemd/system/sour-lan.service
sudo systemctl daemon-reload
sudo systemctl enable --now sour-lan.service
sudo systemctl status sour-lan.service --no-pager
curl -fsS http://127.0.0.1:1337/ >/dev/null && echo 'Sour web client responds'
sudo /opt/sour-lan/scripts/lan-smoke.sh
```

The service is `sour-lan.service`. Admin commands:

```sh
sudo systemctl start sour-lan
sudo systemctl stop sour-lan
sudo systemctl restart sour-lan
sudo systemctl status sour-lan --no-pager
sudo docker logs -f sour-lan
```

The Compose file uses host networking and a restart policy. The system service runs Compose in the foreground with `Restart=on-failure`; Docker must start on boot. Docker's named volume `sour-lan_sour-cache` persists the asset cache across container restarts.

## Config and useful settings

The game config is baked into the image at `/opt/sour/config.yaml`. To change it, edit the repository `config.yaml`, rebuild the image using the build instructions in the accompanying source package, then transfer and load the new image. Main knobs:

- Map rotation: `server.presets[].config.maps` (`complex`, `dust2`, `turbine`).
- Default mode/map: `defaultMode` and `defaultMap`; current public default is FFA on `complex`.
- Capacity: `maxClients` (currently 16 per server instance).

Web binds to `0.0.0.0:1337`; the browser game uses WebSocket on the same port. The optional desktop client ingress uses UDP 28785. Master registration and server discovery are disabled. There are public `lobby` and `insta` spaces and private games created through the browser UI.

## Network and firewall

Use a static/reserved address on the interface actually connected to the player LAN. Discover the interface and address on the server with:

```sh
ip -4 -br addr
ip route
```

Ask college IT to reserve the address if the LAN is managed. On a dedicated switch with no DHCP server, set the server to `192.168.50.1/24` and assign each player a unique address from `192.168.50.10/24` through `192.168.50.25/24` in their network settings; leave gateway and DNS blank. Players then use `http://192.168.50.1:1337`. These are example addresses for a dedicated isolated subnet only. Do not use them on a campus-managed shared subnet. Alternatively, have the admin configure DHCP on the isolated game LAN interface. A DHCP server is not installed automatically because the target interface and subnet are unknown and an accidental second DHCP server can disrupt the LAN.

Allow inbound TCP 1337 and TCP/UDP 28785 on the server firewall, as requested for web and desktop clients.

For Ubuntu with UFW:

```sh
sudo ufw allow 1337/tcp
sudo ufw allow 28785/tcp
sudo ufw allow 28785/udp
sudo ufw status
```

For firewalld:

```sh
sudo firewall-cmd --permanent --add-port=1337/tcp
sudo firewall-cmd --permanent --add-port=28785/tcp
sudo firewall-cmd --permanent --add-port=28785/udp
sudo firewall-cmd --reload
```

The app config has no required external service. Sour's master-server registration is disabled; client assets and engine files come from the bundled release and are served locally. The optional Discord avatar CDN fetch was removed from the web client. Verification still required on the actual server with its WAN disconnected.

## Custom lobby changes

- `pkg/server/servers/module.go`: private game identifiers are cryptographically generated, collision-reserved five-character codes from an alphabet excluding ambiguous characters; empty private servers expire after 30 seconds.
- `pkg/server/servers/gameserver.go`: tracks whether a game instance is private.
- `pkg/server/service/commands.go`: tags created games as private and marks the creator connection as owning the lobby so the UI receives and displays its code.
- `pkg/server/service/users.go`: updates idle tracking when connections leave or drop.
- `client/src/index.tsx`: adds Create Private Lobby, map/mode selection, displayed code, copyable share link, Join with Code, and Join Public FFA.
- `client/src/discord.ts`: removes the external avatar image request.
- `pkg/server/servers/lobby_code_test.go`: checks generated code shape and uniqueness.
- `pkg/server/service/cs.go`: fixes two upstream `go vet` format mismatches in timestamp-based auth-key generation so the Go test gate runs cleanly.

Codes expire shortly after the last player leaves. A public FFA lobby remains available without a code.

## If it breaks

- Packaging initially exposed missing Docker build dependencies (`swig`, zlib headers, and ENet); they are now installed only in the builder stage.
- Sour's Parcel command emits the React bundle but not the HTML shell or Emscripten files. The image now includes the shell/background and verified prebuilt engine JS/WASM; `scripts/lan-smoke.sh` checks all of these paths.
- `docker compose` cannot find the image: run `sudo docker load -i sour-lan-image.tar` from `/opt/sour-lan`.
- Browser cannot open the page: confirm the server LAN IP with `ip -4 -br addr`, confirm `sudo systemctl status sour-lan`, then inspect `sudo docker logs sour-lan` and firewall rules for TCP 1337.
- Page opens but game traffic fails: confirm TCP 1337 is permitted (WebSocket uses it) and check that the browser supports WebGL/WebAssembly.
- No players get addresses on an isolated switch: use static addresses on the same subnet or set up DHCP on that dedicated LAN interface only.
- Private code fails: create a new lobby and share its current five-character code; empty private lobbies expire after 30 seconds.
- If the target is not `x86_64`, do not load this image; rebuild for the architecture reported by the server's `uname -m`.

## Verification state

Local Go tests pass for the server/lobby packages. The Docker container starts on loopback; HTTP 200 checks pass for the HTML shell, React bundle, background, engine loaders, and WASM. Headless Chromium starts the Sauerbraten engine with external DNS blocked, creates private code `4EZUX`, and a second browser tab connects to server `4EZUX` (confirmed in server logs). The actual server IP/interface, its firewall, a second physical player machine joining, and a physical WAN-disconnected test remain pending until run on the target server and player LAN.
