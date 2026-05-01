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
	Role      string    `json:"role" gorm:"size:20;default:user;not null"` // user | admin
	Status    int       `json:"status" gorm:"default:1"`                   // 1=active 0=disabled
	CreatedAt time.Time `json:"created_at"`
}

type Domain struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Hostname  string    `json:"hostname" gorm:"uniqueIndex;size:255;not null"`
	Mode      string    `json:"mode" gorm:"size:20;not null;default:page"`
	Target    string    `json:"target" gorm:"size:512"`
	Template  string    `json:"template" gorm:"size:100"`
	Title     string    `json:"title" gorm:"size:255"`
	Content   string    `json:"content" gorm:"type:text"`
	Status    int       `json:"status" gorm:"default:1"` // 1=active 0=disabled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

// ── DB Init ──

func initDB(path string) *gorm.DB {
	os.MkdirAll(filepath.Dir(path), 0755)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	db.AutoMigrate(&User{}, &Domain{}, &VisitLog{})

	sqlDB, _ := db.DB()
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec("PRAGMA synchronous=NORMAL")
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
		"itoa": func(i int) string {
			return strconv.Itoa(i)
		},
		"uintToString": func(i uint) string {
			return strconv.FormatUint(uint64(i), 10)
		},
	}
}

// ── Session helpers ──

