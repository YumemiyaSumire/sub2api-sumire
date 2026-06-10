# Sub2API Sumire

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.25.7-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![GHCR](https://img.shields.io/badge/GHCR-reasoning--alias-2496ED.svg)](https://github.com/YumemiyaSumire/sub2api-sumire/pkgs/container/sub2api-sumire)

**基于 Sub2API 维护的 Sumire 增强分支。**

[原项目](https://github.com/Wei-Shaw/sub2api) |
[当前 Fork](https://github.com/YumemiyaSumire/sub2api-sumire) |
[自定义镜像](https://github.com/YumemiyaSumire/sub2api-sumire/pkgs/container/sub2api-sumire)

</div>

---

## 项目简介

Sub2API Sumire 是基于 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 维护的增强分支。原项目本身已经是一个完整的 AI API 网关，负责多账号管理、API Key 分发、用量计费、请求调度、限流和后台管理。

这个分支主要补了一些偏实际部署和日常使用的能力：OpenAI / Codex 模型别名、prompt cache 自动处理、后台入口收敛、分组账号测试，以及独立发布的 GHCR 镜像。

当前维护分支是：

```text
reasoning-alias
```

当前镜像：

```text
ghcr.io/yumemiyasumire/sub2api-sumire:reasoning-alias
```

当前分支已经同步到上游 `0.1.146`，并继续叠加 Sumire 分支的定制改动。这个 README 记录的是当前 `reasoning-alias` 相对上游 `main` 多出来的内容。

更详细的分支改动记录见 [SUMIRE_CHANGES.md](SUMIRE_CHANGES.md)。

## 主要改动

### OpenAI / Codex 模型别名

OpenAI 模型名支持更直观的后缀写法。客户端可以直接写：

```text
gpt-5.5-low
gpt-5.5-medium
gpt-5.5-high
gpt-5.5-xhigh
openai/gpt-5.5-high
```

服务端会把这些请求还原成真实模型名，再自动补上对应的 `reasoning.effort`。如果请求的是 `gpt-5.5` 或 `openai/gpt-5.5`，但没有显式写推理强度，就默认按 `medium` 处理。

另外还提供了一个偏快速响应的快捷别名：

```text
gpt-5.4-mini-fast
openai/gpt-5.4-mini-fast
```

它会走 `gpt-5.4-mini`，同时默认使用较低推理强度和 priority service tier，适合想要更快响应的场景。

### Prompt Cache 自动注入和排查

这个分支可以自动给 OpenAI GPT 文本请求补 prompt cache 参数。开关是：

```bash
SUB2API_OPENAI_AUTO_PROMPT_CACHE=1
SUB2API_OPENAI_PROMPT_CACHE_RETENTION=24h
```

它会尽量自动补：

- `prompt_cache_key`
- `prompt_cache_retention`

如果上游不接受自动加进去的 `prompt_cache_retention`，服务会只移除这个自动字段，然后用同一个账号重试一次。客户端自己传进来的字段不会被改掉。

排查缓存有没有命中时，可以打开：

```bash
SUB2API_DEBUG_CACHE_KEYS=1
```

缓存诊断日志会把请求入口、转发前参数、粘性调度和上游返回的 cache usage 串起来看，常用日志名是：

- `openai.cache_debug_ingress`
- `openai.cache_debug_forward`
- `openai.cache_debug_sticky`
- `openai.cache_debug_result`

## 部署

如果使用 Docker Compose，把 `sub2api` 服务镜像指向这个分支的镜像即可：

```yaml
image: ghcr.io/yumemiyasumire/sub2api-sumire:reasoning-alias
```

更新时在部署目录执行：

```bash
docker compose pull sub2api
docker compose up -d sub2api
docker compose ps
docker compose logs --tail=100 sub2api
```

当前 EC2 部署目录约定为：

```text
/home/ubuntu/sub2api-deploy
```

这个目录是 Docker 部署目录，不是源码仓库，所以更新时不要在里面 `git pull`，直接拉镜像并重建容器即可。

## 本地开发

后端：

```bash
cd backend
go run ./cmd/server
```

前端：

```bash
cd frontend
pnpm install
pnpm run dev
```

改了 Ent schema 或 Wire 依赖后：

```bash
cd backend
go generate ./ent
go generate ./cmd/server
```

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go 1.25.7, Gin, Ent |
| 前端 | Vue 3.4+, Vite 5+, TailwindCSS |
| 数据库 | PostgreSQL 15+ |
| 缓存 / 队列 | Redis 7+ |
| 部署 | Docker, Docker Compose, GHCR |

## 说明

这个 fork 继承原 Sub2API 的许可证和免责声明。它主要面向自托管和定制部署场景，不代表原项目官方发布。使用它访问第三方 AI 服务时，请自行确认服务条款、账号风险和部署安全边界。

许可证见 [LICENSE](LICENSE)。原项目文档可以看 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)。
