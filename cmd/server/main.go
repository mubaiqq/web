package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       uint      `json:"user_id" gorm:"index"`
	Hostname     string    `json:"hostname" gorm:"uniqueIndex;size:255;not null"`
	Mode         string    `json:"mode" gorm:"size:20;not null;default:page"`
	RedirectType string    `json:"redirect_type" gorm:"size:10;default:301"` // 301|302|meta
	Target       string    `json:"target" gorm:"size:512"`
	Template     string    `json:"template" gorm:"size:100"`
	Title        string    `json:"title" gorm:"size:255"`
	Content      string    `json:"content" gorm:"type:text"`
	CustomCSS    string    `json:"custom_css" gorm:"type:text"`
	CustomJS     string    `json:"custom_js" gorm:"type:text"`
	ExpiresAt    int64     `json:"expires_at" gorm:"default:0"` // 0=永久
	Status       int       `json:"status" gorm:"default:1"`     // 1=active 0=disabled
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

type Template struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:100;not null"`
	Desc      string    `json:"desc" gorm:"size:255"`
	Content   string    `json:"content" gorm:"type:text"`
	Status    int       `json:"status" gorm:"default:1"`
	CreatedAt time.Time `json:"created_at"`
}

type SiteSetting struct {
	ID    uint   `gorm:"primaryKey"`
	Key   string `gorm:"uniqueIndex;size:100;not null"`
	Value string `gorm:"type:text"`
}

// Phase 4: 域名增加跳转类型和到期时间
// (通过 AutoMigrate ALTER TABLE 添加新列)

type UploadFile struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Filename  string    `json:"filename" gorm:"size:255;not null"`
	OrigName  string    `json:"orig_name" gorm:"size:255"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type" gorm:"size:100"`
	Path      string    `json:"path" gorm:"size:512"`
	CreatedAt time.Time `json:"created_at"`
}

type APIKey struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	Key       string    `json:"key" gorm:"uniqueIndex;size:64;not null"`
	Status    int       `json:"status" gorm:"default:1"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
}

type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	Content   string    `json:"content" gorm:"type:text"`
	Type      string    `json:"type" gorm:"size:20;default:info"` // info|warning|success
	Read      int       `json:"read" gorm:"default:0"`           // 0=unread 1=read
	CreatedAt time.Time `json:"created_at"`
}

type OperationLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Action    string    `json:"action" gorm:"size:100;not null"`
	Target    string    `json:"target" gorm:"size:255"`
	Detail    string    `json:"detail" gorm:"type:text"`
	IP        string    `json:"ip" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
}

// ── DB Init ──

func initDB(path string) *gorm.DB {
	os.MkdirAll(filepath.Dir(path), 0755)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	db.AutoMigrate(&User{}, &Domain{}, &VisitLog{}, &Template{}, &SiteSetting{},
		&UploadFile{}, &APIKey{}, &Notification{}, &OperationLog{})

	// Phase 4: Domain 新字段 (ALTER TABLE 兼容旧数据)
	sqlDB, _ := db.DB()
	sqlDB.Exec("ALTER TABLE domains ADD COLUMN redirect_type TEXT DEFAULT '301'")
	sqlDB.Exec("ALTER TABLE domains ADD COLUMN expires_at INTEGER DEFAULT 0")
	sqlDB.Exec("ALTER TABLE domains ADD COLUMN custom_css TEXT DEFAULT ''")
	sqlDB.Exec("ALTER TABLE domains ADD COLUMN custom_js TEXT DEFAULT ''")

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
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"defaultStr": func(s, def string) string {
			if s == "" {
				return def
			}
			return s
		},
	}
}

// ── Site Settings helpers ──

