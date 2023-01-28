package main

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var sb session_base         // Session库对象
var valid_time int64 = 1800 // Session有效时间（秒）
var db *sqlx.DB             // 数据库对象

type session_base struct {
	m sync.Map
	/* 键：字符串类型，SessionID
	 * 值：gin.H 类型，用户名（userID)，过期时间(due) */

	user2ID sync.Map
}

func (sb *session_base) set(id string, info gin.H) {
	if val, ok := sb.user2ID.Load(info["userID"]); ok {
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

func exec(sql string) bool {
	_, err := db.Exec(sql)
	return err == nil
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

func strcat(a, b string) string {
	return a + b
}
func strcat1(a string, b int64) string {
	c := int(b)
	return a + strconv.Itoa(c)
}

func main() {
	r := gin.Default()
	r.SetFuncMap(template.FuncMap{
		"strcat":  strcat,
		"strcat1": strcat1,
	})
	rand.Seed(time.Now().Unix())            // 服务器每次重启根据当前时间重置随机数种子
	db, _ = sqlx.Open("sqlite3", "data.db") // 打开数据库

	account_types := map[int64]string{
		0: "超级管理员",
		1: "校级管理员",
		2: "单位管理员",
		3: "学院管理员",
		4: "团支部管理员",
		5: "学生",
	}

	org_type := map[int64]string{
		0: "学校",
		1: "单位",
		2: "学院",
		3: "团支部",
	}

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
		// 登录页面处理
		login := c.PostForm("login")
		passwd_get := c.PostForm("pass")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\"", login)
		query_res := query(sql)
		if len(query_res) == 0 {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"msg": "用户不存在！请再次尝试。",
			})
			c.Abort()
		} else {
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
		}
	})

	r.GET("/home.html", Midware_Auth, func(c *gin.Context) {
		// 后台页面，需要登录
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		query_res := query(sql)
		account_type := query_res[0]["account_type"].(int64)
		var add_item, add_basic_item, apply, audit_added, audit_basic, check_branch_info,
			check_record, check_student_info, create_new_org, create_new_manager, item_anal, manage_self_info int
		set_authorities := func(a int) {
			// 根据变量定义顺序，从低位到高位依次赋值
			varieties := []*int{&add_item, &add_basic_item, &apply, &audit_added, &audit_basic, &check_branch_info, &check_record, &check_student_info, &create_new_org, &create_new_manager, &item_anal, &manage_self_info}
			idx := 0
			for a > 0 {
				if a&1 == 1 {
					*varieties[idx] = 1
				}
				a >>= 1
				idx++
			}
		}
		if account_type == 0 {
			// 超级管理员
			set_authorities(0b111110011010)
		} else if account_type == 1 {
			// 校级管理员
			set_authorities(0b111110011000)
		} else if account_type == 2 {
			// 单位管理员
			set_authorities(0b100000000001)
		} else if account_type == 3 {
			// 学院管理员
			set_authorities(0b100010110001)
		} else if account_type == 4 {
			// 团支部管理员
			set_authorities(0b100010010000)
		} else if account_type == 5 {
			// 普通学生
			set_authorities(0b100001000100)
		}

		c.HTML(http.StatusOK, "home.html", gin.H{
			"msg":                "Welcome, " + userID,
			"add_item":           add_item,
			"add_basic_item":     add_basic_item,
			"apply":              apply,
			"audit_added":        audit_added,
			"audit_basic":        audit_basic,
			"check_branch_info":  check_branch_info,
			"check_record":       check_record,
			"check_student_info": check_student_info,
			"create_new_org":     create_new_org,
			"create_new_manager": create_new_manager,
			"item_anal":          item_anal,
			"manage_self_info":   manage_self_info,
		})
	})

	r.POST("/home.html", Midware_Auth, func(c *gin.Context) {
		// 后台页面，需要登录
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		query_res := query(sql)
		account_type := query_res[0]["account_type"].(int64)
		var add_item, add_basic_item, apply, audit_added, audit_basic, check_branch_info,
			check_record, check_student_info, create_new_org, create_new_manager, item_anal, manage_self_info int
		set_authorities := func(a int) {
			// 根据变量定义顺序，从低位到高位依次赋值
			varieties := []*int{&add_item, &add_basic_item, &apply, &audit_added, &audit_basic, &check_branch_info, &check_record, &check_student_info, &create_new_org, &create_new_manager, &item_anal, &manage_self_info}
			idx := 0
			for a > 0 {
				if a&1 == 1 {
					*varieties[idx] = 1
				}
				a >>= 1
				idx++
			}
		}
		if account_type == 0 {
			// 超级管理员
			set_authorities(0b111110011010)
		} else if account_type == 1 {
			// 校级管理员
			set_authorities(0b111110011000)
		} else if account_type == 2 {
			// 单位管理员
			set_authorities(0b100000000001)
		} else if account_type == 3 {
			// 学院管理员
			set_authorities(0b100010110001)
		} else if account_type == 4 {
			// 团支部管理员
			set_authorities(0b100010010000)
		} else if account_type == 5 {
			// 普通学生
			set_authorities(0b100001000100)
		}

		c.HTML(http.StatusOK, "home.html", gin.H{
			"msg":                "Welcome, " + userID,
			"add_item":           add_item,
			"add_basic_item":     add_basic_item,
			"apply":              apply,
			"audit_added":        audit_added,
			"audit_basic":        audit_basic,
			"check_branch_info":  check_branch_info,
			"check_record":       check_record,
			"check_student_info": check_student_info,
			"create_new_org":     create_new_org,
			"create_new_manager": create_new_manager,
			"item_anal":          item_anal,
			"manage_self_info":   manage_self_info,
		})
	})

	r.GET("/logout", func(c *gin.Context) {
		// 退出登录
		if SessionID, err := c.Cookie("SessionID"); err == nil {
			sb.del(SessionID)
		}
		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	r.GET("/add_basic_item.html", Midware_Auth, func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := "SELECT name,score_lower_range,score_higher_range,create_org,description FROM item WHERE type=0;"
		query_res := query(sql)
		c.HTML(http.StatusOK, "add_basic_item.html", gin.H{
			"msg":   "welcome, " + userID,
			"added": query_res,
		})
	})
	r.POST("/add_basic_item", Midware_Auth, func(c *gin.Context) {
		userID := c.GetString("userID")
		item_name := c.PostForm("name")
		var msg string
		sql := fmt.Sprintf("SELECT * FROM item WHERE name=\"%s\";", item_name)
		query_res := query(sql)
		if len(query_res) == 0 {
			score_lower_range, _ := strconv.ParseFloat(c.PostForm("score_lower_range"), 64)
			score_higher_range, _ := strconv.ParseFloat(c.PostForm("score_higher_range"), 64)
			org_name := query("SELECT * FROM user WHERE userID=" + userID)[0]["belonging_org"].(string)
			description := c.PostForm("description")
			sql = fmt.Sprintf("INSERT INTO item VALUES(NULL,0,0,\"%s\",%.1f,%.1f,\"%s\",\"%s\",NULL);", item_name, score_lower_range, score_higher_range, org_name, description)
			ok := exec(sql)

			if ok {
				msg = "添加成功！"
			} else {
				msg = "添加失败，请重试"
			}
		} else {
			msg = "添加失败。项目已存在！"
		}
		sql = "SELECT name,score_lower_range,score_higher_range,create_org,description FROM item WHERE type=0;"
		query_res = query(sql)
		c.HTML(http.StatusOK, "add_basic_item.html", gin.H{
			"msg":   msg,
			"added": query_res,
		})
	})

	r.GET("/delete_item", Midware_Auth, func(c *gin.Context) {
		to_delete := c.Query("name")
		sql := fmt.Sprintf("DELETE FROM item WHERE name=\"%s\";", to_delete)
		exec(sql)
		sql = "SELECT name,score_lower_range,score_higher_range,create_org,description FROM item WHERE type=0;"
		query_res := query(sql)
		c.HTML(http.StatusOK, "add_basic_item.html", gin.H{
			"msg":   "删除成功！",
			"added": query_res,
		})

	})

	r.GET("/create_new_manager.html", Midware_Auth, func(c *gin.Context) {
		orgs := query("SELECT orgID,name FROM organization;")
		admins := query("SELECT user.userID AS userID,user.account_type AS account_type, organization.name AS belonging_org FROM user,organization WHERE organization.orgID=user.belonging_org AND (account_type=1 OR account_type=2 OR account_type=3 OR account_type=4);")
		for _, admin := range admins {
			admin["account_type"] = account_types[admin["account_type"].(int64)]
		}
		c.HTML(http.StatusOK, "create_new_manager.html", gin.H{
			"msg":    "",
			"orgs":   orgs,
			"admins": admins,
		})
	})

	r.POST("/create_new_manager", Midware_Auth, func(c *gin.Context) {
		name := c.PostForm("name")
		default_passwd := "123456"
		admin_type, _ := strconv.Atoi(c.PostForm("type"))
		belonging_org, _ := strconv.Atoi(c.PostForm("belonging_org"))
		sql := fmt.Sprintf("INSERT INTO user VALUES(\"%s\",\"%s\",%d,%d);", name, default_passwd, admin_type, belonging_org)
		ok := exec(sql)
		orgs := query("SELECT orgID,name FROM organization;")
		admins := query("SELECT user.userID AS userID,user.account_type AS account_type, organization.name AS belonging_org FROM user,organization WHERE organization.orgID=user.belonging_org AND (account_type=1 OR account_type=2 OR account_type=3 OR account_type=4);")
		for _, admin := range admins {
			admin["account_type"] = account_types[admin["account_type"].(int64)]
		}
		var msg string
		if ok {
			msg = "添加成功！"
		} else {
			msg = "添加失败"
		}
		c.HTML(http.StatusOK, "create_new_manager.html", gin.H{
			"msg":    msg,
			"orgs":   orgs,
			"admins": admins,
		})
	})

	r.GET("/delete_admin", Midware_Auth, func(c *gin.Context) {
		userID := c.Query("userID")
		sql := fmt.Sprintf("DELETE FROM user WHERE userID=\"%s\"", userID)
		ok := exec(sql)
		var msg string
		if ok {
			msg = "删除成功！"
		} else {
			msg = "删除失败"
		}
		orgs := query("SELECT orgID,name FROM organization;")
		admins := query("SELECT user.userID AS userID,user.account_type AS account_type, organization.name AS belonging_org FROM user,organization WHERE organization.orgID=user.belonging_org AND (account_type=1 OR account_type=2 OR account_type=3 OR account_type=4);")
		for _, admin := range admins {
			admin["account_type"] = account_types[admin["account_type"].(int64)]
		}
		c.HTML(http.StatusOK, "create_new_manager.html", gin.H{
			"msg":    msg,
			"orgs":   orgs,
			"admins": admins,
		})
	})

	r.GET("/manage_self_info.html", Midware_Auth, func(c *gin.Context) {
		c.HTML(http.StatusOK, "manage_self_info.html", gin.H{
			"msg":    "",
			"userID": c.GetString("userID"),
		})
	})
	r.POST("/change_passwd", Midware_Auth, func(c *gin.Context) {
		new_passwd := c.PostForm("new_passwd")
		userID := c.GetString("userID")
		sql := fmt.Sprintf("UPDATE user SET passwd=\"%s\" WHERE userID=\"%s\"", new_passwd, userID)
		ok := exec(sql)
		msg := ""
		if ok {
			msg = "修改成功！"
			SessionID, _ := sb.user2ID.Load(userID)
			sb.del(SessionID.(string))
		} else {
			msg = "修改失败"
		}
		c.HTML(http.StatusOK, "manage_self_info.html", gin.H{
			"msg":    msg,
			"userID": userID,
		})
	})

	r.GET("/create_new_org.html", Midware_Auth, func(c *gin.Context) {
		orgs := query("SELECT a.orgID AS orgID,a.name AS name,a.type AS type, b.name AS higher_org FROM organization AS a LEFT JOIN organization AS b WHERE a.higher_org=b.orgID;")
		for _, org := range orgs {
			org["type"] = org_type[org["type"].(int64)]
		}
		c.HTML(http.StatusOK, "create_new_org.html", gin.H{
			"msg":  "",
			"orgs": orgs,
		})
	})

	r.POST("/create_new_organization", Midware_Auth, func(c *gin.Context) {
		org_name := c.PostForm("name")
		org_mtype := c.PostForm("type")
		higher_org := c.PostForm("belonging_org")
		sql := fmt.Sprintf("SELECT * FROM organization WHERE name=\"%s\";", org_name)
		query_res := query(sql)
		msg := ""
		if len(query_res) > 0 {
			msg = "添加失败：名称重复！"
		} else {
			sql = fmt.Sprintf("INSERT INTO organization VALUES(NULL,\"%s\",%s,%s);", org_name, org_mtype, higher_org)
			ok := exec(sql)
			sql = fmt.Sprintf("SELECT orgID FROM organization WHERE name=\"%s\";", org_name)
			orgID := query(sql)[0]["orgID"].(int64)
			orgtp, _ := strconv.Atoi(org_mtype)
			sql = fmt.Sprintf("INSERT INTO user VALUES(\"%s\",\"123456\",%d,%d);", org_name, orgtp+1, orgID)
			ok1 := exec(sql)
			if ok && !ok1 {
				sql = fmt.Sprintf("DELETE FROM organization WHERE orgID=%d", orgID)
				exec(sql)
				ok = false
			}

			if ok {
				msg = "添加成功！"
			} else {
				msg = "添加失败"
			}
		}
		orgs := query("SELECT a.orgID AS orgID,a.name AS name,a.type AS type, b.name AS higher_org FROM organization AS a LEFT JOIN organization AS b WHERE a.higher_org=b.orgID;")
		for _, org := range orgs {
			org["type"] = org_type[org["type"].(int64)]
		}
		c.HTML(http.StatusOK, "create_new_org.html", gin.H{
			"msg":  msg,
			"orgs": orgs,
		})
	})

	r.GET("/delete_org", Midware_Auth, func(c *gin.Context) {
		to_delete := c.Query("orgID")
		sql := fmt.Sprintf("DELETE FROM organization WHERE orgID=%s;", to_delete)
		ok := exec(sql)
		sql = fmt.Sprintf("DELETE FROM user WHERE belonging_org=%s;", to_delete)
		ok1 := exec(sql)
		ok = ok && ok1
		msg := ""
		if ok {
			msg = "删除成功！"
		} else {
			msg = "删除失败"
		}
		orgs := query("SELECT a.orgID AS orgID,a.name AS name,a.type AS type, b.name AS higher_org FROM organization AS a LEFT JOIN organization AS b WHERE a.higher_org=b.orgID;")
		for _, org := range orgs {
			org["type"] = org_type[org["type"].(int64)]
		}
		c.HTML(http.StatusOK, "create_new_org.html", gin.H{
			"msg":  msg,
			"orgs": orgs,
		})
	})

	r.GET("/check_branch_info.html", Midware_Auth, func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT orgID FROM organization WHERE name=\"%s\"", userID)
		orgID := query(sql)[0]["orgID"].(int64)
		sql = fmt.Sprintf("SELECT * FROM organization WHERE higher_org=%d", orgID)
		branches := query(sql)
		c.HTML(http.StatusOK, "check_branch_info.html", gin.H{
			"msg":      "",
			"userID":   userID,
			"branches": branches,
		})
	})

	r.POST("/create_new_branch", Midware_Auth, func(c *gin.Context) {
		collegeID := c.GetString("userID")
		userID := c.PostForm("name")
		sql := fmt.Sprintf("SELECT * FROM organization WHERE name=\"%s\";", userID)
		query_res := query(sql)
		msg := ""
		var orgID int64
		if len(query_res) > 0 {
			msg = "添加失败：名称重复！"
		} else {
			sql = fmt.Sprintf("SELECT orgID FROM organization WHERE name=\"%s\";", collegeID)
			orgID = query(sql)[0]["orgID"].(int64)
			sql = fmt.Sprintf("INSERT INTO organization VALUES(NULL,\"%s\",3,%d);", userID, orgID)
			ok := exec(sql)
			sql = fmt.Sprintf("SELECT orgID FROM organization WHERE name=\"%s\";", userID)
			branchID := query(sql)[0]["orgID"].(int64)
			sql = fmt.Sprintf("INSERT INTO user VALUES(\"%s\",\"123456\",4,%d);", userID, branchID)
			ok1 := exec(sql)
			ok = ok && ok1
			if ok {
				msg = "添加成功！"
			} else {
				msg = "添加失败"
			}
		}
		sql = fmt.Sprintf("SELECT * FROM organization WHERE higher_org=%d", orgID)
		branches := query(sql)
		c.HTML(http.StatusOK, "check_branch_info.html", gin.H{
			"msg":      msg,
			"userID":   collegeID,
			"branches": branches,
		})
	})

	r.GET("/delete_branch", Midware_Auth, func(c *gin.Context) {
		to_delete := c.Query("branchID")
		sql := fmt.Sprintf("DELETE FROM organization WHERE orgID=%s;", to_delete)
		ok := exec(sql)
		sql = fmt.Sprintf("DELETE FROM user WHERE belonging_org=%s;", to_delete)
		ok = ok && exec(sql)
		msg := ""
		if ok {
			msg = "删除成功！"
		} else {
			msg = "删除失败"
		}
		collegeID := c.GetString("userID")
		sql = fmt.Sprintf("SELECT orgID FROM organization WHERE name=\"%s\";", collegeID)
		orgID := query(sql)[0]["orgID"].(int64)
		sql = fmt.Sprintf("SELECT * FROM organization WHERE higher_org=%d", orgID)
		branches := query(sql)
		c.HTML(http.StatusOK, "check_branch_info.html", gin.H{
			"msg":      msg,
			"userID":   collegeID,
			"branches": branches,
		})
	})

	r.Run(":4203") // Listening at http://localhost:4203
}
