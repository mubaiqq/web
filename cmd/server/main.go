package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ── Models ──

type User struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Username string `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Email    string `json:"email" gorm:"uniqueIndex;size:128;not null"`
	Password string `json:"-" gorm:"size:128;not null"`
	Nickname string `json:"nickname" gorm:"size:64"`
}

type Domain struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	UserID   uint   `json:"user_id" gorm:"index"`
	Hostname string `json:"hostname" gorm:"uniqueIndex;size:255;not null"`
	Mode     string `json:"mode" gorm:"size:20;not null;default:page"`
	Target   string `json:"target" gorm:"size:512"`
	Template string `json:"template" gorm:"size:100"`
	Title    string `json:"title" gorm:"size:255"`
	Content  string `json:"content" gorm:"type:text"`
	Status   int    `json:"status" gorm:"default:1"`
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
		c.Redirect(http.StatusFound, "/dashboard")
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

	// ── 需要登录的页面 ──
	auth := r.Group("/", requireAuth(db))
	{
		auth.GET("/dashboard", func(c *gin.Context) {
			user := c.MustGet("user").(*User)
			var domains []Domain
			db.Where("user_id = ?", user.ID).Order("id DESC").Find(&domains)
			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"User":    user,
				"Domains": domains,
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