func getSetting(db *gorm.DB, key string) string {
	var s SiteSetting
	if err := db.Where("key = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func getAllSettings(db *gorm.DB) map[string]string {
	var settings []SiteSetting
	db.Find(&settings)
	m := make(map[string]string)
	for _, s := range settings {
		m[s.Key] = s.Value
	}
	return m
}

func setSetting(db *gorm.DB, key, value string) {
	var s SiteSetting
	if err := db.Where("key = ?", key).First(&s).Error; err != nil {
		db.Create(&SiteSetting{Key: key, Value: value})
	} else {
		db.Model(&s).Update("value", value)
	}
}

// ── In-Memory Domain Cache (Phase 5) ──

var domainCache sync.Map // hostname -> Domain

func cacheGetDomain(hostname string) (*Domain, bool) {
	if v, ok := domainCache.Load(hostname); ok {
		d := v.(Domain)
		return &d, true
	}
	return nil, false
}

func cacheSetDomain(d Domain) {
	domainCache.Store(d.Hostname, d)
}

func cacheDeleteDomain(hostname string) {
	domainCache.Delete(hostname)
}

func warmDomainCache(db *gorm.DB) {
	var domains []Domain
	db.Where("status = ?", 1).Find(&domains)
	for _, d := range domains {
		domainCache.Store(d.Hostname, d)
	}
	log.Printf("📦 缓存预热: %d 个域名", len(domains))
}

// ── Rate Limiting (Phase 5) ──

var rateLimitMap sync.Map // ip -> []timestamp

func rateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		key := ip

		val, _ := rateLimitMap.LoadOrStore(key, &[]time.Time{})
		times := val.(*[]time.Time)

		// Clean old entries
		var valid []time.Time
		for _, t := range *times {
			if now.Sub(t) < window {
				valid = append(valid, t)
			}
		}

		if len(valid) >= maxRequests {
			c.HTML(http.StatusTooManyRequests, "429.html", gin.H{"Message": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}

		*times = append(valid, now)
		rateLimitMap.Store(key, times)
		c.Next()
	}
}

// ── Operation Log (Phase 5) ──

func logOperation(db *gorm.DB, userID uint, action, target, detail, ip string) {
	db.Create(&OperationLog{
		UserID: userID,
		Action: action,
		Target: target,
		Detail: detail,
		IP:     ip,
	})
}

// ── API Key helpers (Phase 6) ──

func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "dk_" + hex.EncodeToString(b)
}

func requireAPIKey(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("api_key")
		}
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 API Key"})
			c.Abort()
			return
		}
		var ak APIKey
		if err := db.Where("key = ? AND status = 1", key).First(&ak).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API Key 无效"})
			c.Abort()
			return
		}
		db.Model(&ak).Update("last_used", time.Now())
		c.Set("api_user_id", ak.UserID)
		c.Set("api_key_id", ak.ID)
		c.Next()
	}
}

// ── Notification helper ──

