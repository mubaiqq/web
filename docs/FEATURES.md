# 功能清单 — 从 PHP 项目移植

本文档记录原始 PHP 项目（木白云服务）的完整功能，作为 DomainOS Go 项目的开发参考。

---

## 一、首页 (`index.php` / `templates/home.php`)

### 功能
- **域名可用性查询**：输入域名，查询是否可注册
- **查询结果卡片**：显示域名、状态、价格、详情
- **登录/注册入口**：顶部导航跳转用户后台

### 当前 Go 项目状态
- [x] 域名查询（查数据库中是否已绑定）
- [ ] 域名可用性查询（调用 WHOIS / 注册商 API）
- [ ] 显示域名价格信息
- [ ] 查询结果卡片优化

---

## 二、管理员后台 (`/ht`)

### 2.1 仪表盘
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 用户总数统计 | `hom.php` | [ ] |
| 域名总数统计 | `hom.php` | [ ] |
| 总访问量统计 | `hom.php` | [ ] |
| 最近注册用户 | `welcome-1.php` | [ ] |
| 最近添加域名 | `welcome-1.php` | [ ] |

### 2.2 域名管理
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 域名列表（分页、搜索） | `ymlist.php` | [ ] |
| 显示域名所属用户 | `ymlist.php` | [ ] |
| 显示域名状态（启用/禁用） | `ymlist.php` | [ ] |
| 显示域名模板 | `ymlist.php` | [ ] |
| 添加域名（指定用户） | `add_domain.php` | [ ] |
| 批量添加域名 | `addt_domain.php` | [ ] |
| 编辑域名（修改跳转地址） | `edit_domain.php` | [ ] |
| 保存编辑 | `save_edit.php` | [ ] |
| 删除域名 | `delete_domain.php` | [ ] |
| 域名处理（启用/禁用切换） | `ymcl.php` | [ ] |
| 域名清理（批量删除） | `ql.php` | [ ] |

### 2.3 用户管理
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 用户列表（搜索、分页） | `user_list.php` | [ ] |
| 注册新用户 | `register.php` | [ ] |
| 添加用户 | `tjyh.php` | [ ] |
| 查看用户域名 | `user_domains.php` | [ ] |
| 用户名自动补全 | `autocomplete_user.php` | [ ] |
| 修改用户密码 | `czmm.php` | [ ] |
| 编辑用户权限 | `edu.php` | [ ] |

### 2.4 模板管理
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 模板列表 | `template.php` | [ ] |
| 添加模板 | `template.php` | [ ] |
| 删除模板 | `template.php` | [ ] |
| 用户模板分配 | `usertmp.php` | [ ] |
| 域名模板设置 | `tmp.php` | [ ] |

### 2.5 站点设置
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 站点标题 | `site.php` | [ ] |
| 页脚文字 | `site.php` | [ ] |
| 管理员账号密码 | `site.php` | [ ] |
| 背景图片 URL | `site.php` | [ ] |
| 统计代码（51.la 等） | `site.php` | [ ] |

---

## 三、用户后台 (`/u1`)

### 3.1 控制台
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 用户信息展示 | `user.php` | [x] |
| 域名数量统计 | `user.php` | [x] |
| 联系客服入口 | `nav.php` | [ ] |

### 3.2 域名管理
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 我的域名列表 | `url_list.php` / `myym.php` | [x] |
| 提交域名 | `submit_domain.php` | [x] |
| 编辑域名（标题、内容） | `edit_domain.php` | [ ] |
| 域名详情 | `domain_details.php` | [ ] |
| 保存编辑 | `save_edit.php` | [ ] |
| 删除域名 | - | [x] |

### 3.3 模板系统
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 浏览模板列表 | `mbsd.php` | [ ] |
| 模板预览 | `mbsd.php` | [ ] |
| 选择模板应用到域名 | `mbsd.php` | [ ] |
| 更换应用 | `ygyy.php` | [ ] |
| 自由模式 | `ziyou.php` | [ ] |

### 3.4 统计分析
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 域名访问统计 | `tj.php` | [ ] |
| 统计大图 | `tjdt.php` | [ ] |
| 中国地图访问分布 | `dt.php` | [ ] |
| 统计链接生成 | `statsLink.php` | [ ] |

