# Sub2API Sumire

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.25.7-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![GHCR](https://img.shields.io/badge/GHCR-reasoning--alias-2496ED.svg)](https://github.com/YumemiyaSumire/sub2api-sumire/pkgs/container/sub2api-sumire)

**我在 Sub2API 上维护的 Sumire 自用分支。**

[原项目](https://github.com/Wei-Shaw/sub2api) |
[当前 Fork](https://github.com/YumemiyaSumire/sub2api-sumire) |
[自定义镜像](https://github.com/YumemiyaSumire/sub2api-sumire/pkgs/container/sub2api-sumire)

</div>

---

## 这是什么

这个仓库是我基于 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 维护的个人 fork。原项目本身已经是一个完整的 AI API 网关，负责多账号管理、API Key 分发、用量计费、请求调度、限流和后台管理。

我这个分支主要是在原项目基础上加了一些更贴近自己使用方式的东西：OpenAI / Codex 模型别名、prompt cache 自动处理、OAuth 账号休眠保护、后台入口收敛，以及自己部署用的 GHCR 镜像。

当前维护分支是：

```text
reasoning-alias
```

当前镜像是：

```text
ghcr.io/yumemiyasumire/sub2api-sumire:reasoning-alias
```

当前分支已经同步到上游 `0.1.132`，然后继续叠加 Sumire 分支自己的改动。这个 README 记录的是当前 `reasoning-alias` 相对上游 `main` 多出来的内容。

## 我主要改了什么

### OpenAI / Codex 模型别名

我给 OpenAI 模型名加了一层更顺手的后缀写法。客户端可以直接写：

```text
gpt-5.5-low
gpt-5.5-medium
gpt-5.5-high
gpt-5.5-xhigh
openai/gpt-5.5-high
```

服务端会把这些请求还原成真实模型名，再自动补上对应的 `reasoning.effort`。如果请求的是 `gpt-5.5` 或 `openai/gpt-5.5`，但没有显式写推理强度，就默认按 `medium` 处理。

另外还加了一个我常用的快捷别名：

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

我加的日志会把请求入口、转发前参数、粘性调度和上游返回的 cache usage 串起来看，常用日志名是：

- `openai.cache_debug_ingress`
- `openai.cache_debug_forward`
- `openai.cache_debug_sticky`
- `openai.cache_debug_result`

### OAuth Sleeper

OAuth Sleeper 是我给 OAuth 账号加的一层保护。它会根据 Sub2API 已经记录到的用量快照，找出快接近额度窗口的账号，然后临时把这些账号置为休眠限流。

现在它支持：

- 按 OpenAI / Anthropic 平台决定是否参与扫描。
- 按账号分组决定保护范围。
- 全局阈值和分组阈值，账号命中多个分组时取更保守的阈值。
- 每轮扫描限制新休眠账号数量，避免一次扫太猛。
- 用量快照触发临时加速扫描，真正有消耗时才更积极地检查。
- 休眠事件记录、当前休眠账号展示、后台手动扫描。
- 和粘性会话配合，账号刚被休眠时保留很短的宽限时间，减少正在收尾的请求被误伤。

后台入口：

```text
/admin/oauth-sleeper
```

### 后台和部署习惯

这个分支还做了一些更偏自用部署的调整：

- 前端版本徽标会显示 Sumire 标识。
- 账号列表搜索可以同时按账号名和账号邮箱查。
- 模型组导入账号时可以带模型白名单。
- 隐藏了一些我不想在后台暴露的管理入口。
- 推送 `reasoning-alias` 分支时会构建自定义 GHCR 镜像。

## 按日期记录的改动

下面这些是当前分支相对原项目多出来的提交。因为这个分支会跟随上游 rebase，同一件事的提交哈希以后可能还会变，这里记录的是当前分支上的哈希。

### 2026-05-21

这一天先把 Sumire 分支的核心方向定下来了：让模型名本身就能表达推理强度，并准备好自己的镜像发布流程。

- 支持 `low`、`medium`、`high`、`xhigh` 这些 OpenAI 推理强度模型后缀，例如 `gpt-5.5-high` 会转成 `gpt-5.5` 并注入 `reasoning.effort=high`。`00c0d12f`
- 增加 GitHub Actions，用来构建并推送 Sumire 自己的 GHCR 镜像。`8402a98e`
- 把 GHCR 镜像 tag 统一成小写，避免镜像标签大小写引起构建或拉取问题。`2269fd1c`
- 给 `gpt-5.5` 加默认 `medium` 推理强度，并让前端版本展示带上 Sumire 标识。`989d9e02`

### 2026-05-22

这一天主要围绕 OpenAI prompt cache 做排查和自动化。目标不是只把参数塞进去，而是线上真的出问题时能看清楚它为什么没有命中。

- 优化版本徽标展示，并加上 `SUB2API_DEBUG_CACHE_KEYS` 这个缓存排查开关。`db614288`
- 修正缓存诊断日志里结果类型签名不一致的问题。`eb62fc5c`
- 给 `js-cookie` 安全扫描补充例外，处理前端依赖扫描里的已知告警。`610eac60`
- 修正缓存诊断日志测试里的 usage 类型，让测试和真实 usage 结构对齐。`6e362974`
- 手动触发一次 GHCR 镜像构建，用来验证镜像工作流。`3b092783`
- 给缓存诊断增加请求体、输入内容和输入前缀哈希，方便判断两次请求到底是不是同一个可缓存前缀。`116e541f`
- 支持自动注入 OpenAI prompt cache 参数，新增 `SUB2API_OPENAI_AUTO_PROMPT_CACHE` 和 `SUB2API_OPENAI_PROMPT_CACHE_RETENTION`。`ccb24bd2`
- 新增 `gpt-5.4-mini-fast` 快捷别名，默认走低推理强度和 priority service tier。`b33f0d8d`
- 模型组导入账号时支持一起导入模型白名单。`fa991cf9`

### 2026-05-23

这一天是在把前一天的缓存能力补完整，同时处理同步上游后出现的维护问题。

- 把缓存诊断链路补齐，从请求入口、转发前参数、粘性调度到上游返回结果都能串起来。`37e78b2c`
- 把 h2c 配置迁移到 Go 标准库 `http.Server.Protocols` / `http.HTTP2Config`，修掉 lint 告警，同时保留原来的 h2c 配置含义。`59012838`
- 增强 prompt cache retention 降级逻辑，同时让账号搜索可以匹配账号邮箱。`3e64c340`
- 收窄账号邮箱搜索测试范围，让测试更聚焦。`7f6b59fb`
- 增强集成测试 CI 失败日志，失败时给更多上下文。`5fc29a52`
- 收敛集成测试 CI 日志输出，减少干扰信息。`93580bd3`
- 给 repository 集成测试补更明确的失败诊断信息。`d608bd1e`
- 修复账号邮箱搜索 SQL 占位符数量不匹配的问题。`c364e3a7`

### 2026-05-24

这一天开始把 OAuth Sleeper 做进项目里。它不是一个单独脚本，而是接进了后端服务、数据库迁移、后台 API 和前端页面。

- 内置 OAuth 休眠保护，新增后端服务、事件记录、后台接口、前端页面和数据库迁移。`bb775025`
- 修正 OAuth 休眠事件迁移里的列名，让迁移和 repository 写入逻辑一致。`f1f1b0d7`
- 优化 OAuth 休眠概览文案，让后台看起来更容易理解。`12759c35`
- 隐藏后台侧栏里一部分不需要暴露的管理入口。`15dbc8c7`
- 隐藏后台渠道管理入口，让自用后台更干净。`233357d6`

### 2026-05-25

这一天把 OAuth Sleeper 从“全局扫描”推进到“按分组保护”。这样不同用途的账号可以有不同的保护范围。

- OAuth Sleeper 支持按分组执行，启用时需要选择分组，扫描范围、休眠上限、事件和状态都会按选中分组处理。`d78d12ae`
- 修复前端 CI 里的 `vue-router` mock 冲突，让 OAuth Sleeper 相关前端测试能稳定跑。`afde08ce`

### 2026-05-26

这一天主要是同步上游 `0.1.131` 后的兼容修复，以及 OAuth Sleeper 和粘性会话之间的体验打磨。

- 修复同步上游 `0.1.131` 后出现的前端测试兼容问题。`a5f939d9`
- 增强 OAuth Sleeper 智能加速与粘性兼容。缓存命中消耗会按分组临时加快扫描，休眠账号也会保留短暂粘性宽限。`3ed454e7`
- 修复 KeyUsage 页面里的前端 lint 类型误报。`f4b4ac22`

### 2026-05-27

这一天一边把项目说明页改成 Sumire 分支自己的介绍页，一边继续调整 OAuth Sleeper 的触发策略。

- 重写 README，让仓库首页从原项目通用说明改成 Sumire fork 的中文介绍页。`e0668029`
- 优化 OAuth Sleeper 用量阈值分流。默认阈值从 95% 调整为 90%，支持按分组覆盖阈值，多分组命中时取最低有效阈值；同时改成基于账号用量快照触发加速扫描，并把粘性会话宽限调整为 30 秒。`42b87983`

### 2026-05-28

这一天是同步上游 `0.1.132` 后的后端收尾，主要是把 fork 自己的代码和上游新增结构重新对齐。

- 修复 `0.1.132` 同步后的后端兼容问题，包括 group Ent 运行时字段索引、OpenAI gateway 测试构造参数，以及 Chat Completions 合并后遗留的未使用变量。`2bada656`

## 部署

如果你用的是 Docker Compose，把 `sub2api` 服务镜像指向这个分支的镜像即可：

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

我自己的 EC2 部署目录是：

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

这个 fork 继承原 Sub2API 的许可证和免责声明。它主要是我自己的部署和使用需求，不代表原项目官方发布。使用它访问第三方 AI 服务时，请自行确认服务条款、账号风险和部署安全边界。

许可证见 [LICENSE](LICENSE)。原项目文档可以看 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)。
