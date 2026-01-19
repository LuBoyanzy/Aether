<p align="center">
  <img src="internal/site/public/static/logo.svg" alt="Aether" width="140" />
</p>
<h1 align="center">Aether</h1>
<p align="center">服务器与容器监控平台</p>

<p align="center">
  <strong>Top-Rated Web-based Linux Server Monitoring</strong> · <strong>New-Gen Ops Panel</strong>
</p>

<p align="center">
  <a href="https://github.com/LuBoyanzy/Aether/releases"><img src="https://img.shields.io/github/v/release/LuBoyanzy/Aether?label=Release" alt="GitHub Release"></a>
  <a href="https://github.com/LuBoyanzy/Aether"><img src="https://img.shields.io/github/stars/LuBoyanzy/Aether?style=social" alt="GitHub Stars"></a>
  <a href="https://github.com/LuBoyanzy/Aether/blob/main/LICENSE"><img src="https://img.shields.io/github/license/LuBoyanzy/Aether" alt="License"></a>
</p>

<p align="center">
  <a href="https://github.com/LuBoyanzy/Aether/releases">下载</a> ·
  <a href="#快速体验docker-compose">快速体验</a> ·
  <a href="#安装脚本推荐">安装脚本</a> ·
  <a href="#本地开发">本地开发</a>
</p>

<p align="center">
  English · 中文(简体) · 日本語 · Português (Brasil) · العربية · Deutsch · Español · français · 한국어 · Bahasa Indonesia · 中文(繁體) · Türkçe · Русский · Bahasa Melayu
</p>

> Aether 面向自托管场景，镜像与安装脚本全部指向本仓库的发布版本。

## 特性速览

- 轻量化 Web UI，支持多用户与 OAuth/OIDC 登录
- 主机 + Docker/Podman 监控：CPU、内存、磁盘、网络、S.M.A.R.T. 等
- 告警、静默时段与多渠道通知
- Hub / Agent 分离架构，支持自动备份与在线更新

## 快速体验（Docker Compose）

```yaml
services:
  aether-hub:
    image: loboyanzy/aether
    container_name: aether-hub
    restart: unless-stopped
    ports:
      - "19090:19090"
    volumes:
      - ./aether_data:/aether_data

  aether-agent:
    image: loboyanzy/aether-agent
    container_name: aether-agent
    restart: unless-stopped
    network_mode: host
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      LISTEN: 45876
      KEY: "你的公钥"
      TOKEN: "可选 token"
      HUB_URL: "http://localhost:19090"
```

启动后访问 `http://localhost:19090`。

## 发布与镜像

- Hub 镜像：`loboyanzy/aether`  
- Agent 镜像：`loboyanzy/aether-agent`  
- Releases：`https://github.com/LuBoyanzy/Aether/releases`

## 安装脚本（推荐）

Hub：
```bash
curl -sL https://raw.githubusercontent.com/LuBoyanzy/Aether/main/supplemental/scripts/install-hub.sh \
  -o install-hub.sh && chmod +x install-hub.sh
./install-hub.sh
```

Agent：
```bash
curl -sL https://raw.githubusercontent.com/LuBoyanzy/Aether/main/supplemental/scripts/install-agent.sh \
  -o install-agent.sh && chmod +x install-agent.sh
./install-agent.sh -k "ssh-ed25519 ...你的公钥..." -url "http://<hub地址>:19090"
```

提示：  
- macOS 使用 `supplemental/scripts/install-agent-brew.sh`（直接从 GitHub Releases 下载）。  
- Windows 使用 PowerShell 版本 `supplemental/scripts/install-agent.ps1`。

## 本地开发

1) 启动前端（默认 19091，可改 `PORT`）：  
`PORT=19091 make dev-server`

2) 启动 Hub（默认 19090）：  
`APP_URL=http://localhost:19090 make dev-hub`

3) 启动 Agent（示例参数请替换）：  
`KEY="ssh-ed25519 xxx" TOKEN="xxx" HUB_URL="http://localhost:19090" make dev-agent`

> 若端口冲突，请先释放 19090/19091 或调整上述变量。

## 许可

MIT License，详见本仓库 `LICENSE`。
