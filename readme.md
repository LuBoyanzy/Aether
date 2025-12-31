# Aether

Aether 是一款轻量化的服务器监控平台，基于 Beszel 开发。当前官网与新文档尚未上线，安装脚本和下载地址沿用原有 Beszel 版本。

## 主要特性

- 轻量易用的 Web 界面，支持多用户与 OAuth/OIDC 登录
- 采集主机与 Docker/Podman 容器的 CPU、内存、磁盘、网络、S.M.A.R.T. 等指标
- 可配置的告警与静默时段，支持邮件和多种通知渠道
- Hub 与 Agent 分离架构，支持自动备份

## 本地开发启动

1. 启动 Hub  
   `APP_URL=http://localhost:19090 make dev-hub`
2. 启动前端（可自定义端口，如 19091）  
   `PORT=19091 make dev-server`
3. 启动 Agent（示例：请填入实际 KEY/TOKEN/HUB_URL）  
   `KEY="..." TOKEN="..." HUB_URL="http://localhost:19090" make dev-agent`

> 安装与在线安装脚本仍指向原 Beszel 下载源，后续 Aether 专属下载源与官网将另行提供。
