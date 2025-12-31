<p align="center">
  <img src="internal/site/public/static/logo.svg" alt="Aether" width="140" />
</p>

# Aether

基于 Beszel 二次开发的服务器与容器监控平台。代码和功能已切换为 Aether 品牌，下载源与镜像来自本仓库。

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

## 品牌与致谢

- Aether 基于 Beszel 开发，向原作者 [henrygd/beszel](https://github.com/henrygd/beszel) 致谢。
- 所有新特性、修复与品牌素材（如 `internal/site/public/static/logo.svg`）均以 Aether 名义发布。

## 许可

沿用原项目许可证，遵循本仓库附带的开源授权文件。