func createNotification(db *gorm.DB, userID uint, title, content, ntype string) {
	db.Create(&Notification{
		UserID:  userID,
		Title:   title,
		Content: content,
		Type:    ntype,
	})
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
	warmDomainCache(db)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Rate limiting: 全局 60 req/min, 登录/注册 10 req/min
	r.Use(rateLimit(60, time.Minute))

	// 创建上传目录
	os.MkdirAll("./uploads", 0755)

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

			// Get user's domain hostnames for visit count
			var hostnames []string
			for _, d := range domains {
				hostnames = append(hostnames, d.Hostname)
			}
			var visitCount int64
			if len(hostnames) > 0 {
				db.Model(&VisitLog{}).Where("domain IN ?", hostnames).Count(&visitCount)
			}

			// Get available templates
			var templates []Template
			db.Where("status = ?", 1).Order("id ASC").Find(&templates)

			// Announcements (hardcoded for now)
			announcements := []map[string]string{
				{"title": "欢迎使用 DomainOS v0.2.0", "desc": "平台已上线，支持域名绑定、页面渲染、模板系统和智能跳转功能。", "date": "2026-05-01", "color": "indigo"},
				{"title": "新功能上线", "desc": "模板管理、站点设置、域名内容编辑、用户设置等功能已上线。", "date": "2026-05-02", "color": "emerald"},
			}

			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"User":          user,
				"Domains":       domains,
				"VisitCount":    visitCount,
				"Templates":     templates,
				"Announcements": announcements,
			})
		})

		auth.POST("/domains", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			hostname := c.PostForm("hostname")
			mode := c.PostForm("mode")
			redirectType := c.DefaultPostForm("redirect_type", "301")
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
				RedirectType: redirectType,
				Target:       target,
				Title:        title,
				Template:     "default",
			}
			if err := db.Create(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/dashboard?error=域名已存在")
				return
			}
			cacheSetDomain(d)
			c.Redirect(http.StatusFound, "/dashboard")
		})

		auth.POST("/domains/:id/delete", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).Delete(&Domain{})
			c.Redirect(http.StatusFound, "/dashboard")
		})

		auth.POST("/domains/:id", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var d Domain
			if err := db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/dashboard?error=域名不存在")
				return
			}
			title := c.PostForm("title")
			content := c.PostForm("content")
			tmpl := c.PostForm("template")
			redirectType := c.PostForm("redirect_type")
			customCSS := c.PostForm("custom_css")
			customJS := c.PostForm("custom_js")
			if title != "" {
				d.Title = title
			}
			d.Content = content
			if tmpl != "" {
				d.Template = tmpl
			}
			if redirectType != "" {
				d.RedirectType = redirectType
			}
			d.CustomCSS = customCSS
			d.CustomJS = customJS
			db.Save(&d)
			cacheDeleteDomain(d.Hostname)
			if d.Status == 1 {
				cacheSetDomain(d)
			}
			c.Redirect(http.StatusFound, "/dashboard")
		})

		// 用户设置页
		auth.GET("/settings", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			c.HTML(http.StatusOK, "user-settings.html", gin.H{
				"User":    user,
				"Success": c.Query("success"),
				"Error":   c.Query("error"),
			})
		})

		// 修改个人信息
		auth.POST("/settings/profile", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			nickname := c.PostForm("nickname")
			email := c.PostForm("email")
			if nickname != "" {
				db.Model(user).Update("nickname", nickname)
			}
			if email != "" {
				var count int64
				db.Model(&User{}).Where("email = ? AND id != ?", email, user.ID).Count(&count)
				if count > 0 {
					c.Redirect(http.StatusFound, "/settings?error=邮箱已被使用")
					return
				}
				db.Model(user).Update("email", email)
			}
			c.Redirect(http.StatusFound, "/settings?success=个人信息已更新")
		})

		// 修改密码
		auth.POST("/settings/password", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			oldPwd := c.PostForm("old_password")
			newPwd := c.PostForm("new_password")
			confirmPwd := c.PostForm("confirm_password")
			if oldPwd == "" || newPwd == "" {
				c.Redirect(http.StatusFound, "/settings?error=请填写所有字段")
				return
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
				c.Redirect(http.StatusFound, "/settings?error=原密码错误")
				return
			}
			if len(newPwd) < 6 {
				c.Redirect(http.StatusFound, "/settings?error=新密码至少6位")
				return
			}
			if newPwd != confirmPwd {
				c.Redirect(http.StatusFound, "/settings?error=两次密码不一致")
				return
			}
			hash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
			db.Model(user).Update("password", string(hash))
			c.Redirect(http.StatusFound, "/settings?success=密码已修改")
		})

		// 域名编辑页
		auth.GET("/domains/:id/edit", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var d Domain
			if err := db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/dashboard?error=域名不存在")
				return
			}
			var templates []Template
			db.Where("status = ?", 1).Order("id ASC").Find(&templates)
			c.HTML(http.StatusOK, "domain-edit.html", gin.H{
				"User":     user,
				"Domain":   d,
				"Templates": templates,
			})
		})

		// 用户模板中心
		auth.GET("/templates", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var templates []Template
			db.Where("status = ?", 1).Order("id ASC").Find(&templates)
			var domains []Domain
			db.Where("user_id = ? AND mode = ?", user.ID, "page").Order("id ASC").Find(&domains)
			c.HTML(http.StatusOK, "user-templates.html", gin.H{
				"User":      user,
				"Templates": templates,
				"Domains":   domains,
				"Success":   c.Query("success"),
			})
		})

		// 模板预览（用户端）
		auth.GET("/templates/:id/preview", func(c *gin.Context) {
			var t Template
			if db.First(&t, c.Param("id")).Error != nil {
				c.String(http.StatusNotFound, "模板不存在")
				return
			}
			// Render template content as a standalone page
			siteTitle := getSetting(db, "site_title")
			if siteTitle == "" {
				siteTitle = "DomainOS"
			}
			previewHTML := `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>模板预览</title>
<style>@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap');
*{margin:0;padding:0;box-sizing:border-box}body{font-family:'Inter',-apple-system,sans-serif;background:linear-gradient(135deg,#eef2ff,#e0e7ff,#f0f9ff,#ede9fe);min-height:100vh;display:flex;align-items:center;justify-content:center;color:#111827;-webkit-font-smoothing:antialiased}
.wrap{text-align:center;padding:3rem;max-width:640px;background:rgba(255,255,255,0.6);backdrop-filter:blur(20px);border:1px solid rgba(255,255,255,0.5);border-radius:2rem;box-shadow:0 8px 40px rgba(99,102,241,0.08)}
h1{font-size:2.5rem;font-weight:800;letter-spacing:-0.5px;margin-bottom:1.2rem;background:linear-gradient(135deg,#312e81,#6366f1);-webkit-background-clip:text;-webkit-text-fill-color:transparent}
.content{font-size:1.05rem;line-height:1.8;color:#6b7280}</style></head><body><div class="wrap">
<h1>预览: ` + t.Name + `</h1>
<div class="content">` + t.Content + `</div>
<div style="margin-top:2rem;padding:0.3rem 1rem;background:rgba(255,255,255,0.6);border:1px solid rgba(255,255,255,0.5);border-radius:999px;font-size:0.8rem;color:#9ca3af;font-family:ui-monospace,monospace;display:inline-block">模板: ` + t.Name + `</div>
</div></body></html>`
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(previewHTML))
		})

		// 统计分析页
		auth.GET("/analytics", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var domains []Domain
			db.Where("user_id = ?", user.ID).Order("id ASC").Find(&domains)
			c.HTML(http.StatusOK, "analytics.html", gin.H{
				"User":    user,
				"Domains": domains,
			})
		})

		// 统计 API
		auth.GET("/api/analytics/stats", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			domain := c.Query("domain")
			days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
			if days < 1 {
				days = 7
			}
			if days > 365 {
				days = 365
			}

			// Verify domain belongs to user
			var d Domain
			if err := db.Where("hostname = ? AND user_id = ?", domain, user.ID).First(&d).Error; err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
				return
			}

			now := time.Now()
			startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			startTime := startOfDay.AddDate(0, 0, -days+1)

			var totalPV int64
			db.Model(&VisitLog{}).Where("domain = ? AND created_at >= ?", domain, startTime.Unix()).Count(&totalPV)

			var totalUV int64
			db.Model(&VisitLog{}).Where("domain = ? AND created_at >= ?", domain, startTime.Unix()).Distinct("ip").Count(&totalUV)

			var todayPV int64
			db.Model(&VisitLog{}).Where("domain = ? AND created_at >= ?", domain, startOfDay.Unix()).Count(&todayPV)

			var todayUV int64
			db.Model(&VisitLog{}).Where("domain = ? AND created_at >= ?", domain, startOfDay.Unix()).Distinct("ip").Count(&todayUV)

			type DayStat struct {
				Date string
				PV   int64
				UV   int64
			}
			var trend []DayStat
			for i := 0; i < days; i++ {
				dayStart := startTime.AddDate(0, 0, i)
				dayEnd := dayStart.AddDate(0, 0, 1)
				var pv int64
				var uv int64
				db.Model(&VisitLog{}).Where("domain = ? AND created_at >= ? AND created_at < ?", domain, dayStart.Unix(), dayEnd.Unix()).Count(&pv)
				db.Model(&VisitLog{}).Where("domain = ? AND created_at >= ? AND created_at < ?", domain, dayStart.Unix(), dayEnd.Unix()).Distinct("ip").Count(&uv)
				trend = append(trend, DayStat{Date: dayStart.Format("01-02"), PV: pv, UV: uv})
			}

			var trendLabels []string
			var trendPV []int64
			var trendUV []int64
			for _, t := range trend {
				trendLabels = append(trendLabels, t.Date)
				trendPV = append(trendPV, t.PV)
				trendUV = append(trendUV, t.UV)
			}

			type PathCount struct {
				Path  string
				Count int64
			}
			var topPaths []PathCount
			db.Model(&VisitLog{}).Select("path, count(*) as count").Where("domain = ? AND created_at >= ?", domain, startTime.Unix()).Group("path").Order("count DESC").Limit(10).Scan(&topPaths)

			type RefererCount struct {
				Referer string
				Count   int64
			}
			var topReferers []RefererCount
			db.Model(&VisitLog{}).Select("referer, count(*) as count").Where("domain = ? AND created_at >= ? AND referer != ''", domain, startTime.Unix()).Group("referer").Order("count DESC").Limit(10).Scan(&topReferers)

			c.JSON(http.StatusOK, gin.H{
				"total_pv":     totalPV,
				"total_uv":     totalUV,
				"today_pv":     todayPV,
				"today_uv":     todayUV,
				"trend_labels": trendLabels,
				"trend_pv":     trendPV,
				"trend_uv":     trendUV,
				"top_paths":    topPaths,
				"top_referers": topReferers,
			})
		})

		// ── Phase 4: 文件上传 ──
		auth.POST("/upload", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			file, header, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
				return
			}
			defer file.Close()

			// 限制 10MB
			if header.Size > 10*1024*1024 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小不能超过 10MB"})
				return
			}

			// 生成唯一文件名
			ext := filepath.Ext(header.Filename)
			b := make([]byte, 16)
			rand.Read(b)
			newName := hex.EncodeToString(b) + ext
			savePath := filepath.Join("./uploads", newName)

			out, err := os.Create(savePath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
				return
			}
			defer out.Close()
			io.Copy(out, file)

			uf := UploadFile{
				UserID:   user.ID,
				Filename: newName,
				OrigName: header.Filename,
				Size:     header.Size,
				MimeType: header.Header.Get("Content-Type"),
				Path:     "/uploads/" + newName,
			}
			db.Create(&uf)

			c.JSON(http.StatusOK, gin.H{
				"url":      "/uploads/" + newName,
				"filename": header.Filename,
				"id":       uf.ID,
			})
		})

		// 文件管理页
		auth.GET("/files", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var files []UploadFile
			db.Where("user_id = ?", user.ID).Order("id DESC").Find(&files)
			c.HTML(http.StatusOK, "user-files.html", gin.H{
				"User":  user,
				"Files": files,
			})
		})

		// 删除文件
		auth.POST("/files/:id/delete", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var f UploadFile
			if err := db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&f).Error; err != nil {
				c.Redirect(http.StatusFound, "/files?error=文件不存在")
				return
			}
			os.Remove("." + f.Path)
			db.Delete(&f)
			c.Redirect(http.StatusFound, "/files")
		})

		// ── Phase 6: 通知系统 ──
		auth.GET("/notifications", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var notifs []Notification
			db.Where("user_id = ?", user.ID).Order("id DESC").Limit(50).Find(&notifs)
			var unreadCount int64
			db.Model(&Notification{}).Where("user_id = ? AND `read` = 0", user.ID).Count(&unreadCount)
			c.HTML(http.StatusOK, "notifications.html", gin.H{
				"User":        user,
				"Notifications": notifs,
				"UnreadCount": unreadCount,
			})
		})

		auth.POST("/notifications/:id/read", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			db.Model(&Notification{}).Where("id = ? AND user_id = ?", c.Param("id"), user.ID).Update("read", 1)
			c.Redirect(http.StatusFound, "/notifications")
		})

		auth.POST("/notifications/read-all", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			db.Model(&Notification{}).Where("user_id = ? AND `read` = 0", user.ID).Update("read", 1)
			c.Redirect(http.StatusFound, "/notifications")
		})

		// ── Phase 6: API Key 管理 ──
		auth.GET("/api-keys", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var keys []APIKey
			db.Where("user_id = ?", user.ID).Order("id DESC").Find(&keys)
			c.HTML(http.StatusOK, "api-keys.html", gin.H{
				"User": user,
				"Keys": keys,
			})
		})

		auth.POST("/api-keys", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			name := c.PostForm("name")
			if name == "" {
				name = "Default"
			}
			ak := APIKey{
				UserID: user.ID,
				Name:   name,
				Key:    generateAPIKey(),
				Status: 1,
			}
			db.Create(&ak)
			c.Redirect(http.StatusFound, "/api-keys")
		})

		auth.POST("/api-keys/:id/delete", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).Delete(&APIKey{})
			c.Redirect(http.StatusFound, "/api-keys")
		})

		auth.POST("/api-keys/:id/toggle", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var ak APIKey
			if db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&ak).Error != nil {
				c.Redirect(http.StatusFound, "/api-keys?error=Key不存在")
				return
			}
			if ak.Status == 1 {
				ak.Status = 0
			} else {
				ak.Status = 1
			}
			db.Save(&ak)
			c.Redirect(http.StatusFound, "/api-keys")
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
			redirectType := c.DefaultPostForm("redirect_type", "301")
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
				UserID:       uint(userID),
				Hostname:     hostname,
				Mode:         mode,
				RedirectType: redirectType,
				Target:       target,
				Title:        title,
				Template:     "default",
			}
			if err := db.Create(&d).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/domains?error=域名已存在")
				return
			}
			cacheSetDomain(d)
			logOperation(db, c.MustGet("user").(*User).ID, "create_domain", hostname, "", c.ClientIP())
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
			redirectType := c.PostForm("redirect_type")
			target := c.PostForm("target")
			title := c.PostForm("title")
			customCSS := c.PostForm("custom_css")
			customJS := c.PostForm("custom_js")
			userIDStr := c.PostForm("user_id")
			statusStr := c.PostForm("status")

			oldHostname := d.Hostname
			if hostname != "" {
				d.Hostname = hostname
			}
			if mode != "" {
				d.Mode = mode
			}
			if redirectType != "" {
				d.RedirectType = redirectType
			}
			d.Target = target
			d.Title = title
			d.CustomCSS = customCSS
			d.CustomJS = customJS
			if userIDStr != "" {
				uid, _ := strconv.ParseUint(userIDStr, 10, 64)
				d.UserID = uint(uid)
			}
			if statusStr != "" {
				s, _ := strconv.Atoi(statusStr)
				d.Status = s
			}

			db.Save(&d)
			// Update cache
			cacheDeleteDomain(oldHostname)
			if d.Status == 1 {
				cacheSetDomain(d)
			}
			logOperation(db, c.MustGet("user").(*User).ID, "edit_domain", d.Hostname, "", c.ClientIP())
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

		// ── 模板管理 ──
		admin.GET("/templates", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var templates []Template
			db.Order("id DESC").Find(&templates)
			c.HTML(http.StatusOK, "admin-templates.html", gin.H{
				"User":      user,
				"Templates": templates,
				"Page":      "templates",
			})
		})

		admin.POST("/templates", func(c *gin.Context) {
			name := c.PostForm("name")
			desc := c.PostForm("desc")
			content := c.PostForm("content")
			if name == "" {
				c.Redirect(http.StatusFound, "/admin/templates?error=模板名称不能为空")
				return
			}
			t := Template{Name: name, Desc: desc, Content: content, Status: 1}
			if err := db.Create(&t).Error; err != nil {
				c.Redirect(http.StatusFound, "/admin/templates?error=模板名称已存在")
				return
			}
			c.Redirect(http.StatusFound, "/admin/templates?success=1")
		})

		admin.POST("/templates/:id", func(c *gin.Context) {
			var t Template
			if db.First(&t, c.Param("id")).Error != nil {
				c.Redirect(http.StatusFound, "/admin/templates?error=模板不存在")
				return
			}
			name := c.PostForm("name")
			desc := c.PostForm("desc")
			content := c.PostForm("content")
			statusStr := c.PostForm("status")
			if name != "" {
				t.Name = name
			}
			t.Desc = desc
			t.Content = content
			if statusStr != "" {
				s, _ := strconv.Atoi(statusStr)
				t.Status = s
			}
			db.Save(&t)
			c.Redirect(http.StatusFound, "/admin/templates?success=1")
		})

		admin.POST("/templates/:id/delete", func(c *gin.Context) {
			db.Delete(&Template{}, c.Param("id"))
			c.Redirect(http.StatusFound, "/admin/templates?success=1")
		})

		// 模板预览（管理员端）
		admin.GET("/templates/:id/preview", func(c *gin.Context) {
			var t Template
			if db.First(&t, c.Param("id")).Error != nil {
				c.String(http.StatusNotFound, "模板不存在")
				return
			}
			previewHTML := `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>模板预览</title>
<style>@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap');
*{margin:0;padding:0;box-sizing:border-box}body{font-family:'Inter',-apple-system,sans-serif;background:linear-gradient(135deg,#eef2ff,#e0e7ff,#f0f9ff,#ede9fe);min-height:100vh;display:flex;align-items:center;justify-content:center;color:#111827;-webkit-font-smoothing:antialiased}
.wrap{text-align:center;padding:3rem;max-width:640px;background:rgba(255,255,255,0.6);backdrop-filter:blur(20px);border:1px solid rgba(255,255,255,0.5);border-radius:2rem;box-shadow:0 8px 40px rgba(99,102,241,0.08)}
h1{font-size:2.5rem;font-weight:800;letter-spacing:-0.5px;margin-bottom:1.2rem;background:linear-gradient(135deg,#312e81,#6366f1);-webkit-background-clip:text;-webkit-text-fill-color:transparent}
.content{font-size:1.05rem;line-height:1.8;color:#6b7280}</style></head><body><div class="wrap">
<h1>预览: ` + t.Name + `</h1>
<div class="content">` + t.Content + `</div>
<div style="margin-top:2rem;padding:0.3rem 1rem;background:rgba(255,255,255,0.6);border:1px solid rgba(255,255,255,0.5);border-radius:999px;font-size:0.8rem;color:#9ca3af;font-family:ui-monospace,monospace;display:inline-block">模板: ` + t.Name + `</div>
</div></body></html>`
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(previewHTML))
		})

		// ── 站点设置 ──
		admin.GET("/settings", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			settings := getAllSettings(db)
			c.HTML(http.StatusOK, "admin-settings.html", gin.H{
				"User":     user,
				"Settings": settings,
				"Success":  c.Query("success"),
				"Error":    c.Query("error"),
				"Page":     "settings",
			})
		})

		admin.POST("/settings", func(c *gin.Context) {
			keys := []string{"site_title", "footer_text", "bg_image", "stats_code"}
			for _, key := range keys {
				val := c.PostForm(key)
				setSetting(db, key, val)
			}
			c.Redirect(http.StatusFound, "/admin/settings?success=设置已保存")
		})

		admin.POST("/settings/password", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			oldPwd := c.PostForm("old_password")
			newPwd := c.PostForm("new_password")
			confirmPwd := c.PostForm("confirm_password")
			if oldPwd == "" || newPwd == "" {
				c.Redirect(http.StatusFound, "/admin/settings?error=请填写所有字段")
				return
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPwd)); err != nil {
				c.Redirect(http.StatusFound, "/admin/settings?error=原密码错误")
				return
			}
			if len(newPwd) < 6 {
				c.Redirect(http.StatusFound, "/admin/settings?error=新密码至少6位")
				return
			}
			if newPwd != confirmPwd {
				c.Redirect(http.StatusFound, "/admin/settings?error=两次密码不一致")
				return
			}
			hash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
			db.Model(user).Update("password", string(hash))
			c.Redirect(http.StatusFound, "/admin/settings?success=密码已修改")
		})

		// ── Phase 5: 操作日志 ──
		admin.GET("/logs", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			if page < 1 {
				page = 1
			}
			pageSize := 50
			var total int64
			db.Model(&OperationLog{}).Count(&total)
			totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
			if totalPages < 1 {
				totalPages = 1
			}
			var logs []OperationLog
			db.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
			c.HTML(http.StatusOK, "admin-logs.html", gin.H{
				"User":       user,
				"Logs":       logs,
				"Page":       page,
				"TotalPages": totalPages,
				"Total":      total,
				"PageName":   "logs",
			})
		})

		// ── Phase 6: 管理员发送通知 ──
		admin.POST("/notify", func(c *gin.Context) {
			userIDStr := c.PostForm("user_id")
			title := c.PostForm("title")
			content := c.PostForm("content")
			ntype := c.DefaultPostForm("type", "info")
			if title == "" || content == "" {
				c.Redirect(http.StatusFound, "/admin/users?error=标题和内容不能为空")
				return
			}
			if userIDStr == "all" {
				// Send to all users
				var users []User
				db.Find(&users)
				for _, u := range users {
					createNotification(db, u.ID, title, content, ntype)
				}
				c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users?success=已通知 %d 个用户", len(users)))
			} else {
				uid, _ := strconv.ParseUint(userIDStr, 10, 64)
				if uid == 0 {
					c.Redirect(http.StatusFound, "/admin/users?error=无效用户ID")
					return
				}
				createNotification(db, uint(uid), title, content, ntype)
				c.Redirect(http.StatusFound, "/admin/users?success=通知已发送")
			}
		})
	}

	// ── 403 页面 ──
	r.GET("/403", func(c *gin.Context) {
		c.HTML(http.StatusForbidden, "403.html", gin.H{"Message": "权限不足"})
	})

	// ── 429 页面 ──
	r.GET("/429", func(c *gin.Context) {
		c.HTML(http.StatusTooManyRequests, "429.html", gin.H{"Message": "请求过于频繁"})
	})

	// ── 静态文件上传目录 ──
	r.Static("/uploads", "./uploads")

	// ── Phase 6: RESTful API ──
	api := r.Group("/api/v1", requireAPIKey(db))
	{
		// 域名列表
		api.GET("/domains", func(c *gin.Context) {
			uid, _ := c.Get("api_user_id")
			var domains []Domain
			db.Where("user_id = ?", uid).Find(&domains)
			c.JSON(http.StatusOK, gin.H{"domains": domains})
		})

		// 添加域名
		api.POST("/domains", func(c *gin.Context) {
			uid, _ := c.Get("api_user_id")
			var req struct {
				Hostname     string `json:"hostname" binding:"required"`
				Mode         string `json:"mode"`
				RedirectType string `json:"redirect_type"`
				Target       string `json:"target"`
				Title        string `json:"title"`
				Content      string `json:"content"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.Mode == "" {
				req.Mode = "page"
			}
			if req.RedirectType == "" {
				req.RedirectType = "301"
			}
			d := Domain{
				UserID:       uid.(uint),
				Hostname:     req.Hostname,
				Mode:         req.Mode,
				RedirectType: req.RedirectType,
				Target:       req.Target,
				Title:        req.Title,
				Content:      req.Content,
				Template:     "default",
			}
			if err := db.Create(&d).Error; err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "域名已存在"})
				return
			}
			cacheSetDomain(d)
			logOperation(db, uid.(uint), "api_create_domain", req.Hostname, "", c.ClientIP())
			c.JSON(http.StatusCreated, gin.H{"domain": d})
		})

		// 删除域名
		api.DELETE("/domains/:id", func(c *gin.Context) {
			uid, _ := c.Get("api_user_id")
			var d Domain
			if err := db.Where("id = ? AND user_id = ?", c.Param("id"), uid).First(&d).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "域名不存在"})
				return
			}
			cacheDeleteDomain(d.Hostname)
			db.Delete(&d)
			logOperation(db, uid.(uint), "api_delete_domain", d.Hostname, "", c.ClientIP())
			c.JSON(http.StatusOK, gin.H{"message": "已删除"})
		})

		// 域名统计
		api.GET("/domains/:id/stats", func(c *gin.Context) {
			uid, _ := c.Get("api_user_id")
			var d Domain
			if err := db.Where("id = ? AND user_id = ?", c.Param("id"), uid).First(&d).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "域名不存在"})
				return
			}
			var totalPV int64
			db.Model(&VisitLog{}).Where("domain = ?", d.Hostname).Count(&totalPV)
			var totalUV int64
			db.Model(&VisitLog{}).Where("domain = ?", d.Hostname).Distinct("ip").Count(&totalUV)
			c.JSON(http.StatusOK, gin.H{"hostname": d.Hostname, "pv": totalPV, "uv": totalUV})
		})
	}

	// ── 域名解析路由 ──
	r.NoRoute(func(c *gin.Context) {
		host := c.Request.Host
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}

		// Phase 5: 优先从缓存读取
		var d *Domain
		if cached, ok := cacheGetDomain(host); ok {
			d = cached
		} else {
			var domain Domain
			if err := db.Where("hostname = ? AND status = 1", host).First(&domain).Error; err != nil {
				c.HTML(http.StatusNotFound, "404.html", gin.H{"Host": host})
				return
			}
			d = &domain
			cacheSetDomain(domain)
		}

		// 检查域名是否过期
		if d.ExpiresAt > 0 && time.Now().Unix() > d.ExpiresAt {
			c.HTML(http.StatusGone, "404.html", gin.H{"Host": host, "Message": "域名已过期"})
			return
		}

		// 记录访问（异步，使用缓存域名）
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
			// Phase 4: 支持 301/302/meta 跳转
			switch d.RedirectType {
			case "302":
				c.Redirect(http.StatusFound, target)
			case "meta":
				metaHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>跳转中...</title><meta http-equiv="refresh" content="0;url=%s"></head><body><p>正在跳转到 <a href="%s">%s</a></p></body></html>`, target, target, target)
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(metaHTML))
			default: // 301
				c.Redirect(http.StatusMovedPermanently, target)
			}
		case "page":
			// Resolve template content
			tmplContent := ""
			if d.Template != "" && d.Template != "default" {
				var tmpl Template
				if db.Where("name = ? AND status = 1", d.Template).First(&tmpl).Error == nil {
					tmplContent = tmpl.Content
				}
			}
			settings := getAllSettings(db)
			siteTitle := settings["site_title"]
			if siteTitle == "" {
				siteTitle = "DomainOS"
			}
			c.HTML(http.StatusOK, "site.html", gin.H{
				"Domain":      d,
				"Title":       d.Title,
				"Content":     d.Content,
				"Host":        host,
				"TmplContent": tmplContent,
				"SiteTitle":   siteTitle,
				"FooterText":  settings["footer_text"],
				"BgImage":     settings["bg_image"],
				"StatsCode":   template.HTML(settings["stats_code"]),
				"CustomCSS":   d.CustomCSS,
				"CustomJS":    d.CustomJS,
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
