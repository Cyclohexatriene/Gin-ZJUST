package main

import (
	//"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type session_base struct {
	m       sync.Map
	user2ID sync.Map
	/*
		键：字符串类型，SessionID
		值：gin.H 类型，用户名（userID)，过期时间(due)
	*/
}

func (sb *session_base) set(id string, info gin.H) {
	if val, ok := info["userID"]; ok {
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

var sb session_base       // Session数据库
var valid_time int64 = 10 //Session有效时间（秒）

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
	if _, exist := sb.get(res.String()); exist == true {
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
		} else {
			// Session已过期，跳转到登录界面
			c.HTML(http.StatusOK, "Login.html", gin.H{
				"msg": "登录已过期，请重新登录后访问",
			})
			c.Abort()
		}
	} else {
		//未获得SessionID, 跳转到登录页面
		c.HTML(http.StatusOK, "Login.html", gin.H{
			"msg": "请登录后访问",
		})
		c.Abort()
	}
}

func main() {
	r := gin.Default()
	rand.Seed(time.Now().Unix())

	r.LoadHTMLGlob("root/*")

	r.GET("/", Midware_Auth, func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"msg": "Login Test",
		})
	})

	r.POST("/login", func(c *gin.Context) {
		login := c.PostForm("login")
		passwd := c.PostForm("pass")
		if login == "3200104203" && passwd == "4203" {
			newcookie := produce_cookie()
			c.SetCookie("SessionID", newcookie, 3600, "/", "localhost", false, true)
			sb.set(newcookie, gin.H{
				"userID": login,
				"due":    time.Now().Unix() + valid_time,
			})
			c.HTML(http.StatusOK, "index.html", gin.H{
				"msg": "登录成功",
			})
		} else {
			c.HTML(http.StatusOK, "Login.html", gin.H{
				"msg": "密码错误，请再次尝试",
			})
		}
	})
	r.Run(":4203") // Listening at http://localhost:4203
}
