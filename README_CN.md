# Hekato-Go

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Hekato-Go 是一个支持 Kiro 与 CodeBuddy 账号的多上游 OpenAI / Anthropic 兼容 API 网关。

[English](README.md) | 中文

> **项目来源**
>
> Hekato-Go 起源于 [Quorinex/Kiro-Go](https://github.com/Quorinex/Kiro-Go) 的增强 Fork。
> 感谢 Quorinex/Kiro-Go 维护者创建的原始项目。
> 本 Fork 由 [billyriantono/Hekato-Go](https://github.com/billyriantono/Hekato-Go) 独立维护，并增加了 Kiro 与 CodeBuddy 等多 Provider 支持。
> 也感谢 [Quorinex/Kiro-Go PR #131](https://github.com/Quorinex/Kiro-Go/pull/131) 中的工作与讨论，为本 Fork 的 Enterprise SSO / Azure AD 支持提供了参考。

如果这个项目帮到了你，欢迎点个 Star 支持一下。

## 功能特性

- Anthropic `/v1/messages` 与 OpenAI `/v1/chat/completions`
- 多上游 Provider 路由：Kiro + CodeBuddy Global / China
- Kiro 与 CodeBuddy 使用独立请求转换路径
- 多账号池轮询负载均衡
- 自动 Token 刷新、SSE 流式输出、Web 管理面板
- 多种 Kiro 认证方式：AWS Builder ID、IAM Identity Center (企业 SSO)、SSO Token、本地缓存、凭证 JSON
- CodeBuddy API Key 账号导入
- 用量追踪、账号导入导出、中英双语
- 支持设置出站代理（SOCKS5 / HTTP）

## 快速开始

### Docker Compose（推荐）

```bash
git clone https://github.com/billyriantono/Hekato-Go.git
cd Hekato-Go
mkdir -p data
docker-compose up -d
```

### Docker 运行

```bash
docker run -d \
  --name hekato-go \
  -p 8080:8080 \
  -e ADMIN_PASSWORD=your_secure_password \
  -v /path/to/data:/app/data \
  --restart unless-stopped \
  ghcr.io/billyriantono/hekato-go:latest
```

### 源码编译

```bash
git clone https://github.com/billyriantono/Hekato-Go.git
cd Hekato-Go
go build -o hekato-go .
./hekato-go
```

### 部署到 Zeabur

仓库已包含 `Dockerfile`，可直接在 Zeabur 上构建运行。

**方式一：面板一键部署**

1. Fork 本仓库到你的 GitHub 账号。
2. 在 Zeabur 新建服务，选择 **Deploy from GitHub**，绑定刚才 fork 的仓库。
3. Zeabur 自动识别 `Dockerfile` 并完成构建。
4. 在 **Networking** 标签暴露端口 `8080` 并绑定域名。
5. 在 **Variables** 标签至少设置 `ADMIN_PASSWORD`（管理面板密码）。
6. 如需持久化账号 / 配置，挂载 Volume 到 `/app/data`。

**方式二：CLI 部署**

```bash
npm i -g zeabur
zeabur auth login
zeabur deploy
```

> 命令需在项目根目录执行。CLI 会生成 `.zeabur/context.json` 记录目标 project / service，包含个人 ID，请勿提交。

部署完成后访问 `https://<你的域名>/admin` 登录管理面板。

首次运行会在 `data/config.json` 自动生成配置，挂载 `/app/data` 以持久化。默认管理密码为 `changeme`，生产环境请务必通过 `ADMIN_PASSWORD` 环境变量或在管理面板中修改。

## 使用方法

访问 `http://localhost:8080/admin` 登录、添加账号，然后调用 API：

```bash
# Claude
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet-4.5","max_tokens":1024,"messages":[{"role":"user","content":"你好！"}]}'

# OpenAI
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer any" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"你好！"}]}'
```

## 思考模式

在模型名后加后缀（默认 `-thinking`）即可启用，例如 `claude-sonnet-4.5-thinking`。Claude 兼容请求如果带有顶层 `thinking` 配置，例如 `{"type":"enabled","budget_tokens":2048}` 或 `{"type":"adaptive"}`，也会自动启用 thinking 模式。输出格式可在管理面板「设置 - Thinking 模式」中配置。

## 出站代理

可在管理面板「设置 - 出站代理设置」中配置代理。支持 SOCKS5 和 HTTP 代理。

设置保存后即时生效，无需重启服务。

## 环境变量

| 变量 | 说明 | 默认值 |
|-----|------|-------|
| `CONFIG_PATH` | JSON 配置路径与迁移种子路径；也用于推导默认 SQLite 数据库位置。 | `data/config.json` |
| `DB_DRIVER` | 存储后端：`json` / `file`、`sqlite` / `sqlite3`，或 `postgres` / `postgresql` / `pgx`。不设置时使用兼容旧版本的 JSON 文件后端。 | `json` |
| `DATABASE_URL` | 数据库 DSN。SQLite 可填写文件路径，例如 `/app/data/kiro.db`；如果 `DB_DRIVER=sqlite` 且未填写，则默认使用 `CONFIG_PATH` 同目录下的 `kiro.db`。PostgreSQL 需填写类似 `postgres://user:pass@host:5432/db?sslmode=disable` 的连接 URL。 | - |
| `ADMIN_PASSWORD` | 管理面板密码。首次安装未设置时，可在浏览器首屏初始化；如设置该变量，会覆盖配置中的密码。 | - |
| `KIRO_SSO_CALLBACK_BIND` | Enterprise SSO 临时回调监听地址；Docker 发布回调端口时常用。 | 仅本机回环 |
| `KIRO_PROFILE_REGIONS` | 可选的 Kiro profile/usage 探测区域列表，逗号分隔。 | 内置区域 |
| `LOG_LEVEL` | 日志级别：`debug`、`info`、`warn` 或 `error`。 | `info` |

### 存储后端

默认情况下，Hekato-Go 使用兼容旧版本的单文件 `CONFIG_PATH` 存储。
如需启用 SQL 存储：

```bash
# SQLite
DB_DRIVER=sqlite
# 可选；默认使用 CONFIG_PATH 同目录下的 kiro.db
DATABASE_URL=/app/data/kiro.db

# PostgreSQL
DB_DRIVER=postgres
DATABASE_URL=postgres://user:pass@postgres:5432/hekato?sslmode=disable
```

首次启用 SQL 存储时，已有的 `config.json` 会作为迁移种子，用于迁移现有账号与设置。

## 参与贡献

欢迎友好交流。遇到问题时，建议先让 Claude Code、Codex 等工具帮忙排查一下，大部分问题都能自己解决。如果能直接提个 PR 就更好了。

## 友情链接

- [LINUX DO](https://linux.do)

## 免责声明

本项目仅供学习和研究目的使用，与 Amazon、AWS、Kiro、Tencent 或 CodeBuddy 没有任何关联。Hekato-Go 是基于 Quorinex/Kiro-Go 的独立维护增强 Fork。用户需自行确保使用行为符合所有适用的服务条款和法律法规，使用风险自负。

## 许可证

[MIT](LICENSE)