func currentUser(c *gin.Context, db *gorm.DB) *User {
	uid, exists := c.Get("user_id")
	if !exists {
		return nil
	}
	var user User
	if db.First(&user, uid).Error != nil {
		return nil
	}
	return &user
}

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
		user, exists := c.Get("user")
		if !exists {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		u := user.(*User)
		if u.Role != "admin" {
			c.HTML(http.StatusForbidden, "403.html", gin.H{"Message": "权限不足，需要管理员身份"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ── Seed admin ──

func seedAdmin(db *gorm.DB) {
	var count int64
	db.Model(&User{}).Where("role = ?", "admin").Count(&count)
	if count == 0 {
		// Check if any user exists, make first user admin
		var first User
		if db.Order("id ASC").First(&first).Error == nil {
			db.Model(&first).Update("role", "admin")
			log.Printf("👑 已将第一个用户 [%s] 设为管理员", first.Username)
		}
	}
}

// ── Main ──

func main() {
	db := initDB("./data/platform.db")
	seedAdmin(db)

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
			"User": user,
		})
	})

	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{
			"Error": "",
		})
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
		c.HTML(http.StatusOK, "login.html", gin.H{
			"Error": "",
		})
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

			// Announcements (hardcoded for now)
			announcements := []map[string]string{
				{"title": "欢迎使用 DomainOS v0.1.0", "desc": "平台已上线，支持域名绑定、页面渲染和智能跳转功能。", "date": "2026-05-01", "color": "indigo"},
				{"title": "功能预告", "desc": "管理员后台、模板系统、访问统计等功能正在开发中，敬请期待。", "date": "2026-05-01", "color": "emerald"},
			}

			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"User":          user,
				"Domains":       domains,
				"Announcements": announcements,
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
				UserID:   user.ID,
				Hostname: hostname,
				Mode:     mode,
				Target:   target,
				Title:    title,
				Template: "default",
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
	}

	// ── 管理员后台 ──
	admin := r.Group("/admin", requireAuth(db), requireAdmin(db))
	{
		// 管理仪表盘
		admin.GET("", func(c *gin.Context) {
			user := c.MustGet("user").(*User)

			var userCount, domainCount, visitCount int64
			db.Model(&User{}).Count(&userCount)
			db.Model(&Domain{}).Count(&domainCount)
			db.Model(&VisitLog{}).Count(&visitCount)

			var activeDomains int64
			db.Model(&Domain{}).Where("status = ?", 1).Count(&activeDomains)

			var disabledDomains int64
			db.Model(&Domain{}).Where("status = ?", 0).Count(&disabledDomains)

			var recentUsers []User
			db.Order("id DESC").Limit(10).Find(&recentUsers)

			var recentDomains []Domain
			db.Order("id DESC").Limit(10).Find(&recentDomains)

			// Enrich domains with user info
			type DomainWithUser struct {
				Domain
				OwnerName  string
				OwnerEmail string
			}
			var domainsWithUser []DomainWithUser
			for _, d := range recentDomains {
				var u User
				ownerName := "未知"
				ownerEmail := ""
				if db.First(&u, d.UserID).Error == nil {
					ownerName = u.Nickname
					ownerEmail = u.Email
				}
				domainsWithUser = append(domainsWithUser, DomainWithUser{
					Domain:     d,
					OwnerName:  ownerName,
					OwnerEmail: ownerEmail,
				})
			}

			c.HTML(http.StatusOK, "admin.html", gin.H{
				"User":            user,
				"UserCount":       userCount,
				"DomainCount":     domainCount,
				"VisitCount":      visitCount,
				"ActiveDomains":   activeDomains,
				"DisabledDomains": disabledDomains,
				"RecentUsers":     recentUsers,
				"RecentDomains":   domainsWithUser,
				"Page":            "dashboard",
			})
		})

		// 域名管理列表
		admin.GET("/domains", func(c *gin.Context) {
			user := c.MustGet("user").(*User)

			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			if page < 1 {
				page = 1
			}
			pageSize := 20
			search := c.Query("search")
			statusFilter := c.Query("status")
			modeFilter := c.Query("mode")

			query := db.Model(&Domain{})
			if search != "" {
				query = query.Where("hostname LIKE ? OR title LIKE ?", "%"+search+"%", "%"+search+"%")
			}
			if statusFilter != "" {
				query = query.Where("status = ?", statusFilter)
			}
			if modeFilter != "" {
				query = query.Where("mode = ?", modeFilter)
			}

			var total int64
			query.Count(&total)
			totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
			if totalPages < 1 {
				totalPages = 1
			}

			var domains []Domain
			query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&domains)

			type DomainWithUser struct {
				Domain
				OwnerName  string
				OwnerEmail string
			}
			var list []DomainWithUser
			for _, d := range domains {
				var u User
				ownerName := "未知"
				ownerEmail := ""
				if db.First(&u, d.UserID).Error == nil {
					ownerName = u.Nickname
					ownerEmail = u.Email
				}
				list = append(list, DomainWithUser{
					Domain:     d,
					OwnerName:  ownerName,
					OwnerEmail: ownerEmail,
				})
			}

			c.HTML(http.StatusOK, "admin-domains.html", gin.H{
				"User":       user,
				"Domains":    list,
				"Total":      total,
				"Page":       page,
				"TotalPages": totalPages,
				"Search":     search,
				"Status":     statusFilter,
				"Mode":       modeFilter,
				"PageName":   "domains",
			})
		})

		// 添加域名
		admin.POST("/domains", func(c *gin.Context) {
			hostname := c.PostForm("hostname")
			mode := c.DefaultPostForm("mode", "page")
			target := c.PostForm("target")
			title := c.PostForm("title")
			userIDStr := c.PostForm("user_id")

			if hostname == "" {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名不能为空")
				return
			}

			userID, _ := strconv.ParseUint(userIDStr, 10, 64)
			if userID == 0 {
				c.Redirect(http.StatusFound, "/admin/domains?error=请选择用户")
				return
			}

			d := Domain{
				UserID:   uint(userID),
				Hostname: hostname,
				Mode:     mode,
				Target:   target,
				Title:    title,
				Template: "default",
			}
			if err := db.Create(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名已存在")
				return
			}
			c.Redirect(http.StatusFound, "/admin/domains?success=1")
		})

		// 批量添加域名
		admin.POST("/domains/batch", func(c *gin.Context) {
			hostnames := c.PostForm("hostnames")
			mode := c.DefaultPostForm("mode", "page")
			userIDStr := c.DefaultPostForm("user_id", "0")
			title := c.PostForm("title")

			userID, _ := strconv.ParseUint(userIDStr, 10, 64)
			if userID == 0 {
				c.Redirect(http.StatusFound, "/admin/domains?error=请选择用户")
				return
			}

			lines := strings.Split(hostnames, "\n")
			added := 0
			for _, line := range lines {
				h := strings.TrimSpace(line)
				if h == "" {
					continue
				}
				d := Domain{
					UserID:   uint(userID),
					Hostname: h,
					Mode:     mode,
					Title:    title,
					Template: "default",
				}
				if db.Create(&d).Error == nil {
					added++
				}
			}
			c.Redirect(http.StatusFound, fmt.Sprintf("/admin/domains?success=批量添加成功，共 %d 个", added))
		})

		// 编辑域名
		admin.POST("/domains/:id", func(c *gin.Context) {
			id := c.Param("id")
			var d Domain
			if db.First(&d, id).Error != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名不存在")
				return
			}

			hostname := c.PostForm("hostname")
			mode := c.PostForm("mode")
			target := c.PostForm("target")
			title := c.PostForm("title")
			userIDStr := c.PostForm("user_id")
			statusStr := c.PostForm("status")

			if hostname != "" {
				d.Hostname = hostname
			}
			if mode != "" {
				d.Mode = mode
			}
			d.Target = target
			d.Title = title
			if userIDStr != "" {
				uid, _ := strconv.ParseUint(userIDStr, 10, 64)
				d.UserID = uint(uid)
			}
			if statusStr != "" {
				s, _ := strconv.Atoi(statusStr)
				d.Status = s
			}

			db.Save(&d)
			c.Redirect(http.StatusFound, "/admin/domains?success=1")
		})

		// 删除域名
		admin.POST("/domains/:id/delete", func(c *gin.Context) {
			db.Delete(&Domain{}, c.Param("id"))
			c.Redirect(http.StatusFound, "/admin/domains?success=1")
		})

		// 启用/禁用域名
		admin.POST("/domains/:id/toggle", func(c *gin.Context) {
			var d Domain
			if db.First(&d, c.Param("id")).Error != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名不存在")
				return
			}
			if d.Status == 1 {
				d.Status = 0
			} else {
				d.Status = 1
			}
			db.Save(&d)
			c.Redirect(http.StatusFound, "/admin/domains")
		})

		// 批量操作
		admin.POST("/domains/batch-action", func(c *gin.Context) {
			action := c.PostForm("action")
			ids := c.PostFormArray("ids")
			if len(ids) == 0 {
				c.Redirect(http.StatusFound, "/admin/domains?error=请选择域名")
				return
			}

			switch action {
			case "enable":
				db.Model(&Domain{}).Where("id IN ?", ids).Update("status", 1)
			case "disable":
				db.Model(&Domain{}).Where("id IN ?", ids).Update("status", 0)
			case "delete":
				db.Delete(&Domain{}, ids)
			}
			c.Redirect(http.StatusFound, "/admin/domains?success=1")
		})

		// 用户管理列表
		admin.GET("/users", func(c *gin.Context) {
			user := c.MustGet("user").(*User)

			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			if page < 1 {
				page = 1
			}
			pageSize := 20
			search := c.Query("search")

			query := db.Model(&User{})
			if search != "" {
				query = query.Where("username LIKE ? OR email LIKE ? OR nickname LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
			}

			var total int64
			query.Count(&total)
			totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
			if totalPages < 1 {
				totalPages = 1
			}

			var users []User
			query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)

			// Count domains per user
			type UserWithStats struct {
				User
				DomainCount int64
			}
			var list []UserWithStats
			for _, u := range users {
				var dc int64
				db.Model(&Domain{}).Where("user_id = ?", u.ID).Count(&dc)
				list = append(list, UserWithStats{User: u, DomainCount: dc})
			}

			c.HTML(http.StatusOK, "admin-users.html", gin.H{
				"User":       user,
				"Users":      list,
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
			c.Redirect(http.StatusFound, "/admin/users?success=1")
		})

		// 编辑用户
		admin.POST("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			var u User
			if db.First(&u, id).Error != nil {
				c.Redirect(http.StatusFound, "/admin/users?error=用户不存在")
				return
			}

			nickname := c.PostForm("nickname")
			email := c.PostForm("email")
			role := c.PostForm("role")
			statusStr := c.PostForm("status")
			newPassword := c.PostForm("new_password")

			if nickname != "" {
				u.Nickname = nickname
			}
			if email != "" {
				u.Email = email
			}
			if role != "" {
				u.Role = role
			}
			if statusStr != "" {
				s, _ := strconv.Atoi(statusStr)
				u.Status = s
			}
			if newPassword != "" {
				if len(newPassword) < 6 {
					c.Redirect(http.StatusFound, "/admin/users?error=密码至少6位")
					return
				}
				hash, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
				u.Password = string(hash)
			}

			db.Save(&u)
			c.Redirect(http.StatusFound, "/admin/users?success=1")
		})

		// 删除用户
		admin.POST("/users/:id/delete", func(c *gin.Context) {
			id := c.Param("id")
			// Don't delete self
			currentUser := c.MustGet("user").(*User)
			if fmt.Sprintf("%d", currentUser.ID) == id {
				c.Redirect(http.StatusFound, "/admin/users?error=不能删除自己")
				return
			}
			db.Delete(&User{}, id)
			// Also delete user's domains
			db.Where("user_id = ?", id).Delete(&Domain{})
			c.Redirect(http.StatusFound, "/admin/users?success=1")
		})

		// 获取所有用户列表（JSON，用于域名添加时选择用户）
		admin.GET("/api/users", func(c *gin.Context) {
			var users []User
			db.Select("id", "username", "nickname", "email").Order("id ASC").Find(&users)
			c.JSON(http.StatusOK, users)
		})
	}

	// ── 403 页面 ──
	r.GET("/403", func(c *gin.Context) {
		c.HTML(http.StatusForbidden, "403.html", gin.H{"Message": "权限不足"})
	})

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
			c.Redirect(http.StatusMovedPermanently, target)
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
