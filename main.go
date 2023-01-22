package main

import (
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var sb session_base         // Session数据库对象
var valid_time int64 = 1800 // Session有效时间（秒）
var db *sqlx.DB             // 数据库对象

type session_base struct {
	m sync.Map
	/* 键：字符串类型，SessionID
	 * 值：gin.H 类型，用户名（userID)，过期时间(due) */

	user2ID sync.Map
}

func (sb *session_base) set(id string, info gin.H) {
	if val, ok := sb.user2ID.Load("userID"); ok {
		sb.del(val.(string))
	}
	sb.m.Store(id, info)
	sb.user2ID.Store(info["userID"], id)
}
func (sb *session_base) del(id string) {
	sb.m.Delete(id)
}
func (sb *session_base) get(id string) (gin.H, bool) {
	value, OK := sb.m.Load(id)
	if OK {
		info, _ := value.(gin.H)
		due := info["due"].(int64)
		if time.Now().Unix() > due {
			// 有Session记录，但已过期，返回false
			return gin.H{}, false
		} else {
			// 有Session记录，且未过期，返回数据
			return info, true
		}
	} else {
		// 无Session记录，返回false
		return gin.H{}, false
	}
}

func query(sql string) []map[string]any {
	res := []map[string]any{}
	rows, _ := db.Queryx(sql)
	for rows.Next() {
		temp := map[string]any{}
		rows.MapScan(temp)
		res = append(res, temp)
	}
	return res
}

func produce_cookie() string {
	// 随机生成新cookie算法
	// cookie是长度为10的字符串，由数字、大小写字母组成
	var res strings.Builder
	for i := 0; i < 10; i++ {
		a := rand.Intn(3) // 0 : 生成数字， 1 : 生成小写字母， 2 : 生成大写字母
		if a == 0 {
			b := rand.Intn(10)
			res.WriteRune(rune('0' + b))
		} else if a == 1 {
			b := rand.Intn(26)
			res.WriteRune(rune('a' + b))
		} else {
			b := rand.Intn(26)
			res.WriteRune(rune('A' + b))
		}
	}
	if _, exist := sb.get(res.String()); exist {
		// cookie已存在，重新生成
		return produce_cookie()
	} else {
		return res.String()
	}
}

func Midware_Auth(c *gin.Context) {

	if cookie, err := c.Request.Cookie("SessionID"); err == nil {
		// 获得了SessionID
		SessionID := cookie.Value
		info, OK := sb.get(SessionID)
		if OK {
			// Session尚未过期，重置Session时间和cookie
			newID := produce_cookie()
			sb.del(SessionID)
			info["due"] = time.Now().Unix() + valid_time
			sb.set(newID, info)
			c.SetCookie("SessionID", newID, 3600, "/", "localhost", false, true)
			c.Set("login_status", true)
			c.Set("userID", info["userID"])
		} else {
			// Session已过期，跳转到登录界面
			c.HTML(http.StatusOK, "login.html", gin.H{
				"msg": "登录已过期，请重新登录后访问",
			})
			c.Abort()
		}
	} else {
		//未获得SessionID, 跳转到登录页面
		c.HTML(http.StatusOK, "login.html", gin.H{
			"msg": "请登录后访问",
		})
		c.Abort()
	}
}

func main() {
	r := gin.Default()
	rand.Seed(time.Now().Unix())            // 服务器每次重启根据当前时间重置随机数种子
	db, _ = sqlx.Open("sqlite3", "data.db") // 打开数据库

	r.LoadHTMLGlob("root/*") // 加载HTML模板根目录

	r.GET("/", func(c *gin.Context) {
		// 首页，无需登录
		// 检查登录状态，若已登录则显示个人中心，若未登录则显示登录界面
		var welcome, link string
		if SessionID, err := c.Cookie("SessionID"); err == nil {
			if info, OK := sb.get(SessionID); OK {
				// Session未过期，即已登录
				username := info["userID"].(string)
				welcome = "Welcome, " + username
				link = "personal_center"
			} else {
				// Session过期，视作未登录
				welcome = "您尚未登录"
				link = "login"
			}
		} else {
			// 无Session，即未登录
			welcome = "您尚未登录"
			link = "login"
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"welcome": welcome,
			"link":    link,
		})

	})
	r.GET("/login.html", Midware_Auth, func(c *gin.Context) {
		// 登录页面，若已登录则直接跳转到首页
		if login_status, exist := c.Get("login_status"); exist && login_status.(bool) {
			userID := c.GetString("userID")
			c.HTML(http.StatusOK, "index.html", gin.H{
				"welcome": "welcome" + userID,
				"link":    "personal_center",
			})
		} else {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"msg": "请登录后访问",
			})
		}
	})
	r.POST("/login", func(c *gin.Context) {
		// 登录页面处理，暂时只有一套账号密码，后续从数据库中读取
		login := c.PostForm("login")
		passwd_get := c.PostForm("pass")
		sql := "SELECT * FROM user WHERE userID=" + login
		query_res := query(sql)
		if len(query_res) == 0 {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"msg": "用户不存在！请再次尝试。",
			})
			c.Abort()
		}
		passwd_need := query_res[0]["passwd"].(string)
		if passwd_get == passwd_need {
			newcookie := produce_cookie()
			c.SetCookie("SessionID", newcookie, 3600, "/", "localhost", false, true)
			sb.set(newcookie, gin.H{
				"userID": login,
				"due":    time.Now().Unix() + valid_time,
			})
			c.Redirect(http.StatusTemporaryRedirect, "/home.html")
		} else {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"msg": "密码错误，请再次尝试。",
			})
		}
	})

	r.GET("/home.html", Midware_Auth, func(c *gin.Context) {
		// 后台页面，需要登录
		userID := c.GetString("userID")

		c.HTML(http.StatusOK, "home.html", gin.H{
			"msg": "Welcome, " + userID,
		})
	})

	r.POST("/home.html", Midware_Auth, func(c *gin.Context) {
		// 后台页面，需要登录
		userID := c.GetString("userID")

		c.HTML(http.StatusOK, "home.html", gin.H{
			"msg": "Welcome, " + userID,
		})
	})

	r.GET("/logout", func(c *gin.Context) {
		// 退出登录
		if SessionID, err := c.Cookie("SessionID"); err == nil {
			sb.del(SessionID)
		}

		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	r.Run(":4203") // Listening at http://localhost:4203
}
