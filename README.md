# smb-proxy

Go-based SMB proxy as a Docker image. Connects to a remote SMB server (credentials via environment variables) and exposes the share locally over SMB.

Source: [github.com/danielgtmn/smb-proxy](https://github.com/danielgtmn/smb-proxy)

## Quick start

Pull the image and run in gateway mode:

```bash
docker run -d \
  --name smb-proxy \
  --privileged \
  -p 1445:445 \
  -e SMB_HOST=192.168.1.100 \
  -e SMB_SHARE=daten \
  -e SMB_USER=backup \
  -e SMB_PASSWORD=geheim \
  -e LOCAL_SHARE=proxy \
  -e LOCAL_USER=proxy \
  -e LOCAL_PASSWORD=lokal \
  ghcr.io/danielgtmn/smb-proxy:latest
```

## Modes

| Mode | Description |
| --- | --- |
| `gateway` (default) | Authenticates with remote credentials, mounts the share, and exports it locally via Samba. Clients connect with `\\localhost\<LOCAL_SHARE>`. |
| `tcp` | Pure TCP forwarder from local port to remote SMB server. Authentication happens on the client side against the target server. |

## Environment variables

### General

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `SMB_PROXY_MODE` | no | `gateway` | `gateway` or `tcp` |
| `SMB_HOST` | yes | — | Hostname or IP of the remote SMB server |
| `SMB_HOST_IP` | no | — | Force a specific IPv4 address (useful when Docker prefers broken IPv6) |
| `SMB_PORT` | no | `445` | Remote port |
| `SMB_FORCE_IPV4` | no | `true` | Resolve and connect via IPv4 only |
| `SMB_DIAL_TIMEOUT` | no | `30s` | Timeout for remote SMB connections |
| `SMB_MOUNT_OPTIONS` | no | see below | Extra `mount.cifs` options (without `credentials=`) |
| `LOCAL_PORT` | no | `445` | Local SMB port inside the container |

Default `SMB_MOUNT_OPTIONS`: `iocharset=utf8,rw,seal,vers=3.0,uid=0,gid=0,file_mode=0664,dir_mode=0775,noserverino`

### Gateway mode

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `SMB_SHARE` | yes | — | Remote share name |
| `SMB_USER` | yes | — | Username for the remote server |
| `SMB_PASSWORD` | yes | — | Password for the remote server |
| `SMB_DOMAIN` | no | — | Windows domain |
| `LOCAL_SHARE` | no | `proxy` | Name of the locally exported share |
| `LOCAL_USER` | no | `proxy` | Local Samba user |
| `LOCAL_PASSWORD` | yes* | — | Password for local clients |
| `LOCAL_ALLOW_GUEST` | no | `false` | Allow guest access without a password |
| `MOUNT_PATH` | no | `/mnt/remote` | Mount path inside the container |

\* Not required when `LOCAL_ALLOW_GUEST=true`.

## Hetzner Storage Box

Official docs: [Zugriff mit SAMBA/CIFS](https://docs.hetzner.com/de/storage/storage-box/access/access-samba-cifs/)

1. Enable **SMB support** in Hetzner Console (Settings → activate SMB)
2. Wait a few minutes after activation before connecting
3. Use these values:

| Setting | Main account | Subaccount |
| --- | --- | --- |
| `SMB_HOST` | `u599718.your-storagebox.de` | `u599718-sub1.your-storagebox.de` |
| `SMB_SHARE` | `backup` | `u599718-sub1` (same as username) |
| `SMB_USER` | `u599718` | `u599718-sub1` |
| `SMB_PASSWORD` | Storage Box password | Subaccount password |

Remote UNC: `//u599718.your-storagebox.de/backup` — in `.env` only set `SMB_SHARE=backup`, not the full path.

Hetzner recommends the `seal` mount option for encrypted SMB connections; smb-proxy sets this by default.

**Notes from Hetzner:**

- SMB uses port **445** (SSH/SFTP uses port **23** — different protocol)
- FritzBox users may need to disable the NetBIOS filter
- For files **over 4 GB**, add `cache=none` to `SMB_MOUNT_OPTIONS`

Example `.env`:

```env
SMB_HOST=u599718.your-storagebox.de
SMB_SHARE=backup
SMB_USER=u599718
SMB_PASSWORD=your-password
SMB_FORCE_IPV4=true
LOCAL_SHARE=test
LOCAL_USER=proxy
LOCAL_PASSWORD=localpass
```

## Local test with docker compose

```bash
cp .env.example .env
# edit .env with your credentials

docker compose up --build
docker compose logs -f
```

Connect locally:

```bash
smbclient //127.0.0.1/test -p 1445 -U proxy
```

On macOS, map the share in Finder with `smb://127.0.0.1:1445/test`.

## Docker

### Build

```bash
docker build -t smb-proxy .
```

### Gateway (recommended)

```bash
docker run -d \
  --name smb-proxy \
  --privileged \
  -p 1445:445 \
  -e SMB_PROXY_MODE=gateway \
  -e SMB_HOST=192.168.1.100 \
  -e SMB_SHARE=daten \
  -e SMB_USER=backup \
  -e SMB_PASSWORD=geheim \
  -e LOCAL_SHARE=proxy \
  -e LOCAL_USER=proxy \
  -e LOCAL_PASSWORD=lokal \
  ghcr.io/danielgtmn/smb-proxy:latest
```

Connect:

- Windows/macOS: `\\localhost\proxy` (port 1445 may need to be mapped via `net use` / port forwarding)
- Linux: `smbclient //localhost/proxy -p 1445 -U proxy`

### TCP proxy

```bash
docker run -d \
  --name smb-proxy \
  -p 1445:445 \
  -e SMB_PROXY_MODE=tcp \
  -e SMB_HOST=192.168.1.100 \
  ghcr.io/danielgtmn/smb-proxy:latest
```

In TCP mode, clients authenticate directly against the remote server. Remote credentials from environment variables are not used.

## Release

Images are published to GHCR when a GitHub Release is published:

- `ghcr.io/danielgtmn/smb-proxy:<tag>`
- `ghcr.io/danielgtmn/smb-proxy:latest` (stable releases only)

### docker compose

```bash
cp docker-compose.yml docker-compose.local.yml
# Adjust values in docker-compose.local.yml
docker compose -f docker-compose.local.yml up -d
```

## Local development

```bash
go run ./cmd/smb-proxy
```

Gateway mode requires Linux with `mount.cifs` and `smbd` (typically root).

## Architecture

```mermaid
flowchart LR
  Client["Local client"] --> Samba["Samba in container"]
  Samba --> Mount["CIFS mount"]
  Mount --> Remote["Remote SMB server"]
  Go["Go smb-proxy"] --> Verify["go-smb2 connection test"]
  Go --> Samba
```

1. Go verifies the remote connection with `go-smb2`.
2. The remote share is mounted via `mount.cifs`.
3. Samba exports the mount as a local share.

## Notes

- The container requires `--privileged` or `CAP_SYS_ADMIN` for CIFS mounts.
- Port `445` is often in use on macOS; use e.g. `-p 1445:445`.
- For production: provide secrets via Docker Secrets or a vault, not in plain text in compose files.
