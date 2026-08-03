# image-mcp-hub

image-mcp-hub 是一个基于 Go 实现的 HTTP 中间层服务，将任意兼容 OpenAI 的同步文生图接口（`/v1/images/generations`）封装为 MCP 工具，供多个 agent 通过 [streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/transports#streamable-http) 共用。

服务以单一常驻进程运行、维护一份统一配置，供多个客户端共享使用；配置经 Web 管理端修改后即时生效，无需中断现有连接。相较于社区常见的 stdio 传输方案，本方案无需为各客户端分别配置。项目采用 Go 编写，单二进制、单端口部署。

## 适用场景

- 存在多个 MCP 客户端，需共享同一批文生图模型，且不希望为每个客户端单独配置 stdio 进程与密钥；或客户端仅支持 HTTP 传输、不支持 stdio
- 需要通过 Web 界面统一管理模型增删、API key 轮换与用量统计，修改后对所有 agent 即时生效
- 上游接口返回形态不一（部分返回 URL、部分返回 base64），中间层统一完成下载与解码并保存为本地文件，agent 无需关注上游差异，直接使用本地图片链接

## 特性

- **单端口多路径**：`/mcp`（MCP 端点）、`/admin`（Web 管理端）、`/images/`（图片访问）复用同一 HTTP 服务
- **模型即工具**：每个已配置模型对应一个 MCP 工具，工具名使用自定义 alias，支持按需增删
- **参数透传**：`prompt` 为必填参数，`size` / `n` / `quality` / `style` / `background` / `output_format` 为可选参数，均原样转发至上游
- **`response_format` 内部处理**：上游返回 `url` 时由中间层下载，返回 `b64_json` 时由中间层解码，统一保存为本地文件，agent 获取到的始终为本地 URL；本地文件扩展名按图片实际字节判定，与 `output_format` 无关，避免扩展名与字节不符被误作为 Content-Type
- **API key 轮询**：每个模型的 `api_keys` 列表采用 round-robin 轮询，游标在内存推进、定时与退出时惰性落盘；遇 401/403 错误原样上抛，不做失效标记
- **配置热加载**：token、密码、清理规则、模型列表修改后即时生效；端口与存储目录需重启生效
- **Web 管理端**：支持中英双语、深浅主题、仪表盘（请求统计 / 趋势图 / 最近调用）、模型增删改查、图片浏览与全局设置
- **统计持久化**：请求统计定期写入 `data/stats.json`，重启后累计数据不丢失

## 架构

```text
Agents (A / B / Claude Code / Cursor / ...)
  │
  │  POST /mcp   ·   Authorization: Bearer <mcp_token>
  ▼
image-mcp-hub   (:12300 · Go · MCP)
  │  端点：/mcp (MCP·Bearer)   ·   /admin (Web·密码)   ·   /images/ (公开)
  │
  ├── POST /v1/images/generations ─────►  OpenAI 兼容上游
  │      上游返回 url → 中间层下载  /  b64_json → 中间层解码
  │
  ├── 保存图片 + sidecar .meta.json ──►  ./data/images/
  │
  └── 写请求统计 ─────────────────────►  ./data/stats.json

Web 管理端 /admin ── JSON API ──►  hub：模型管理 · 配置管理 · 统计查看
/images/<时间戳_uuid>.png ── 公开访问
```

## 快速开始

```bash
go build -o image-mcp-hub .
./image-mcp-hub
```

首次运行将在当前目录生成默认 `config.json`：

```json
{
  "server":  { "host": "0.0.0.0", "port": 12300, "mcp_token": "sk-123456", "admin_password": "password" },
  "storage": { "dir": "./data/images", "max_age_days": 0, "max_count": 0 },
  "models":  []
}
```

- 管理界面：<http://localhost:12300/admin>（默认密码 `password`，**部署前请务必修改**）
- MCP 端点：<http://localhost:12300/mcp>（Bearer `sk-123456`，**部署前请务必修改**）

在管理端「模型」页面添加一个模型（对应一个 MCP 工具），填写 alias、model_id、base_url、api_keys 与描述后，即可供 agent 调用。

## Docker

镜像发布至 GitHub Container Registry（GHCR），随每个 Release 自动构建，支持 `linux/amd64` 与 `linux/arm64` 架构（x86 服务器 / Apple Silicon / 树莓派均可使用）。配置、图片与统计数据统一存放于卷 `/app/data`，挂载单个卷即可完成持久化：

```bash
docker run -d --name image-mcp-hub \
  -p 12300:12300 \
  -v image-mcp-hub-data:/app/data \
  --restart unless-stopped \
  ghcr.io/echoping07/image-mcp-hub:latest
```

首次启动时会在卷内自动生成默认 `config.json`（默认凭证与上述一致，请务必修改）。端口 12300 复用三个路径。如需固定版本，可将 `:latest` 替换为具体 Release tag（如 `:v1.0.0`）。

**docker-compose.yml**：

```yaml
services:
  image-mcp-hub:
    image: ghcr.io/echoping07/image-mcp-hub:latest
    container_name: image-mcp-hub
    ports:
      - "12300:12300"
    volumes:
      - image-mcp-hub-data:/app/data
      # 如需使用宿主机配置覆盖：先 docker cp 拷出默认 config.json 并修改，
      # 再取消下行注释（bind-mount 文件须事先存在，且属主为容器内 app 用户 uid 1000）：
      # - ./config.json:/app/data/config.json
    restart: unless-stopped

volumes:
  image-mcp-hub-data:
```

> `mcp_token`、`admin_password`、清理规则与模型列表的修改走热加载，**无需重启容器**；端口与存储目录变更后需重建容器。
> `/images/` 无鉴权，对公网部署建议在反向代理层添加访问控制并启用 HTTPS。

## 客户端接入

任意支持 MCP streamable HTTP 的客户端，将端点指向 `/mcp` 并携带 Bearer token 即可接入。

**Claude Desktop**（`claude_desktop_config.json`）：

```json
{
  "mcpServers": {
    "image-mcp-hub": {
      "type": "http",
      "url": "http://localhost:12300/mcp",
      "headers": { "Authorization": "Bearer sk-123456" }
    }
  }
}
```

**Claude Code**：

```bash
claude mcp add --transport http image-mcp-hub \
  --url http://localhost:12300/mcp \
  --header "Authorization: Bearer sk-123456"
```

> 若服务部署于反向代理之后，请使用代理地址，并确保代理转发 `Authorization` 头。

## 工具参数与返回

每个模型暴露为同名的 MCP 工具：

| 参数              | 类型     | 必填  | 说明                                           |
| --------------- | ------ | --- | -------------------------------------------- |
| `prompt`        | string | ✅   | 提示词                                          |
| `size`          | string | -   | 尺寸，如 `1024x1024`                             |
| `n`             | number | -   | 生成图片数量                                       |
| `quality`       | string | -   | 生成质量                                         |
| `style`         | string | -   | 生成风格                                         |
| `background`    | string | -   | 图片背景                                         |
| `output_format` | string | -   | 输出格式（png/jpg/webp…），原样透传至上游；本地文件扩展名按图片实际字节判定 |

返回值：一个或多个**本地图片 URL**，格式如 `http://host:12300/images/20060102-150405_uuid.png`，按行分隔。

## 端点

| 路径         | 说明                     | 鉴权                                  |
| ---------- | ---------------------- | ----------------------------------- |
| `/mcp`     | MCP streamable HTTP 端点 | `Authorization: Bearer <mcp_token>` |
| `/admin`   | Web 管理界面               | 密码登录                                |
| `/images/` | 图片访问（公开，默认开启目录列举）      | 无                                   |

## 图片存储与清理

- 存储位置：`./data/images/`
- 命名规则：`20060102-150405_uuid.png`（时间戳精确到秒）
- 元数据：以 sidecar 文件 `20060102-150405_uuid.meta.json` 保存（包含模型名、prompt、参数、时间、上游信息）
- 清理规则：两条独立规则，满足任一即删除；均设为 0 时永久留存（默认配置）：
  - `max_age_days`（0 表示不限时）
  - `max_count`（0 表示不滚动）

## Web 管理端

采用密码登录（默认密码 `password`）。主要功能：仪表盘（请求总数、成功/失败数、成功率、出图数、近 30 天趋势图、按模型成功率统计、失败原因 Top、最近调用记录含耗时与错误）、中英双语（浏览器为中文环境时默认中文，可一键切换并持久化偏好）、深浅主题（首次访问读取系统偏好，之后手动切换并持久化）、模型增删改查（描述附填写模板）、全局设置（端口、MCP token、管理密码、清理规则、监听地址）、图片浏览与删除。

> 端口、监听地址与存储目录的修改需重启生效（`PUT /admin/api/config` 响应含 `restart_required` 字段提示是否需重启）；MCP token、管理密码、清理规则与模型列表均支持热加载。

## 配置字段

| 字段 | 说明 |
|------|------|
| `server.host` | 监听地址，`0.0.0.0` 对外开放 |
| `server.port` | 监听端口，默认 12300 |
| `server.mcp_token` | `/mcp` 端点 Bearer token |
| `server.admin_password` | `/admin` 登录密码 |
| `storage.dir` | 图片存储目录 |
| `storage.max_age_days` | 按时间清理，0 = 不限时 |
| `storage.max_count` | 按数量滚动清理，0 = 不滚动 |
| `models[].name` | 自定义 alias，用作 MCP 工具名（`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`） |
| `models[].model_id` | 真实模型 id，转发上游时使用 |
| `models[].base_url` | 上游 OpenAI 兼容端点（含或不含 `/v1` 均可） |
| `models[].api_keys` | 该模型的 key 列表，round-robin 轮询 |
| `models[].key_index` | 轮询游标（定时 + 退出时惰性落盘，重启后接续） |
| `models[].description` | 工具描述，由用户自行填写 |

## 安全

- **默认凭证仅适用于本地体验**：`mcp_token` 与 `admin_password` 在部署前必须修改
- **`config.json` 包含明文 API key 与管理密码，文件权限为 `0600`，已被 `.gitignore` 忽略**——请勿修改后提交至仓库
- `/images/` 无鉴权且默认开启目录列举（`GET /images/` 返回完整文件列表，任何人可读取图片及 sidecar 元数据，含 prompt、上游信息）；对公网部署务必在反向代理层添加访问控制并启用 HTTPS
- 对公网开放前，建议在反向代理后启用 HTTPS
- 上游 key 遇 401/403 等错误时原样上抛，**不会自动标记失效**，需在管理端手动调整

## 项目结构

```
.
├── main.go              # 入口：路由、定时清理、统计存储、优雅退出
├── go.mod / go.sum      # Go 模块依赖
├── config.json          # 配置
├── Dockerfile           # 多阶段构建：纯 Go 交叉编译 → alpine 运行镜像
├── .dockerignore        # Docker 构建上下文过滤
├── .github/
│   └── workflows/
│       └── release.yml  # Release 触发：6 平台二进制 + 多架构 Docker 镜像
├── internal/
│   ├── config/          # config.json 加载 + 热重载 + key 轮询游标
│   ├── mcpserver/       # MCP 工具注册/调用、Bearer 鉴权、streamable HTTP
│   ├── upstream/        # OpenAI 兼容上游客户端（url 下载 / b64_json 解码）
│   ├── storage/         # 图片存储、sidecar 元数据、清理规则
│   ├── stats/           # 请求统计（内存 + data/stats.json 持久化）
│   ├── admin/           # /admin/api/* JSON API、会话登录
│   └── web/             # 管理端 SPA（go:embed 打包进二进制）
│       └── static/      # 前端资源（app.js / index.html / style.css）
├── data/                # 运行时数据（images/、stats.json，已 gitignore）
└── LICENSE              # MIT
```

## 开发

```bash
go test ./...      # 单元与集成测试（含 mock 上游全流程）
go vet ./...
go build -o image-mcp-hub .
```

环境变量 `IMAGE_MCP_HUB_CONFIG` 可指定配置文件路径。

## FAQ

**修改端口 / 存储目录后为何需要重启？**
二者在进程启动时固定（监听地址、存储句柄）；token、密码、清理规则与模型列表走热加载，修改后即时生效。

**上游无法识别的参数会如何处理？**
参数原样透传，由上游自行忽略；`response_format` 由中间层接管，不会转发至上游。

**key 轮询遇到失效的 key 会怎样？**
错误原样上抛给 agent，不做失效标记；可在管理端编辑模型并替换 key。

**是否支持异步任务制供应商？**
不支持，仅支持同步 `generations` 接口。上游返回 `url` 或 `b64_json` 两种形态均会被下载/解码为本地文件。

**统计与图片是否会随仓库推送？**
不会。`data/`、`config.json`、`.obsidian/` 均已在 `.gitignore` 中排除。

## 开源协议

[MIT License](./LICENSE) © 2026 EchoPing
