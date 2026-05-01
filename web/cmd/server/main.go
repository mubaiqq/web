package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ── Models ──

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Email     string    `json:"email" gorm:"uniqueIndex;size:128;not null"`
	Password  string    `json:"-" gorm:"size:128;not null"`
	Nickname  string    `json:"nickname" gorm:"size:64"`
	Role      string    `json:"role" gorm:"size:20;not null;default:user"` // admin / user
	CreatedAt time.Time `json:"created_at"`
}

type Domain struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       uint      `json:"user_id" gorm:"index"`
	Hostname     string    `json:"hostname" gorm:"uniqueIndex;size:255;not null"`
	Mode         string    `json:"mode" gorm:"size:20;not null;default:page"`
	Target       string    `json:"target" gorm:"size:512"`
	Template     string    `json:"template" gorm:"size:100"`
	Title        string    `json:"title" gorm:"size:255"`
	Content      string    `json:"content" gorm:"type:text"`
	HTML         string    `json:"html" gorm:"type:text"`
	RedirectType string    `json:"redirect_type" gorm:"size:20;default:301"` // 301 / 302 / meta
	Styles       string    `json:"styles" gorm:"type:text"`
	Status       int       `json:"status" gorm:"default:1"` // 0=禁用 1=启用
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type VisitLog struct {
	ID        uint   `gorm:"primaryKey"`
	Domain    string `gorm:"index;size:255"`
	Path      string `gorm:"size:512"`
	IP        string `gorm:"size:64"`
	UA        string `gorm:"size:512"`
	Referer   string `gorm:"size:512"`
	Status    int
	CreatedAt int64 `gorm:"autoCreateTime"`
}

type SiteConfig struct {
	ID              uint   `gorm:"primaryKey"`
	SiteTitle       string `gorm:"size:255;default:DomainOS"`
	FooterText      string `gorm:"size:512"`
	AdminUsername   string `gorm:"size:64"`
	AdminPassword   string `gorm:"size:128"`
	BackgroundImage string `gorm:"size:512"`
	TrackingCode    string `gorm:"type:text"`
}

// ── DB Init ──

func initDB(path string) *gorm.DB {
	os.MkdirAll(filepath.Dir(path), 0755)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	db.AutoMigrate(&User{}, &Domain{}, &VisitLog{}, &SiteConfig{})

	sqlDB, _ := db.DB()
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec("PRAGMA synchronous=NORMAL")

	// 初始化站点配置
	var count int64
	db.Model(&SiteConfig{}).Count(&count)
	if count == 0 {
		db.Create(&SiteConfig{
			SiteTitle:    "DomainOS",
			FooterText:   "© 2026 DomainOS — 多域名建站平台",
			AdminUsername: "admin",
			AdminPassword: "", // 空表示未设置
		})
	}

	// 创建默认管理员（如果不存在）
	var adminCount int64
	db.Model(&User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		db.Create(&User{
			Username: "admin",
			Email:    "admin@domainos.local",
			Password: string(hash),
			Nickname: "管理员",
			Role:     "admin",
		})
		log.Println("📌 默认管理员已创建: admin / admin123")
	}

	return db
}

// ── Template Funcs ──

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"slice": func(s string, start, end int) string {
			runes := []rune(s)
			if start >= len(runes) {
				return ""
			}
			if end > len(runes) {
				end = len(runes)
			}
			return string(runes[start:end])
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"seq": func(start, end int) []int {
			var s []int
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.Format("2006-01-02 15:04")
		},
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.Format("2006-01-02")
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"statusText": func(s int) string {
			if s == 1 {
				return "启用"
			}
			return "禁用"
		},
		"statusClass": func(s int) string {
			if s == 1 {
				return "emerald"
			}
			return "red"
		},
		"roleText": func(r string) string {
			if r == "admin" {
				return "管理员"
			}
			return "用户"
		},
		"roleClass": func(r string) string {
			if r == "admin" {
				return "amber"
			}
			return "gray"
		},
	}
}

