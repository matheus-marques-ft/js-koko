
# KoKo

**English** · [简体中文](./README_zh-CN.md)

KoKo is a connector of JumpServer for secure connections using character protocols, supporting SSH, Telnet, Kubernetes, SFTP and database protocols

Koko is implemented using Golang and Vue, and the name comes from a Dota hero [Kunkka](https://www.dota2.com.cn/hero/kunkka)。

## Features


- SSH
- SFTP
- Web Terminal
- Web File Management


## Installation

1. Clone the project

```shell
git clone https://github.com/matheus-marques-ft/js-koko.git
```

2. Build the application

Build the application in the koko project.
```shell
make
```
> If the build is successful, the build folder will be automatically generated under the project, which contains compressed packages of various architectures of the current branch.

## Usage (for Linux amd64 server)

1. Copy the compressed package file to the corresponding server

```
Build the default compressed package through make, the file name is as follows:
koko-[branch name]-[commit]-linux-amd64.tar.gz
```

2. Unzip the compiled compressed package
```shell
tar xzvf koko-[branch name]-[commit]-linux-amd64.tar.gz
```

3. Create the file `config.yml`, refer to [config_example.yml](https://github.com/matheus-marques-ft/js-koko/blob/main/config_example.yml)
```shell
touch config.yml
```

4. run koko
```shell
cd koko-[branch name]-[commit]-linux-amd64

./koko
```


## Setup development environment

1. Run the backend server

```shell

$ cp config_example.yml config.yml # 1. Prepare the configuration file
$ vim config.yml  # 2. Modify the configuration file, edit the address and bootstrap key
CORE_HOST: http://127.0.0.1:8080
BOOTSTRAP_TOKEN: PleaseChangeMe <change to the same as core>

$ go run ./cmd/koko/ # 3. Run, running requires go if not, download and install from go.dev
```


2. Run the ui frontend

```shell
$ cd ui 
$ yarn install
$ npm run serve
```

## Docker
To build multi-platform images using Docker Buildx, you need to install Docker version 19.03 or higher and enable the Docker Buildx plugin.

```shell
make docker
```

## Acknowledgments
This project depends on [usql](https://github.com/xo/usql) for database connections. We appreciate their support.

## Repository Layout

This repo builds the `koko` image consumed by [js-installer](https://github.com/matheus-marques-ft/js-installer)'s `compose/koko.yml`. Koko is JumpServer's character-protocol connector — it's what a user's SSH/Telnet/K8s/database client or the browser-based Web Terminal (served by [js-luna](https://github.com/matheus-marques-ft/js-luna)) actually connects through to reach an asset.

- **`pkg/sshd/`** — the native SSH server (`sshd`): dispatches an inbound SSH connection to the right backend protocol handler.
- **`pkg/httpd/`** — the HTTP/WebSocket server behind the Web Terminal, Web SFTP volume, and the AI chat panel.
- **`pkg/handler/`** — interactive session/menu logic shared by both `sshd` and `httpd` entrypoints (asset selection, banners, login confirmation, direct-connect).
- **`pkg/proxy/`** — the actual protocol proxy: `server.go`'s `getServerConn()` dispatches by protocol (SSH, K8s, database, RDP-gateway) to the matching connection type in `pkg/srvconn/`, then `switch.go` pipes data between client and backend while `recorder.go`/`command_check.go` handle session recording and command-filter ACL enforcement.
- **`pkg/srvconn/`** — per-protocol backend connections: `conn_ssh.go`, `conn_k8s*.go` (spawns a local `kubectl`/`unshare` PTY), `conn_telnet.go`, `conn_mongodb.go`, `conn_redis.go`, `conn_usql.go` (generic SQL via the vendored `usql`), SFTP variants.
- **`pkg/exchange/`** — pub/sub session-sharing backbone (in-memory or Redis-backed), used for session monitoring/joining.
- **`ui/`** — the Vue 3 + TypeScript frontend for the Web Terminal/Web SFTP/Kubernetes tree UI, built separately and embedded into the Go binary via `assets.go`.
- **`locale/`** — gettext `.po`/`.mo` catalogs (`zh`, `ja`, `zh_Hant`) for CLI/server-side i18n; `ui/src/locales/modules/*.json` are the separate frontend i18n catalogs.
- **`static/plugins/elfinder/`** — vendored third-party file-manager library (elfinder.org), not part of this fork's own code.
- **`Dockerfile-base`** / **`Dockerfile`** — two-stage build: `Dockerfile-base` compiles Go dependencies and vendors external CLI tools (`k8s-bundle`, `healthcheck`, `usql`) into the `koko-base` image; `Dockerfile` builds the actual binary + embedded UI on top of that base.

### CI → GHCR mapping

| Workflow | Publishes |
|---|---|
| `build-base-image.yml` | `ghcr.io/matheus-marques-ft/koko-base:<timestamp>` — triggered by changes to `Dockerfile-base`, auto-commits the new tag into `Dockerfile` |
| `build-ghcr-image.yml` | `ghcr.io/matheus-marques-ft/koko:<tag>` — triggered on `v*` tags or manual dispatch |
| `release-drafter.yml` | drafts a GitHub Release with build artifacts — triggered on `v*` tags |
