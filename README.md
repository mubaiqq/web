# DomainOS — 多域名建站平台

轻量级多域名绑定建站平台，支持用户绑定域名后进行页面制作、模板渲染或智能跳转。

## 技术栈

| 组件 | 技术 | 用途 |
|------|------|------|
| 后端 | Go (Gin) | HTTP 服务、域名路由、管理 API |
| 业务数据库 | SQLite (WAL) | 用户、域名绑定、模板配置 |
| 访问日志 | ClickHouse（规划中） | 高性能写入、统计分析 |
| 日志缓冲 | Redis（规划中） | 削峰填谷 |
| HTTPS | Caddy（规划中） | 自动 Let's Encrypt 证书 |
| 前端 | Tailwind CSS + Lucide Icons | 响应式 UI |

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
domain-platform/
├── cmd/server/main.go          # 入口：路由、初始化、优雅退出
├── templates/                   # HTML 模板（Tailwind CSS）
│   ├── index.html              # 首页：域名查询
│   ├── register.html           # 注册
│   ├── login.html              # 登录
│   ├── dashboard.html          # 用户控制台
│   ├── 404.html                # 域名未绑定
│   └── site.html               # 绑定域名展示页
├── static/css/style.css        # 备用样式（当前用 Tailwind CDN）
├── data/platform.db            # SQLite 数据库（自动生成）
├── docs/                       # 项目文档
│   ├── FEATURES.md             # 功能清单
│   ├── ARCHITECTURE.md         # 架构设计
│   ├── TODO.md                 # 开发路线图
│   └── PAGES.md                # 页面清单
├── Makefile                    # 构建命令
├── docker-compose.yml          # 依赖服务（ClickHouse/Redis/Caddy）
└── README.md
```

## 管理 API

```bash
# 域名查询
curl "http://localhost:8080/api/check?domain=example.com"

# 用户注册
curl -X POST http://localhost:8080/register \
  -d "username=test&email=test@test.com&password=123456&confirm=123456"

# 用户登录
curl -X POST http://localhost:8080/login \
  -d "username=test&password=123456"
```

## 当前状态

**v0.1.0 — 基础骨架已完成**

- [x] 用户注册/登录/退出
- [x] 首页域名查询
- [x] 用户控制台（域名 CRUD）
- [x] 域名路由分发（页面渲染 / 301 跳转）
- [x] 访问日志记录（SQLite 简易版）
- [x] Tailwind CSS + Lucide Icons UI
- [x] 毛玻璃效果 + 渐变主题

## 下一步

详见 [docs/TODO.md](docs/TODO.md)

## 从 PHP 项目移植的功能

详见 [docs/FEATURES.md](docs/FEATURES.md)

## License

Private — 仅供内部使用
