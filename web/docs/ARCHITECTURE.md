# 架构设计

## 系统架构

```
                    ┌─────────────────┐
                    │   用户浏览器     │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Caddy / Nginx  │  HTTPS 终止 + 反向代理
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   Go (Gin)      │  单进程 HTTP 服务
                    │                 │
                    │  ├─ 域名路由    │  根据 Host 分发
                    │  ├─ 管理 API    │  /_admin/*
                    │  ├─ 用户 API    │  /api/*
                    │  └─ 页面渲染    │  模板 + 数据
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼──────┐ ┌────▼────┐ ┌───────▼───────┐
     │    SQLite     │ │  Redis  │ │  ClickHouse   │
     │  (业务数据)   │ │ (缓冲)  │ │  (访问日志)   │
     │               │ │         │ │               │
     │  users        │ │ 队列:   │ │  visits       │
     │  domains      │ │ visits  │ │  (90天TTL)    │
     │  templates    │ │ :queue  │ │               │
     │  site_config  │ │         │ │               │
     └───────────────┘ └─────────┘ └───────────────┘
```

## 请求流程

### 1. 域名访问流程
```
用户访问 example.com
    │
    ├─ Go NoRoute 匹配
    │   ├─ 查 SQLite: domains WHERE domain = 'example.com'
    │   │   ├─ 找到 → 根据 template 字段渲染对应模板
    │   │   │   ├─ mode=page → 渲染模板页面
    │   │   │   ├─ mode=redirect → 301 跳转
    │   │   │   └─ mode=html → 输出自定义 HTML
    │   │   └─ 未找到 → 显示 404 页面
    │   │
    │   └─ 异步记录访问日志
    │       ├─ 写入 Redis 队列（当前直接写 SQLite）
    │       └─ 后台协程批量写入 ClickHouse
    │
    └─ 返回响应
```

### 2. 用户操作流程
```
用户注册/登录
    │
    ├─ 注册: POST /register
    │   ├─ 校验输入
    │   ├─ bcrypt 加密密码
    │   ├─ 写入 users 表
    │   └─ 设置 cookie → 跳转 dashboard
    │
    ├─ 登录: POST /login
    │   ├─ 查 users 表
    │   ├─ bcrypt 验证密码
    │   └─ 设置 cookie → 跳转 dashboard
    │
    └─ 控制台: GET /dashboard
        ├─ requireAuth 中间件验证
        ├─ 查询用户域名列表
        └─ 渲染 dashboard.html
```

## 目录结构

```
domain-platform/
├── cmd/
│   └── server/
│       └── main.go              # 入口 + 路由 + 配置
├── internal/                    # （规划中，当前代码全在 main.go）
│   ├── handler/                 # HTTP 处理器
│   │   ├── auth.go              # 注册/登录/退出
│   │   ├── admin.go             # 管理员 API
│   │   ├── domain.go            # 域名 CRUD
│   │   ├── template.go          # 模板管理
│   │   └── stats.go             # 统计查询
│   ├── middleware/               # 中间件
│   │   ├── auth.go              # 登录验证
│   │   ├── admin.go             # 管理员权限
│   │   └── visit.go             # 访问日志
│   ├── model/                   # 数据模型
│   │   ├── user.go
│   │   ├── domain.go
│   │   ├── template.go
│   │   └── visit.go
│   ├── repository/              # 数据访问层
│   │   ├── sqlite.go
│   │   ├── user_repo.go
│   │   ├── domain_repo.go
│   │   └── template_repo.go
│   └── service/                 # 业务逻辑层
│       ├── auth.go
│       ├── domain.go
│       └── stats.go
├── templates/                   # HTML 模板
├── static/                      # 静态资源
├── migrations/                  # 数据库迁移脚本
├── docs/                        # 项目文档
├── docker-compose.yml
├── Makefile
└── README.md
```

## 数据模型（当前 SQLite）

### users
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增 ID |
| username | TEXT UNIQUE | 用户名 |
| email | TEXT UNIQUE | 邮箱 |
| password | TEXT | bcrypt 哈希 |
| nickname | TEXT | 昵称 |
| role | TEXT | admin / user |
| templates | TEXT | 可用模板列表（JSON） |
| created_at | DATETIME | 创建时间 |

### domains
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增 ID |
| user_id | INTEGER | 所属用户 |
| hostname | TEXT UNIQUE | 绑定域名 |
| mode | TEXT | page / redirect |
| template | TEXT | 模板名 |
| title | TEXT | 页面标题 |
| content | TEXT | 页面内容 |
| html | TEXT | 自定义 HTML |
| target | TEXT | 跳转目标 URL |
| redirect_type | TEXT | 301 / 302 / meta |
| styles | TEXT | 自定义样式 |
| state | INTEGER | 0=禁用 1=启用 |
| created_at | DATETIME | 创建时间 |

### visit_logs
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增 ID |
| domain | TEXT | 访问域名 |
| path | TEXT | 访问路径 |
| ip | TEXT | 客户端 IP |
| ua | TEXT | User-Agent |
| referer | TEXT | 来源页 |
| status | INTEGER | HTTP 状态码 |
| created_at | INTEGER | 时间戳 |

## 安全设计

- 密码使用 bcrypt 加密存储
- Cookie session 设置 HttpOnly
- 管理员接口独立路由组，需 admin 角色
- SQL 参数化查询，防注入
- 输入校验 + 长度限制
