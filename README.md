# DomainOS — 多域名建站平台

轻量级多域名绑定建站平台，支持用户绑定域名后进行页面制作、模板渲染或智能跳转。

## 技术栈

| 组件 | 技术 | 用途 |
|------|------|------|
| 后端 | Go (Gin) | HTTP 服务、域名路由、管理 API |
| 数据库 | SQLite (WAL) | 用户、域名、模板、通知、API Key |
| 缓存 | sync.Map | 域名内存缓存，零依赖高性能 |
| 安全 | Rate Limiting + bcrypt | 全局限流、密码加密 |
| 前端 | Tailwind CSS + Lucide Icons | 响应式毛玻璃 UI |

## 快速开始

```bash
# 前置条件：Go 1.22+, GCC (CGO for SQLite)

# 1. 安装依赖
GOPROXY=https://goproxy.cn,direct go mod tidy

# 2. 编译
CGO_ENABLED=1 go build -o bin/server ./cmd/server

# 3. 运行
PORT=8080 ./bin/server

# 访问 http://localhost:8080
```

## 项目结构

```
web/
├── cmd/server/
│   └── main.go               # 全部后端逻辑（模型/路由/中间件/缓存/API）
├── templates/                  # 21 个 HTML 模板
│   ├── index.html             # 首页：域名查询
│   ├── register.html          # 注册
│   ├── login.html             # 登录
│   ├── dashboard.html         # 用户控制台
│   ├── domain-edit.html       # 域名编辑（CodeMirror）
│   ├── analytics.html         # 统计分析（Chart.js）
│   ├── user-templates.html    # 模板中心
│   ├── user-files.html        # 文件管理
│   ├── notifications.html     # 通知中心
│   ├── api-keys.html          # API Key 管理
│   ├── user-settings.html     # 个人设置
│   ├── admin.html             # 管理仪表盘
│   ├── admin-domains.html     # 域名管理
│   ├── admin-users.html       # 用户管理
│   ├── admin-templates.html   # 模板管理
│   ├── admin-settings.html    # 站点设置
│   ├── admin-logs.html        # 操作日志
│   ├── site.html              # 域名展示页
│   ├── 403.html / 404.html / 429.html
├── uploads/                    # 用户上传文件目录
├── data/platform.db            # SQLite 数据库（自动生成）
├── docs/                       # 项目文档
├── Makefile                    # 构建命令
└── README.md
```

## 功能特性

### 用户端
- 注册 / 登录 / 退出
- 首页域名查询（可用性检测）
- 控制台（域名列表、真实访问统计、系统公告）
- 域名编辑（标题、内容、模板选择、自定义 CSS/JS）
- 三种跳转模式：301 永久 / 302 临时 / HTML Meta
- 统计分析（PV/UV、每日趋势、热门路径、来源分析）
- 模板中心（浏览、预览、应用到域名）
- 文件管理（上传、删除、复制链接，10MB 限制）
- 通知中心（已读/未读）
- API Key 管理
- 个人设置（昵称/邮箱/密码）

### 管理员后台
- 管理仪表盘（用户/域名/访问量统计）
- 域名管理（分页搜索、CRUD、启用禁用、批量操作、跳转类型）
- 用户管理（搜索分页、CRUD、角色权限、发送通知）
- 模板管理（CRUD、预览、启用禁用）
- 站点设置（标题/页脚/背景/统计代码/密码修改）
- 操作日志查看

### RESTful API
```bash
# 认证：Header X-API-Key 或查询参数 ?api_key=

# 域名列表
GET /api/v1/domains

# 添加域名
POST /api/v1/domains
{"hostname":"example.com","mode":"page","title":"Hello"}

# 删除域名
DELETE /api/v1/domains/:id

# 域名统计
GET /api/v1/domains/:id/stats
```

### 性能与安全
- 域名内存缓存（sync.Map + 启动预热）
- 全局 Rate Limiting（60 req/min）
- 操作日志记录
- bcrypt 密码加密

### 移动端适配
- 响应式布局（iOS / Android 输入框缩放修复）
- 侧边栏可收起 / 展开（汉堡菜单）
- 表单竖排、表格横向滚动

## 管理 API（公开查询）

```bash
# 域名查询
curl "http://localhost:8080/api/check?domain=example.com"
```

## 当前状态

**v0.6.0 — 全部 Phase 1-6 已完成 ✅**

| Phase | 功能 | 状态 |
|-------|------|------|
| 1 | 基础骨架 | ✅ |
| 2 | 管理员后台 | ✅ |
| 3 | 用户后台增强 | ✅ |
| 4 | 高级功能 | ✅ |
| 5 | 性能与扩展 | ✅ |
| 6 | 运营功能 | ✅ |

详见 [docs/TODO.md](docs/TODO.md)

## License

Private — 仅供内部使用