// ── Session helpers ──

func requireAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Cookie("session")
		if uid == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		var user User
		if err := db.First(&user, uid).Error; err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set("user_id", user.ID)
		c.Set("user", &user)
		c.Next()
	}
}

func requireAdmin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Cookie("session")
		if uid == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		var user User
		if err := db.First(&user, uid).Error; err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		if user.Role != "admin" {
			c.HTML(http.StatusForbidden, "403.html", gin.H{"Message": "权限不足"})
			c.Abort()
			return
		}
		c.Set("user_id", user.ID)
		c.Set("user", &user)
		c.Next()
	}
}

func getSiteConfig(db *gorm.DB) SiteConfig {
	var cfg SiteConfig
	db.First(&cfg)
	return cfg
}

// ── Main ──

func main() {
	db := initDB("./data/platform.db")

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 加载模板
	tmpl := template.Must(
		template.New("").Funcs(templateFuncs()).ParseGlob("templates/*.html"),
	)
	r.SetHTMLTemplate(tmpl)

	// 静态文件
	r.Static("/static", "./static")

	// ── 公开页面 ──
	r.GET("/", func(c *gin.Context) {
		uid, _ := c.Cookie("session")
		var user *User
		if uid != "" {
			var u User
			if db.First(&u, uid).Error == nil {
				user = &u
			}
		}
		c.HTML(http.StatusOK, "index.html", gin.H{
			"User":       user,
			"SiteConfig": getSiteConfig(db),
		})
	})

	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{"Error": ""})
	})

	r.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		email := c.PostForm("email")
		password := c.PostForm("password")
		confirm := c.PostForm("confirm")

		if username == "" || email == "" || password == "" {
			c.HTML(http.StatusOK, "register.html", gin.H{"Error": "所有字段必填"})
			return
		}
		if password != confirm {
			c.HTML(http.StatusOK, "register.html", gin.H{"Error": "两次密码不一致"})
			return
		}
		if len(password) < 6 {
			c.HTML(http.StatusOK, "register.html", gin.H{"Error": "密码至少6位"})
			return
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		user := User{
			Username: username,
			Email:    email,
			Password: string(hash),
			Nickname: username,
			Role:     "user",
		}
		if err := db.Create(&user).Error; err != nil {
			c.HTML(http.StatusOK, "register.html", gin.H{"Error": "用户名或邮箱已存在"})
			return
		}

		c.SetCookie("session", fmt.Sprintf("%d", user.ID), 86400*30, "/", "", false, false)
		c.Redirect(http.StatusFound, "/dashboard")
	})

	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"Error": ""})
	})

	r.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		var user User
		if err := db.Where("username = ? OR email = ?", username, username).First(&user).Error; err != nil {
			c.HTML(http.StatusOK, "login.html", gin.H{"Error": "用户不存在"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			c.HTML(http.StatusOK, "login.html", gin.H{"Error": "密码错误"})
			return
		}

		c.SetCookie("session", fmt.Sprintf("%d", user.ID), 86400*30, "/", "", false, false)
		if user.Role == "admin" {
			c.Redirect(http.StatusFound, "/admin")
		} else {
			c.Redirect(http.StatusFound, "/dashboard")
		}
	})

	r.GET("/logout", func(c *gin.Context) {
		c.SetCookie("session", "", -1, "/", "", false, false)
		c.Redirect(http.StatusFound, "/")
	})

	// ── API ──
	r.GET("/api/check", func(c *gin.Context) {
		domain := c.Query("domain")
		if domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "domain required"})
			return
		}
		var d Domain
		if err := db.Where("hostname = ?", domain).First(&d).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"exists": false})
			return
		}
		var owner string
		var u User
		if db.First(&u, d.UserID).Error == nil {
			owner = u.Nickname
		}
		c.JSON(http.StatusOK, gin.H{
			"exists": true,
			"mode":   d.Mode,
			"owner":  owner,
		})
	})

	// ── 用户后台 ──
	auth := r.Group("/", requireAuth(db))
	{
		auth.GET("/dashboard", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var domains []Domain
			db.Where("user_id = ?", user.ID).Order("id DESC").Find(&domains)

			var visitCount int64
			var domainHostnames []string
			for _, d := range domains {
				domainHostnames = append(domainHostnames, d.Hostname)
			}
			if len(domainHostnames) > 0 {
				db.Model(&VisitLog{}).Where("domain IN ?", domainHostnames).Count(&visitCount)
			}

			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"User":       user,
				"Domains":    domains,
				"VisitCount": visitCount,
				"SiteConfig": getSiteConfig(db),
			})
		})

		auth.POST("/domains", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			hostname := c.PostForm("hostname")
			mode := c.PostForm("mode")
			target := c.PostForm("target")
			title := c.PostForm("title")

			if hostname == "" || mode == "" {
				c.Redirect(http.StatusFound, "/dashboard")
				return
			}

			d := Domain{
				UserID:       user.ID,
				Hostname:     hostname,
				Mode:         mode,
				Target:       target,
				Title:        title,
				Template:     "default",
				RedirectType: "301",
				Status:       1,
			}
			if err := db.Create(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/dashboard?error=域名已存在")
				return
			}
			c.Redirect(http.StatusFound, "/dashboard")
		})

		auth.POST("/domains/:id/delete", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).Delete(&Domain{})
			c.Redirect(http.StatusFound, "/dashboard")
		})

		// 用户修改密码
		auth.GET("/settings", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			c.HTML(http.StatusOK, "user_settings.html", gin.H{
				"User":       user,
				"SiteConfig": getSiteConfig(db),
				"Success":    "",
				"Error":      "",
			})
		})

		auth.POST("/settings/password", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			oldPwd := c.PostForm("old_password")
			newPwd := c.PostForm("new_password")
			confirm := c.PostForm("confirm")

			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
				c.HTML(http.StatusOK, "user_settings.html", gin.H{
					"User":       user,
					"SiteConfig": getSiteConfig(db),
					"Error":      "原密码错误",
					"Success":    "",
				})
				return
			}
			if len(newPwd) < 6 {
				c.HTML(http.StatusOK, "user_settings.html", gin.H{
					"User":       user,
					"SiteConfig": getSiteConfig(db),
					"Error":      "新密码至少6位",
					"Success":    "",
				})
				return
			}
			if newPwd != confirm {
				c.HTML(http.StatusOK, "user_settings.html", gin.H{
					"User":       user,
					"SiteConfig": getSiteConfig(db),
					"Error":      "两次密码不一致",
					"Success":    "",
				})
				return
			}

			hash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
			db.Model(&user).Update("password", string(hash))
			c.HTML(http.StatusOK, "user_settings.html", gin.H{
				"User":       user,
				"SiteConfig": getSiteConfig(db),
				"Success":    "密码修改成功",
				"Error":      "",
			})
		})
	}

	// ── 管理员后台 ──
	admin := r.Group("/admin", requireAdmin(db))
	{
		// 管理仪表盘
		admin.GET("", func(c *gin.Context) {
			user := c.MustGet("user").(*User)

			var userCount, domainCount, visitCount int64
			db.Model(&User{}).Count(&userCount)
			db.Model(&Domain{}).Count(&domainCount)
			db.Model(&VisitLog{}).Count(&visitCount)

			var activeDomains int64
			db.Model(&Domain{}).Where("status = 1").Count(&activeDomains)

			var recentUsers []User
			db.Order("id DESC").Limit(5).Find(&recentUsers)

			var recentDomains []Domain
			db.Order("id DESC").Limit(5).Find(&recentDomains)

			// 今日访问
			var todayVisits int64
			todayStart := time.Now().Truncate(24 * time.Hour).Unix()
			db.Model(&VisitLog{}).Where("created_at >= ?", todayStart).Count(&todayVisits)

			c.HTML(http.StatusOK, "admin_dashboard.html", gin.H{
				"User":          user,
				"SiteConfig":    getSiteConfig(db),
				"UserCount":     userCount,
				"DomainCount":   domainCount,
				"ActiveDomains": activeDomains,
				"VisitCount":    visitCount,
				"TodayVisits":   todayVisits,
				"RecentUsers":   recentUsers,
				"RecentDomains": recentDomains,
				"Page":          "dashboard",
			})
		})

		// ── 域名管理 ──
		admin.GET("/domains", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			search := c.Query("search")
			statusFilter := c.Query("status")
			if page < 1 {
				page = 1
			}
			pageSize := 20

			query := db.Model(&Domain{})
			if search != "" {
				query = query.Where("hostname LIKE ? OR title LIKE ?", "%"+search+"%", "%"+search+"%")
			}
			if statusFilter != "" {
				query = query.Where("status = ?", statusFilter)
			}

			var total int64
			query.Count(&total)

			var domains []Domain
			query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&domains)

			// 构建 userID -> username 映射
			userMap := make(map[uint]string)
			for _, d := range domains {
				if _, ok := userMap[d.UserID]; !ok {
					var u User
					if db.First(&u, d.UserID).Error == nil {
						userMap[d.UserID] = u.Username
					}
				}
			}

			totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

			c.HTML(http.StatusOK, "admin_domains.html", gin.H{
				"User":         user,
				"SiteConfig":   getSiteConfig(db),
				"Domains":      domains,
				"UserMap":      userMap,
				"Total":        total,
				"Page":         page,
				"TotalPages":   totalPages,
				"Search":       search,
				"StatusFilter": statusFilter,
				"PageName":     "domains",
			})
		})

		// 添加域名
		admin.POST("/domains", func(c *gin.Context) {
			hostname := c.PostForm("hostname")
			mode := c.DefaultPostForm("mode", "page")
			target := c.PostForm("target")
			title := c.PostForm("title")
			content := c.PostForm("content")
			userID, _ := strconv.Atoi(c.PostForm("user_id"))
			redirectType := c.DefaultPostForm("redirect_type", "301")

			if hostname == "" {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名不能为空")
				return
			}

			d := Domain{
				UserID:       uint(userID),
				Hostname:     hostname,
				Mode:         mode,
				Target:       target,
				Title:        title,
				Content:      content,
				Template:     "default",
				RedirectType: redirectType,
				Status:       1,
			}
			if err := db.Create(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名已存在")
				return
			}
			c.Redirect(http.StatusFound, "/admin/domains?success=域名添加成功")
		})

		// 编辑域名
		admin.POST("/domains/:id/edit", func(c *gin.Context) {
			id := c.Param("id")
			var d Domain
			if err := db.First(&d, id).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名不存在")
				return
			}

			hostname := c.PostForm("hostname")
			mode := c.PostForm("mode")
			target := c.PostForm("target")
			title := c.PostForm("title")
			content := c.PostForm("content")
			redirectType := c.PostForm("redirect_type")
			userID, _ := strconv.Atoi(c.PostForm("user_id"))

			updates := map[string]interface{}{
				"hostname":      hostname,
				"mode":          mode,
				"target":        target,
				"title":         title,
				"content":       content,
				"redirect_type": redirectType,
				"user_id":       uint(userID),
			}
			db.Model(&d).Updates(updates)
			c.Redirect(http.StatusFound, "/admin/domains?success=域名更新成功")
		})

		// 删除域名
		admin.POST("/domains/:id/delete", func(c *gin.Context) {
			db.Delete(&Domain{}, c.Param("id"))
			c.Redirect(http.StatusFound, "/admin/domains?success=域名已删除")
		})

		// 切换域名状态
		admin.POST("/domains/:id/toggle", func(c *gin.Context) {
			var d Domain
			if err := db.First(&d, c.Param("id")).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名不存在")
				return
			}
			newStatus := 1
			if d.Status == 1 {
				newStatus = 0
			}
			db.Model(&d).Update("status", newStatus)
			c.Redirect(http.StatusFound, "/admin/domains?success=状态已更新")
		})

		// 批量删除域名
		admin.POST("/domains/batch-delete", func(c *gin.Context) {
			ids := c.PostFormArray("ids")
			if len(ids) > 0 {
				db.Delete(&Domain{}, ids)
			}
			c.Redirect(http.StatusFound, "/admin/domains?success=批量删除完成")
		})

		// ── 用户管理 ──
		admin.GET("/users", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			search := c.Query("search")
			if page < 1 {
				page = 1
			}
			pageSize := 20

			query := db.Model(&User{})
			if search != "" {
				query = query.Where("username LIKE ? OR email LIKE ? OR nickname LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
			}

			var total int64
			query.Count(&total)

			var users []User
			query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)

			// 每个用户的域名数
			type UserWithDomains struct {
				User
				DomainCount int64
			}
			var usersWithDomains []UserWithDomains
			for _, u := range users {
				var dc int64
				db.Model(&Domain{}).Where("user_id = ?", u.ID).Count(&dc)
				usersWithDomains = append(usersWithDomains, UserWithDomains{User: u, DomainCount: dc})
			}

			totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

			c.HTML(http.StatusOK, "admin_users.html", gin.H{
				"User":       user,
				"SiteConfig": getSiteConfig(db),
				"Users":      usersWithDomains,
				"Total":      total,
				"Page":       page,
				"TotalPages": totalPages,
				"Search":     search,
				"PageName":   "users",
			})
		})

		// 添加用户
		admin.POST("/users", func(c *gin.Context) {
			username := c.PostForm("username")
			email := c.PostForm("email")
			password := c.PostForm("password")
			role := c.DefaultPostForm("role", "user")

			if username == "" || email == "" || password == "" {
				c.Redirect(http.StatusFound, "/admin/users?error=所有字段必填")
				return
			}

			hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			u := User{
				Username: username,
				Email:    email,
				Password: string(hash),
				Nickname: username,
				Role:     role,
			}
			if err := db.Create(&u).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/users?error=用户名或邮箱已存在")
				return
			}
			c.Redirect(http.StatusFound, "/admin/users?success=用户创建成功")
		})

		// 编辑用户
		admin.POST("/users/:id/edit", func(c *gin.Context) {
			id := c.Param("id")
			var u User
			if err := db.First(&u, id).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/users?error=用户不存在")
				return
			}

			nickname := c.PostForm("nickname")
			email := c.PostForm("email")
			role := c.PostForm("role")

			updates := map[string]interface{}{
				"nickname": nickname,
				"email":    email,
				"role":     role,
			}
			db.Model(&u).Updates(updates)
			c.Redirect(http.StatusFound, "/admin/users?success=用户更新成功")
		})

		// 删除用户
		admin.POST("/users/:id/delete", func(c *gin.Context) {
			id := c.Param("id")
			// 同时删除该用户的域名
			db.Where("user_id = ?", id).Delete(&Domain{})
			db.Delete(&User{}, id)
			c.Redirect(http.StatusFound, "/admin/users?success=用户已删除")
		})

		// 修改用户密码
		admin.POST("/users/:id/password", func(c *gin.Context) {
			id := c.Param("id")
			newPwd := c.PostForm("new_password")
			if len(newPwd) < 6 {
				c.Redirect(http.StatusFound, "/admin/users?error=密码至少6位")
				return
			}
			hash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
			db.Model(&User{}).Where("id = ?", id).Update("password", string(hash))
			c.Redirect(http.StatusFound, "/admin/users?success=密码已重置")
		})

		// ── 站点设置 ──
		admin.GET("/settings", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			cfg := getSiteConfig(db)
			c.HTML(http.StatusOK, "admin_settings.html", gin.H{
				"User":       user,
				"SiteConfig": cfg,
				"Success":    "",
				"Error":      "",
				"PageName":   "settings",
			})
		})

		admin.POST("/settings", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			cfg := getSiteConfig(db)

			cfg.SiteTitle = c.PostForm("site_title")
			cfg.FooterText = c.PostForm("footer_text")
			cfg.BackgroundImage = c.PostForm("background_image")
			cfg.TrackingCode = c.PostForm("tracking_code")

			db.Model(&cfg).Updates(map[string]interface{}{
				"site_title":       cfg.SiteTitle,
				"footer_text":      cfg.FooterText,
				"background_image": cfg.BackgroundImage,
				"tracking_code":    cfg.TrackingCode,
			})

			c.HTML(http.StatusOK, "admin_settings.html", gin.H{
				"User":       user,
				"SiteConfig": cfg,
				"Success":    "设置已保存",
				"Error":      "",
				"PageName":   "settings",
			})
		})

		// 修改管理员密码
		admin.POST("/settings/password", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			cfg := getSiteConfig(db)
			oldPwd := c.PostForm("old_password")
			newPwd := c.PostForm("new_password")
			confirm := c.PostForm("confirm")

			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
				c.HTML(http.StatusOK, "admin_settings.html", gin.H{
					"User":       user,
					"SiteConfig": cfg,
					"Error":      "原密码错误",
					"Success":    "",
					"PageName":   "settings",
				})
				return
			}
			if len(newPwd) < 6 {
				c.HTML(http.StatusOK, "admin_settings.html", gin.H{
					"User":       user,
					"SiteConfig": cfg,
					"Error":      "新密码至少6位",
					"Success":    "",
					"PageName":   "settings",
				})
				return
			}
			if newPwd != confirm {
				c.HTML(http.StatusOK, "admin_settings.html", gin.H{
					"User":       user,
					"SiteConfig": cfg,
					"Error":      "两次密码不一致",
					"Success":    "",
					"PageName":   "settings",
				})
				return
			}

			hash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
			db.Model(&user).Update("password", string(hash))
			c.HTML(http.StatusOK, "admin_settings.html", gin.H{
				"User":       user,
				"SiteConfig": cfg,
				"Success":    "密码修改成功",
				"Error":      "",
				"PageName":   "settings",
			})
		})

		// 用户名自动补全 API
		admin.GET("/api/users/autocomplete", func(c *gin.Context) {
			q := c.Query("q")
			var users []User
			db.Where("username LIKE ?", "%"+q+"%").Limit(10).Select("id", "username", "nickname").Find(&users)
			c.JSON(http.StatusOK, users)
		})
	}

	// ── 域名解析路由 ──
	r.NoRoute(func(c *gin.Context) {
		host := c.Request.Host
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}

		var d Domain
		if err := db.Where("hostname = ? AND status = 1", host).First(&d).Error; err != nil {
			c.HTML(http.StatusNotFound, "404.html", gin.H{"Host": host})
			return
		}

		// 记录访问（异步）
		go func() {
			db.Create(&VisitLog{
				Domain:  host,
				Path:    c.Request.URL.Path,
				IP:      c.ClientIP(),
				UA:      c.Request.UserAgent(),
				Referer: c.Request.Referer(),
				Status:  200,
			})
		}()

		switch d.Mode {
		case "redirect":
			target := d.Target
			if !strings.HasPrefix(target, "http") {
				target = "https://" + target
			}
			switch d.RedirectType {
			case "302":
				c.Redirect(http.StatusFound, target)
			default:
				c.Redirect(http.StatusMovedPermanently, target)
			}
		case "page":
			c.HTML(http.StatusOK, "site.html", gin.H{
				"Domain":  d,
				"Title":   d.Title,
				"Content": d.Content,
				"Host":    host,
			})
		default:
			c.String(http.StatusBadRequest, "unknown mode")
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 http://localhost:%s", port)
	r.Run(":" + port)
}