### 3.5 内容编辑
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 代码编辑器（HTML） | `代码编辑器.php` | [ ] |
| 文档编辑器 | `文档编辑.php` | [ ] |
| 文档编辑器 2.0 | `文档编辑2.0.php` | [ ] |
| 图片上传 | `img.php` / `upload.php` | [ ] |

### 3.6 账号管理
| 功能 | 原文件 | Go 状态 |
|------|--------|---------|
| 修改密码 | `xgmm.php` | [ ] |
| 退出登录 | `logout.php` | [x] |
| 社交账号登录（QQ/微信） | `dl1.php` / `dl2.php` | [ ] |

---

## 四、模板系统

### 可用模板
| 模板 | 文件 | 说明 |
|------|------|------|
| 默认首页 | `templates/home.php` | 域名查询 + 结果展示 |
| 导航页 | `templates/导航.php` | 多链接导航页 |
| 域名跳转 | `templates/域名跳转.php` | 301/302 跳转 |
| 域名跳转 HTML | `templates/域名跳转html` | HTML meta 跳转 |
| 原神主题 | `templates/原神.php` | 游戏主题页面 |
| 元梦之星 | `templates/元梦之星.php` | 游戏主题页面 |
| 文档编辑 | `templates/文档编辑.php` | 在线文档 |
| 代码编辑器 | `templates/代码编辑器.php` | 在线代码编辑 |

### 模板数据字段
每个域名可配置：
- `title` — 页面标题
- `content` — 页面内容
- `template` — 使用的模板名
- `tz` — 跳转目标 URL
- `tzlx` — 跳转类型（301/302/meta）
- `ys` / `ys2` — 自定义样式
- `ym` / `ym1` — 自定义域名参数
- `stats` — 统计代码开关
- `state` — 状态（0=禁用, 1=启用）
- `html` — 自定义 HTML（代码编辑器保存的内容）

---

## 五、数据库结构（参考）

### users 表
```sql
CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(128) UNIQUE,
    password VARCHAR(128) NOT NULL,
    templates VARCHAR(128),     -- 用户可用模板
    ziyou INT DEFAULT 0,        -- 自由模式权限
    qquid VARCHAR(64),          -- QQ 登录 UID
    wxuid VARCHAR(64),          -- 微信登录 UID
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### domains 表
```sql
CREATE TABLE domains (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    domain VARCHAR(255) UNIQUE NOT NULL,
    template VARCHAR(128),      -- 使用的模板
    templatesa VARCHAR(128),    -- 备用模板
    title VARCHAR(255),
    content TEXT,
    html TEXT,                  -- 自定义 HTML
    tz VARCHAR(512),            -- 跳转目标
    tzlx VARCHAR(20),           -- 跳转类型
    ys VARCHAR(255),            -- 自定义样式
    ys2 VARCHAR(255),
    ym VARCHAR(255),
    ym1 VARCHAR(255),
    stats INT DEFAULT 0,        -- 统计开关
    state INT DEFAULT 0,        -- 状态 0/1
    source VARCHAR(64),         -- 来源
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### template 表
```sql
CREATE TABLE template (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    img VARCHAR(255),
    ms TEXT,
    jg VARCHAR(64),
    yl VARCHAR(255)
);
```

### IPStats 表
```sql
CREATE TABLE IPStats (
    id INT PRIMARY KEY AUTO_INCREMENT,
    domain VARCHAR(255),
    ip VARCHAR(64),
    visit_date DATE,
    visit_count INT DEFAULT 1
);
```

### dhgg 表（公告）
```sql
CREATE TABLE dhgg (
    id INT PRIMARY KEY AUTO_INCREMENT,
    domain_id INT,
    content TEXT
);
```

### dhapp 表（应用）
```sql
CREATE TABLE dhapp (
    id INT PRIMARY KEY AUTO_INCREMENT,
    domain_id INT,
    name VARCHAR(128),
    url VARCHAR(512),
    icon VARCHAR(255)
);
```

### dhhb 表（海报）
```sql
CREATE TABLE dhhb (
    id INT PRIMARY KEY AUTO_INCREMENT,
    domain_id INT,
    poster_url VARCHAR(512)
);
```

### mubai 表（站点配置）
```sql
CREATE TABLE mubai (
    id INT PRIMARY KEY AUTO_INCREMENT,
    site_title VARCHAR(255),
    footer_text TEXT,
    admin_username VARCHAR(64),
    admin_password VARCHAR(128),
    background_image_url VARCHAR(512),
    tracking_code TEXT
);
```
