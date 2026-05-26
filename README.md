# Sub2API Sumire

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.25.7-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/GHCR-reasoning--alias-2496ED.svg)](https://github.com/YumemiyaSumire/sub2api-sumire/pkgs/container/sub2api-sumire)

**基于 Sub2API 的 Sumire 自用增强分支**

[原项目](https://github.com/Wei-Shaw/sub2api) |
[当前 Fork](https://github.com/YumemiyaSumire/sub2api-sumire) |
[自定义镜像](https://github.com/YumemiyaSumire/sub2api-sumire/pkgs/container/sub2api-sumire)

</div>

---

## 项目说明

Sub2API 是一个用于订阅额度分发和统一转发的 AI API 网关平台，支持多账号管理、API Key 分发、计费统计、调度、限流和后台管理。

`sub2api-sumire` 是在原项目基础上维护的个人增强分支，核心目标是：

- 更好地适配 OpenAI / Codex 类模型的推理强度、优先级和 prompt cache 使用方式。
- 增加 OAuth 账号接近额度窗口时的自动休眠保护，降低账号被打到极限后的不可用风险。
- 保留上游 Sub2API 的主要功能，同时加入 Sumire 分支自己的镜像、版本标识、后台入口调整和测试维护。

当前主要维护分支：

```text
reasoning-alias
```

当前自定义 Docker 镜像：

```text
ghcr.io/yumemiyasumire/sub2api-sumire:reasoning-alias
```

当前分支以已同步的上游 `0.1.131` 为基底，继续叠加 Sumire 自定义改动。

## Sumire 分支特性

### OpenAI 模型别名与推理强度

支持在模型名后追加推理强度后缀，让客户端不用手动写 `reasoning.effort`：

```text
gpt-5.5-low
gpt-5.5-medium
gpt-5.5-high
gpt-5.5-xhigh
openai/gpt-5.5-high
```

转发到上游时会去掉后缀，并自动注入对应的 `reasoning.effort`。当请求 `gpt-5.5` 或 `openai/gpt-5.5` 且客户端没有显式传入推理强度时，默认使用 `medium`。

另外支持：

```text
gpt-5.4-mini-fast
openai/gpt-5.4-mini-fast
```

这类别名会映射到 `gpt-5.4-mini`，并默认注入低推理强度与 priority service tier。

### OpenAI Prompt Cache 增强

可通过环境变量开启自动 prompt cache 注入：

```bash
SUB2API_OPENAI_AUTO_PROMPT_CACHE=1
SUB2API_OPENAI_PROMPT_CACHE_RETENTION=24h
```

支持自动注入：

- `prompt_cache_key`
- `prompt_cache_retention`

如果上游明确拒绝自动注入的 `prompt_cache_retention`，会移除自动注入字段并使用原账号重试一次；客户端显式传入的字段不会被擅自删除。

调试缓存命中时可以开启：

```bash
SUB2API_DEBUG_CACHE_KEYS=1
```

关键日志包括：

- `openai.cache_debug_ingress`
- `openai.cache_debug_forward`
- `openai.cache_debug_sticky`
- `openai.cache_debug_result`

### OAuth Sleeper 休眠保护

后台新增 OAuth Sleeper，用于根据已记录的 OAuth 账号用量自动将接近额度窗口的账号置为休眠限流。

支持能力：

- 按 OpenAI / Anthropic 平台选择是否参与扫描。
- 按分组选择需要保护的账号范围。
- 配置触发阈值、扫描间隔、每轮每组最多休眠账号数。
- 记录休眠事件，展示当前休眠账号。
- 当同一分组短时间内出现缓存命中消耗时，自动临时加快扫描。
- 与粘性会话兼容，避免刚被标记休眠时立刻打断仍在收尾的会话。

后台入口：

```text
/admin/oauth-sleeper
```

### 后台与运维调整

- 前端版本徽标显示 Sumire 标识，例如 `v0.1.131 - Sumire`。
- 账号列表搜索同时支持账号名和账号邮箱。
- 模型组导入账号时支持账号白名单配置。
- 隐藏部分不需要暴露的后台侧栏和渠道管理入口。
- 增加专用 GHCR 构建流程，推送 `reasoning-alias` 分支时构建自定义镜像。

## 相对原项目的更改记录

下面按提交日期列出 Sumire 分支在已同步上游基底之后叠加的全部自定义改动。

### 2026-05-21

- `10507af2` 支持 OpenAI 推理强度模型别名。新增 `low`、`medium`、`high`、`xhigh` 后缀解析，客户端请求如 `gpt-5.5-high` 时会转成真实模型 `gpt-5.5` 并注入 `reasoning.effort=high`。
- `db839825` 新增 GitHub Actions 自定义镜像构建流程。推送 `reasoning-alias` 分支后构建并推送 GHCR 镜像。
- `2dd16337` 修正 GHCR 镜像 tag 为小写，避免容器镜像标签大小写导致构建或拉取异常。
- `d4b53ece` 增加 `gpt-5.5` 默认推理强度与 Sumire 版本标识。未显式指定推理强度时默认使用 `medium`，前端版本展示增加 Sumire 标记。

### 2026-05-22

- `d9ad57bd` 优化前端版本徽标展示，并新增缓存排查日志开关 `SUB2API_DEBUG_CACHE_KEYS`。
- `e574dcd7` 修复缓存诊断日志结果类型签名，确保缓存排查结果日志字段类型与服务结果结构一致。
- `bddaa04c` 补充 `js-cookie` 安全扫描例外，处理依赖安全扫描中已确认的前端依赖告警。
- `c304fb26` 修复缓存诊断日志测试中的 usage 类型，保持测试数据与真实 usage 字段一致。
- `34b71b94` 触发 GHCR 镜像构建，用于验证自定义镜像工作流。
- `e050927d` 增加缓存诊断请求哈希日志，记录请求体、输入内容、输入前缀等哈希，便于判断 prompt cache 是否复用同一前缀。
- `969f7e4a` 自动注入 OpenAI prompt cache 参数。新增 `SUB2API_OPENAI_AUTO_PROMPT_CACHE` 和 `SUB2API_OPENAI_PROMPT_CACHE_RETENTION`，在兼容路径和 Responses 路径中自动补充缓存参数。
- `4f1b07f6` 新增 `gpt-5.4-mini-fast` 快速别名，将其映射为 `gpt-5.4-mini`，并默认注入低推理强度和 priority service tier。
- `4de9dcdd` 支持模型组导入账号白名单，让分组批量导入账号时可以带上模型白名单相关配置。

### 2026-05-23

- `a177c264` 补全 OpenAI 缓存诊断链路，串联 ingress、forward、sticky、result 等日志，方便从请求进入到上游返回追踪缓存行为。
- `d7556791` 迁移 h2c 配置以修复 lint 告警，从旧的包装器方式迁移到 Go 标准库 `http.Server.Protocols` / `http.HTTP2Config`，保留原 h2c 配置语义。
- `f4621ab7` 增强 OpenAI prompt cache 降级逻辑，并让账号搜索支持邮箱匹配。自动注入的 retention 被上游拒绝时会移除后重试，账号搜索同时匹配账号名和账号邮箱。
- `71b40321` 收窄账号邮箱搜索测试范围，让 repository 测试聚焦邮箱搜索行为本身，减少不相关集成影响。
- `62188190` 增强集成测试 CI 失败日志，失败时输出更多上下文，便于定位 repository 测试问题。
- `65bd0ee4` 收敛集成测试 CI 日志输出，减少无关噪音，让失败信息更集中。
- `8113cbd1` 增强 repository 集成测试失败诊断，补充更明确的错误定位信息。
- `b854c3a4` 修复账号邮箱搜索 SQL 占位符问题，解决搜索条件扩展后参数数量与 SQL 占位符不匹配的风险。

### 2026-05-24

- `4e33c00c` 内置 OAuth 休眠保护。新增后端服务、事件记录、后台接口、前端页面和迁移，用于自动休眠接近额度窗口的 OAuth 账号。
- `74225a32` 修复 OAuth 休眠事件迁移列名，保证数据库迁移字段与 repository 写入逻辑一致。
- `d8f71248` 优化 OAuth 休眠概览文案，让后台页面更清楚地展示扫描状态、休眠账号和事件信息。
- `b828b0f0` 隐藏后台侧栏部分管理入口，收敛 Sumire 分支不常用或不希望暴露的管理菜单。
- `a16a9362` 隐藏后台渠道管理入口，进一步简化后台可见入口。

### 2026-05-25

- `e940d1aa` 让 OAuth Sleeper 支持按分组执行。启用前必须选择分组，扫描、休眠上限、事件和状态都会按所选分组范围处理。
- `6bee7888` 修复前端 CI 中 `vue-router` mock 冲突，保证 OAuth Sleeper 等前端测试在 CI 中稳定运行。

### 2026-05-26

- `6081ee81` 修复同步上游 `0.1.131` 后的前端测试兼容问题，处理上游更新后测试环境和组件行为变化带来的失败。
- `a66c6e5a` 增强 OAuth Sleeper 智能加速与粘性兼容。缓存命中消耗触发时按分组临时加快扫描，同时为休眠账号保留短暂粘性宽限，避免误伤正在收尾的请求。
- `703252d0` 修复 KeyUsage 前端 lint 类型误报，处理前端类型检查中的误判问题。

## 快速部署

如果已经使用 Docker Compose 部署，可以把 `sub2api` 服务镜像指向：

```yaml
image: ghcr.io/yumemiyasumire/sub2api-sumire:reasoning-alias
```

更新镜像：

```bash
docker compose pull sub2api
docker compose up -d sub2api
docker compose logs -f --tail=100 sub2api
```

完整安装、配置项、数据库、Redis、反向代理和安全设置请参考原项目文档：

- [Sub2API README](https://github.com/Wei-Shaw/sub2api)
- [Sub2API 中文 README](README_CN.md)

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

生成 Ent / Wire：

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

## 免责声明

本项目继承原 Sub2API 的开源协议和免责声明，仅用于技术学习、研究和自托管场景。使用本项目访问第三方 AI 服务可能受到对应服务条款限制，账号风险、服务中断、费用损失等后果由使用者自行承担。

原项目版权与许可证信息请查看 [LICENSE](LICENSE)。
