package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
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

func Authorities(auth int) gin.HandlerFunc {
	// 从高位到低位依次代表学生用户、团支部账号、学院账号、单位账号、校级账号、超级管理员是否拥有访问权限
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT account_type FROM user WHERE userID=\"%s\";", userID)
		account_type := query(sql)[0]["account_type"].(int64)
		if auth&(1<<account_type) == 0 {
			c.String(http.StatusOK, "权限不足！")
			c.Abort()
		} else {
			c.Set("account_type", account_type)
		}
	}
}

func strcat(a, b string) string {
	return a + b
}
func strcat1(a string, b int64) string {
	c := int(b)
	return a + strconv.Itoa(c)
}
func get_file_name(a string) string {
	b := strings.Split(a, "/")
	return b[len(b)-1]
}

func main() {
	r := gin.Default()
	r.SetFuncMap(template.FuncMap{
		"strcat":        strcat,
		"strcat1":       strcat1,
		"get_file_name": get_file_name,
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

	item_types := map[int64]string{
		0: "第二课堂",
		1: "第三课堂",
		2: "第二课堂",
		3: "第三课堂",
	}

	appliance_status := map[int64]string{
		0: "团支部待审核",
		1: "团支部审核通过",
		2: "团支部审核不通过",
		3: "学院审核通过",
		4: "学院审核不通过",
		5: "学校审核通过",
		6: "学校审核不通过",
	}

	item_status := map[int64]string{
		1: "待审核",
		2: "预审核通过",
		3: "预审核不通过",
		4: "审核通过",
		5: "审核不通过",
	}

	to_audit_map := map[int64]int64{ // 管理员类型 to 可操作项目状态
		0: 3,
		1: 3, //校级管理员和超级管理员可审核学院审核已通过的项目
		3: 1, //学院管理员可审核团支部审核已通过的项目
		4: 0, //团支部管理员可审核尚未进行团支部审核的项目
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
	r.GET("/login.html", Midware_Auth, Authorities(0b111111), func(c *gin.Context) {
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

	r.GET("/home.html", Midware_Auth, Authorities(0b111111), func(c *gin.Context) {
		// 后台页面，需要登录
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		query_res := query(sql)
		account_type := query_res[0]["account_type"].(int64)
		var add_item, add_basic_item, apply, audit_added, audit_basic, check_branch_info,
			check_record, check_student_info, create_new_org, create_new_manager, item_anal, manage_self_info, import_new_student int
		set_authorities := func(a int) {
			// 根据变量定义顺序，从低位到高位依次赋值
			varieties := []*int{&add_item, &add_basic_item, &apply, &audit_added, &audit_basic, &check_branch_info, &check_record, &check_student_info, &create_new_org, &create_new_manager, &item_anal, &manage_self_info, &import_new_student}
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
			set_authorities(0b0111110011010)
		} else if account_type == 1 {
			// 校级管理员
			set_authorities(0b0111110011000)
		} else if account_type == 2 {
			// 单位管理员
			set_authorities(0b0100000000001)
		} else if account_type == 3 {
			// 学院管理员
			set_authorities(0b0100010110001)
		} else if account_type == 4 {
			// 团支部管理员
			set_authorities(0b1100010010000)
		} else if account_type == 5 {
			// 普通学生
			set_authorities(0b0100001000100)
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
			"import_new_student": import_new_student,
		})
	})

	r.POST("/home.html", Midware_Auth, Authorities(0b111111), func(c *gin.Context) {
		// 后台页面，需要登录
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		query_res := query(sql)
		account_type := query_res[0]["account_type"].(int64)
		var add_item, add_basic_item, apply, audit_added, audit_basic, check_branch_info,
			check_record, check_student_info, create_new_org, create_new_manager, item_anal, manage_self_info, import_new_student int
		set_authorities := func(a int) {
			// 根据变量定义顺序，从低位到高位依次赋值
			varieties := []*int{&add_item, &add_basic_item, &apply, &audit_added, &audit_basic, &check_branch_info, &check_record, &check_student_info, &create_new_org, &create_new_manager, &item_anal, &manage_self_info, &import_new_student}
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
			set_authorities(0b0111110011010)
		} else if account_type == 1 {
			// 校级管理员
			set_authorities(0b0111110011000)
		} else if account_type == 2 {
			// 单位管理员
			set_authorities(0b0100000000001)
		} else if account_type == 3 {
			// 学院管理员
			set_authorities(0b0100010110001)
		} else if account_type == 4 {
			// 团支部管理员
			set_authorities(0b1100010010000)
		} else if account_type == 5 {
			// 普通学生
			set_authorities(0b0100001000100)
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
			"import_new_student": import_new_student,
		})
	})

	r.GET("/logout", func(c *gin.Context) {
		// 退出登录
		if SessionID, err := c.Cookie("SessionID"); err == nil {
			sb.del(SessionID)
		}
		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	r.GET("/add_basic_item.html", Midware_Auth, Authorities(0b000001), func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := "SELECT * FROM item WHERE type=0 OR type=1;"
		query_res := query(sql)
		for _, item := range query_res {
			item["type"] = item_types[item["type"].(int64)]
		}
		c.HTML(http.StatusOK, "add_basic_item.html", gin.H{
			"msg":   "welcome, " + userID,
			"added": query_res,
		})
	})
	r.POST("/add_basic_item", Midware_Auth, Authorities(0b000001), func(c *gin.Context) {
		userID := c.GetString("userID")
		item_name := c.PostForm("name")
		var msg string
		sql := fmt.Sprintf("SELECT * FROM item WHERE name=\"%s\";", item_name)
		query_res := query(sql)
		if len(query_res) == 0 {
			score_lower_range, _ := strconv.ParseFloat(c.PostForm("score_lower_range"), 64)
			score_higher_range, _ := strconv.ParseFloat(c.PostForm("score_higher_range"), 64)
			tp := c.PostForm("type")
			orgID := query("SELECT * FROM user WHERE userID=" + userID)[0]["belonging_org"].(int64)
			description := c.PostForm("description")
			sql = fmt.Sprintf("INSERT INTO item VALUES(NULL,%s,0,\"%s\",%.1f,%.1f,%d,\"%s\",%d,\"\");", tp, item_name, score_lower_range, score_higher_range, orgID, description, time.Now().Unix())
			ok := exec(sql)
			if ok {
				msg = "添加成功！"
			} else {
				msg = "添加失败，请重试"
			}
		} else {
			msg = "添加失败。项目已存在！"
		}
		sql = "SELECT * FROM item WHERE type=0 OR type=1;"
		query_res = query(sql)
		for _, item := range query_res {
			item["type"] = item_types[item["type"].(int64)]
		}
		c.HTML(http.StatusOK, "add_basic_item.html", gin.H{
			"msg":   msg,
			"added": query_res,
		})
	})

	r.GET("/delete_basic_item", Midware_Auth, Authorities(0b000001), func(c *gin.Context) {
		to_delete := c.Query("name")
		sql := fmt.Sprintf("DELETE FROM item WHERE name=\"%s\";", to_delete)
		exec(sql)
		sql = "SELECT name,score_lower_range,score_higher_range,create_org,description FROM item WHERE type=0 OR type=1;"
		query_res := query(sql)
		c.HTML(http.StatusOK, "add_basic_item.html", gin.H{
			"msg":   "删除成功！",
			"added": query_res,
		})

	})

	r.GET("/create_new_manager.html", Midware_Auth, Authorities(0b000001), func(c *gin.Context) {
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

	r.POST("/create_new_manager", Midware_Auth, Authorities(0b000001), func(c *gin.Context) {
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

	r.GET("/delete_admin", Midware_Auth, Authorities(0b000001), func(c *gin.Context) {
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

	r.GET("/manage_self_info.html", Midware_Auth, Authorities(0b111111), func(c *gin.Context) {
		c.HTML(http.StatusOK, "manage_self_info.html", gin.H{
			"msg":    "",
			"userID": c.GetString("userID"),
		})
	})
	r.POST("/change_passwd", Midware_Auth, Authorities(0b111111), func(c *gin.Context) {
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

	r.GET("/create_new_org.html", Midware_Auth, Authorities(0b000011), func(c *gin.Context) {
		orgs := query("SELECT a.orgID AS orgID,a.name AS name,a.type AS type, b.name AS higher_org FROM organization AS a LEFT JOIN organization AS b WHERE a.higher_org=b.orgID;")
		for _, org := range orgs {
			org["type"] = org_type[org["type"].(int64)]
		}
		c.HTML(http.StatusOK, "create_new_org.html", gin.H{
			"msg":  "",
			"orgs": orgs,
		})
	})

	r.POST("/create_new_organization", Midware_Auth, Authorities(0b000011), func(c *gin.Context) {
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

	r.GET("/delete_org", Midware_Auth, Authorities(0b000011), func(c *gin.Context) {
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

	r.GET("/check_branch_info.html", Midware_Auth, Authorities(0b001000), func(c *gin.Context) {
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

	r.POST("/create_new_branch", Midware_Auth, Authorities(0b001000), func(c *gin.Context) {
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

	r.GET("/delete_branch", Midware_Auth, Authorities(0b001000), func(c *gin.Context) {
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

	r.GET("/check_student_info.html", Midware_Auth, Authorities(0b011011), func(c *gin.Context) {
		//根据不同类型的组织查询管辖范围内的学生
		userID := c.GetString("userID")
		account_type := c.GetInt64("account_type")
		var stus []map[string]any
		var sql string
		if account_type == 4 {
			sql = fmt.Sprintf("SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.belonging_org=organization.orgID AND user.userID!=\"%s\" AND organization.name=\"%s\";", userID, userID)
			stus = query(sql)
		} else if account_type == 3 {
			sql = fmt.Sprintf("SELECT orgID from organization WHERE name=\"%s\";", userID)
			orgID := query(sql)[0]["orgID"].(int64)
			sql = fmt.Sprintf("SELECT orgID,name from organization WHERE higher_org=%d;", orgID)
			branches := query(sql)
			for _, branch := range branches {
				sql = fmt.Sprintf("SELECT userID AS name FROM user WHERE belonging_org=%d AND userID!=\"%s\";", branch["orgID"].(int64), branch["name"])
				temp := query(sql)
				for _, t := range temp {
					t["belonging_org"] = branch["name"]
					stus = append(stus, t)
				}
			}
		} else if account_type == 1 || account_type == 0 {
			sql = "SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.account_type=5 AND user.belonging_org=organization.orgID AND user.userID!=organization.name ;"
			stus = query(sql)
		}

		c.HTML(http.StatusOK, "check_student_info.html", gin.H{
			"msg":  "",
			"stus": stus,
		})
	})

	r.GET("/delete_stu", Midware_Auth, Authorities(0b011011), func(c *gin.Context) {
		to_delete := c.Query("name")
		userID := c.GetString("userID")
		account_type := c.GetInt64("account_type")
		msg := ""
		if account_type == 0 || account_type == 1 {
			// 学校管理员、超级管理员，可删除所有学生
			sql := fmt.Sprintf("DELETE FROM user WHERE userID=\"%s\";", to_delete)
			ok := exec(sql)
			if ok {
				msg = "删除成功！"
			} else {
				msg = "删除失败"
			}
		} else if account_type == 3 {
			// 学院管理员
			sql := fmt.Sprintf("SELECT belonging_org FROM user WHERE userID=\"%s\";", userID)
			belonging_branch := query(sql)[0]["belonging_org"].(int64)
			sql = fmt.Sprintf("SELECT higher_org FROM organization WHERE orgID=%d", belonging_branch)
			belonging_college := query(sql)[0]["higher_org"].(int64)
			sql = fmt.Sprintf("SELECT name FROM organization WHERE orgID=%d", belonging_college)
			college_name := query(sql)[0]["name"].(string)
			if college_name == userID {
				sql := fmt.Sprintf("DELETE FROM user WHERE userID=\"%s\";", to_delete)
				ok := exec(sql)
				if ok {
					msg = "删除成功！"
				} else {
					msg = "删除失败"
				}
			} else {
				msg = "删除失败：权限不足。"
			}
		} else if account_type == 4 {
			sql := fmt.Sprintf("SELECT belonging_org FROM user WHERE userID=\"%s\";", userID)
			belonging_branch := query(sql)[0]["belonging_org"].(int64)
			sql = fmt.Sprintf("SELECT name FROM organization WHERE orgID=%d", belonging_branch)
			branch_name := query(sql)[0]["name"].(string)
			if branch_name == userID {
				sql := fmt.Sprintf("DELETE FROM user WHERE userID=\"%s\";", to_delete)
				ok := exec(sql)
				if ok {
					msg = "删除成功！"
				} else {
					msg = "删除失败"
				}
			} else {
				msg = "删除失败：权限不足。"
			}
		}

		var stus []map[string]any
		var sql string
		if account_type == 4 {
			sql = fmt.Sprintf("SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.belonging_org=organization.orgID AND user.userID!=\"%s\" AND organization.name=\"%s\";", userID, userID)
			stus = query(sql)
		} else if account_type == 3 {
			sql = fmt.Sprintf("SELECT orgID from organization WHERE name=\"%s\";", userID)
			orgID := query(sql)[0]["orgID"].(int64)
			sql = fmt.Sprintf("SELECT orgID,name from organization WHERE higher_org=%d;", orgID)
			branches := query(sql)
			for _, branch := range branches {
				sql = fmt.Sprintf("SELECT userID AS name FROM user WHERE belonging_org=%d AND userID!=\"%s\";", branch["orgID"].(int64), branch["name"])
				temp := query(sql)
				for _, t := range temp {
					t["belonging_org"] = branch["name"]
					stus = append(stus, t)
				}
			}
		} else if account_type == 1 || account_type == 0 {
			sql = "SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.account_type=5 AND user.belonging_org=organization.orgID AND user.userID!=organization.name ;"
			stus = query(sql)
		}

		c.HTML(http.StatusOK, "check_student_info.html", gin.H{
			"msg":  msg,
			"stus": stus,
		})
	})

	r.GET("/import_new_student.html", Midware_Auth, Authorities(0b010000), func(c *gin.Context) {
		c.HTML(http.StatusOK, "import_new_student.html", gin.H{
			"msg":         "",
			"branch_name": c.GetString("userID"),
		})
	})

	r.POST("/import_student", Midware_Auth, func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT orgID FROM organization WHERE name=\"%s\";", userID)
		orgID := query(sql)[0]["orgID"].(int64)
		student_name := c.PostForm("name")
		sql = fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", student_name)
		msg := ""
		if len(query(sql)) > 0 {
			msg = "添加失败：重复名称！"
		} else {
			sql = fmt.Sprintf("INSERT INTO user VALUES(\"%s\",\"123456\",5,%d);", student_name, orgID)
			ok := exec(sql)
			if ok {
				msg = "添加成功！"
			} else {
				msg = "添加失败"
			}
		}
		c.HTML(http.StatusOK, "import_new_student.html", gin.H{
			"msg":         msg,
			"branch_name": userID,
		})
	})

	r.GET("/apply.html", Midware_Auth, Authorities(0b100000), func(c *gin.Context) {
		sql := "SELECT * FROM item WHERE type=0 OR type=1;"
		items := query(sql)
		for _, item := range items {
			item["type"] = item_types[item["type"].(int64)]
		}
		c.HTML(http.StatusOK, "apply.html", gin.H{
			"msg":   "",
			"items": items,
		})
	})

	r.GET("/item_info", Midware_Auth, Authorities(0b100000), func(c *gin.Context) {
		itemID, _ := strconv.Atoi(c.Query("itemID"))
		sql := fmt.Sprintf("SELECT * from item WHERE itemID=%d", itemID)
		msg := ""
		item := query(sql)
		if len(item) == 0 {
			msg = "项目不存在！"
			c.HTML(http.StatusOK, "item_info.html", gin.H{
				"msg": msg,
			})
		} else {
			create_orgID := item[0]["create_org"].(int64)
			sql = fmt.Sprintf("SELECT * FROM organization WHERE orgID=%d", create_orgID)
			item[0]["create_org"] = query(sql)[0]["name"].(string)
			c.HTML(http.StatusOK, "item_info.html", gin.H{
				"msg":  msg,
				"item": item[0],
			})
		}
	})

	r.POST("/apply_item", Midware_Auth, Authorities(0b100000), func(c *gin.Context) {
		itemID := c.Query("ID")
		userID := c.GetString("userID")
		description := c.PostForm("description")
		cur_time := time.Now().Unix()
		sql := fmt.Sprintf("SELECT * FROM appliance WHERE userID=\"%s\" AND time_unix=%d;", userID, cur_time)
		msg := ""
		if len(query(sql)) > 0 {
			msg = "操作过于频繁，请稍候再试！"
		} else {
			sql = fmt.Sprintf("INSERT INTO appliance VALUES(NULL,%s,\"%s\",0,0,\"[]\",%d,\"%s\");", itemID, userID, cur_time, description)
			ok := exec(sql)
			if ok {
				form, _ := c.MultipartForm()
				files := form.File
				path := fmt.Sprintf("upload/basic/%s/%d/", userID, cur_time)
				_, err := os.Stat(path)
				if os.IsNotExist(err) {
					os.MkdirAll(path, os.ModePerm)
				}
				for _, file := range files {
					f, _ := file[0].Open()
					defer f.Close()
					c.SaveUploadedFile(file[0], path+file[0].Filename)
				}
				msg = "申请成功！"
			} else {
				msg = "申请失败"
			}
		}

		sql = fmt.Sprintf("SELECT * from item WHERE itemID=%s", itemID)
		item := query(sql)
		c.HTML(http.StatusOK, "item_info.html", gin.H{
			"msg":  msg,
			"item": item[0],
		})
	})

	r.GET("/check_record.html", Midware_Auth, Authorities(0b100000), func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT appliance.applianceID AS applianceID,item.name AS name,item.type AS type,appliance.score AS score,appliance.status AS status,appliance.record AS record,appliance.time_unix AS time_unix FROM appliance,item WHERE appliance.userID=\"%s\" AND appliance.itemID=item.itemID;", userID)
		appliances := query(sql)
		var sum2, sum3 float64
		for _, appliance := range appliances {
			if appliance["status"].(int64) == 5 {
				if appliance["type"].(int64)%2 == 0 {
					sum2 += appliance["score"].(float64)
				} else {
					sum3 += appliance["score"].(float64)
				}
			}
			appliance["type"] = item_types[appliance["type"].(int64)]
			appliance["status"] = appliance_status[appliance["status"].(int64)]

		}
		c.HTML(http.StatusOK, "check_record.html", gin.H{
			"msg":        "",
			"appliances": appliances,
			"sum2":       sum2,
			"sum3":       sum3,
		})
	})

	r.GET("/appliance_detail", Midware_Auth, Authorities(0b100000), func(c *gin.Context) {
		userID := c.GetString("userID")
		applianceID := c.Query("applianceID")
		sql := fmt.Sprintf("SELECT * FROM appliance WHERE applianceID=%s", applianceID)
		appliance := query(sql)
		msg := ""
		if len(appliance) == 0 {
			msg = "项目不存在！"
			c.HTML(http.StatusOK, "appliance_detail.html", gin.H{
				"msg": msg,
			})
		} else if appliance[0]["userID"].(string) != userID {
			msg = "非本人项目！"
			c.HTML(http.StatusOK, "appliance_detail.html", gin.H{
				"msg": msg,
			})
		} else {
			appliance[0]["status"] = appliance_status[appliance[0]["status"].(int64)]
			itemID := appliance[0]["itemID"].(int64)
			sql = fmt.Sprintf("SELECT * FROM item WHERE itemID=%d", itemID)
			item := query(sql)[0]
			records_json := appliance[0]["record"].(string)
			records := []map[string]any{}
			json.Unmarshal([]byte(records_json), &records)
			time := appliance[0]["time_unix"].(int64)
			path := "upload/basic/" + userID + "/" + strconv.Itoa(int(time)) + "/"
			dir, _ := os.ReadDir(path)
			paths := []string{}
			for _, file := range dir {
				if !file.IsDir() {
					paths = append(paths, path+file.Name())
				}
			}

			c.HTML(http.StatusOK, "appliance_detail.html", gin.H{
				"msg":       msg,
				"item":      item,
				"appliance": appliance[0],
				"records":   records,
				"paths":     paths,
			})
		}
	})

	r.GET("/delete_appliance", Midware_Auth, Authorities(0b100000), func(c *gin.Context) {
		userID := c.GetString("userID")
		applianceID := c.Query("applianceID")
		sql := fmt.Sprintf("SELECT * FROM appliance WHERE applianceID=%s", applianceID)
		appliance := query(sql)
		msg := ""
		if len(appliance) == 0 {
			msg = "项目不存在！"
		} else if appliance[0]["userID"].(string) != userID {
			msg = "非本人项目！"
		} else {
			sql = fmt.Sprintf("DELETE FROM appliance WHERE applianceID=%s", applianceID)
			ok := exec(sql)
			if ok {
				msg = "删除成功！"
				// 同时删除硬盘中存放的附件
			} else {
				msg = "删除失败！"
			}
		}

		sql = fmt.Sprintf("SELECT appliance.applianceID AS applianceID,item.name AS name,item.type AS type,appliance.score AS score,appliance.status AS status,appliance.record AS record,appliance.time_unix AS time_unix FROM appliance,item WHERE appliance.userID=\"%s\" AND appliance.itemID=item.itemID;", userID)
		appliances := query(sql)
		var sum2, sum3 float64
		for _, appliance := range appliances {
			if appliance["type"].(int64)%2 == 0 {
				sum2 += appliance["score"].(float64)
			} else {
				sum3 += appliance["score"].(float64)
			}
			appliance["type"] = item_types[appliance["type"].(int64)]
			appliance["status"] = appliance_status[appliance["status"].(int64)]

		}
		c.HTML(http.StatusOK, "check_record.html", gin.H{
			"msg":        msg,
			"appliances": appliances,
			"sum2":       sum2,
			"sum3":       sum3,
		})
	})
	r.GET("/get_file", Midware_Auth, func(c *gin.Context) {
		path := c.Query("path")
		fields := strings.Split(path, "/")
		if fields[0] != "upload" {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"路径有误！\"}")
			return
		}
		userID := c.GetString("userID")
		if fields[1] == "basic" {
			userID_get := fields[2]
			if userID != userID_get {
				c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
				return
			}
		} else if fields[1] == "activity" {
			orgID_get := fields[2]
			userID := c.GetString("userID")
			sql := fmt.Sprintf("SELECT belonging_org FROM user WHERE userID=\"%s\";", userID)
			orgID_need := query(sql)[0]["belonging_org"].(int64)
			a, ok := strconv.Atoi(orgID_get)
			if ok != nil || int64(a) != orgID_need {
				c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
				return
			}
		} else {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"路径有误！\"}")
			return
		}

		c.File(path)
	})

	r.GET("/audit_basic.html", Midware_Auth, Authorities(0b011011), func(c *gin.Context) {
		// 根据不同管理员类型检索出管辖范围内的学生
		userID := c.GetString("userID")
		account_type := c.GetInt64("account_type")
		var stus []map[string]any
		var sql string
		if account_type == 4 {
			sql = fmt.Sprintf("SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.belonging_org=organization.orgID AND user.userID!=\"%s\" AND organization.name=\"%s\";", userID, userID)
			stus = query(sql)
		} else if account_type == 3 {
			sql = fmt.Sprintf("SELECT orgID from organization WHERE name=\"%s\";", userID)
			orgID := query(sql)[0]["orgID"].(int64)
			sql = fmt.Sprintf("SELECT orgID,name from organization WHERE higher_org=%d;", orgID)
			branches := query(sql)
			for _, branch := range branches {
				sql = fmt.Sprintf("SELECT userID AS name FROM user WHERE belonging_org=%d AND userID!=\"%s\";", branch["orgID"].(int64), branch["name"])
				temp := query(sql)
				for _, t := range temp {
					t["belonging_org"] = branch["name"]
					stus = append(stus, t)
				}
			}
		} else if account_type == 1 || account_type == 0 {
			sql = "SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.account_type=5 AND user.belonging_org=organization.orgID AND user.userID!=organization.name ;"
			stus = query(sql)
		}

		// 检索所有需要审核的申请

		appliances := []map[string]any{}
		to_audit := to_audit_map[account_type]
		for _, stu := range stus {
			sql = fmt.Sprintf("SELECT ap.applianceID AS applianceID,ap.userID AS userID,item.name AS item,item.type AS type,ap.score AS score,ap.description AS description,ap.status AS status FROM appliance AS ap,item WHERE ap.itemID=item.itemID AND ap.status=%d AND ap.userID=\"%s\";", to_audit, stu["name"])
			temp := query(sql)
			appliances = append(appliances, temp...)
		}

		aps := []map[string]any{}
		for _, ap := range appliances {
			ap["type"] = item_types[ap["type"].(int64)]
			ap["status"] = appliance_status[ap["status"].(int64)]
			aps = append(aps, ap)
		}
		c.HTML(http.StatusOK, "audit_basic.html", gin.H{
			"msg":          "",
			"to_audit_sum": len(aps),
			"appliances":   aps,
		})

	})

	r.POST("/audit_basic.html", Midware_Auth, Authorities(0b011011), func(c *gin.Context) {
		// 根据不同管理员类型检索出管辖范围内的学生
		userID := c.GetString("userID")
		account_type := c.GetInt64("account_type")
		var stus []map[string]any
		var sql string
		if account_type == 4 {
			sql = fmt.Sprintf("SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.belonging_org=organization.orgID AND user.userID!=\"%s\" AND organization.name=\"%s\";", userID, userID)
			stus = query(sql)
		} else if account_type == 3 {
			sql = fmt.Sprintf("SELECT orgID from organization WHERE name=\"%s\";", userID)
			orgID := query(sql)[0]["orgID"].(int64)
			sql = fmt.Sprintf("SELECT orgID,name from organization WHERE higher_org=%d;", orgID)
			branches := query(sql)
			for _, branch := range branches {
				sql = fmt.Sprintf("SELECT userID AS name FROM user WHERE belonging_org=%d AND userID!=\"%s\";", branch["orgID"].(int64), branch["name"])
				temp := query(sql)
				for _, t := range temp {
					t["belonging_org"] = branch["name"]
					stus = append(stus, t)
				}
			}
		} else if account_type == 1 || account_type == 0 {
			sql = "SELECT user.userID AS name,organization.name AS belonging_org FROM user,organization WHERE user.account_type=5 AND user.belonging_org=organization.orgID AND user.userID!=organization.name ;"
			stus = query(sql)
		}

		// 检索所有需要审核的申请

		appliances := []map[string]any{}
		to_audit := to_audit_map[account_type]
		for _, stu := range stus {
			sql = fmt.Sprintf("SELECT ap.applianceID AS applianceID,ap.userID AS userID,item.name AS item,item.type AS type,ap.score AS score,ap.description AS description,ap.status AS status FROM appliance AS ap,item WHERE ap.itemID=item.itemID AND ap.status=%d AND ap.userID=\"%s\";", to_audit, stu["name"])
			temp := query(sql)
			appliances = append(appliances, temp...)
		}

		aps := []map[string]any{}
		for _, ap := range appliances {
			ap["type"] = item_types[ap["type"].(int64)]
			ap["status"] = appliance_status[ap["status"].(int64)]
			aps = append(aps, ap)
		}
		c.HTML(http.StatusOK, "audit_basic.html", gin.H{
			"msg":          "",
			"to_audit_sum": len(aps),
			"appliances":   aps,
		})

	})

	r.GET("/audit_detail", Midware_Auth, Authorities(0b011011), func(c *gin.Context) {
		// 检验是否有审核权限(是否属于同一级审核、是否处于对应组织管理下)
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		admin_org := query(sql)[0]["belonging_org"].(int64)
		account_type := c.GetInt64("account_type")
		applianceID := c.Query("applianceID")
		sql = fmt.Sprintf("SELECT * FROM appliance WHERE applianceID=%s;", applianceID)
		appliance := query(sql)
		if len(appliance) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"申请不存在！\"}")
			return
		}
		can_audit_status, ok := to_audit_map[account_type]
		if !ok || can_audit_status != appliance[0]["status"].(int64) {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
			return
		}
		to_audit_user := appliance[0]["userID"].(string)
		sql = fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", to_audit_user)
		user_info := query(sql)[0]
		branchID := user_info["belonging_org"].(int64)
		if account_type == 4 && admin_org != branchID {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
			return
		}
		if account_type == 3 {
			sql = fmt.Sprintf("SELECT * FROM organization WHERE orgID=%d;", branchID)
			query_res := query(sql)[0]
			collegeID := query_res["higher_org"].(int64)
			if admin_org != collegeID {
				c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
				return
			}
		}
		sql = fmt.Sprintf("SELECT ap.applianceID AS applianceID,ap.userID AS userID, item.name AS item, item.type AS type, ap.score AS score, ap.description AS description, ap.status AS status FROM appliance as ap,item WHERE ap.itemID=item.itemID AND ap.applianceID=%s;", applianceID)
		ap := query(sql)[0]
		ap["status"] = appliance_status[ap["status"].(int64)]
		ap["type"] = item_types[ap["type"].(int64)]
		c.HTML(http.StatusOK, "audit_detail.html", gin.H{
			"appliance":    ap,
			"account_type": account_type,
		})
	})

	r.POST("/audit_basic_item", Midware_Auth, Authorities(0b011011), func(c *gin.Context) {
		// 检验是否有审核权限(是否属于同一级审核、是否处于对应组织管理下)
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		admin_org := query(sql)[0]["belonging_org"].(int64)
		account_type := c.GetInt64("account_type")
		applianceID := c.Query("applianceID")
		sql = fmt.Sprintf("SELECT * FROM appliance WHERE applianceID=%s;", applianceID)
		appliance := query(sql)
		operation := ""
		var score float64 = -1
		if len(appliance) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"申请不存在！\"}")
			return
		}
		can_audit_status, ok := to_audit_map[account_type]
		if !ok || can_audit_status != appliance[0]["status"].(int64) {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
			return
		}
		to_audit_user := appliance[0]["userID"].(string)
		sql = fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", to_audit_user)
		user_info := query(sql)[0]
		branchID := user_info["belonging_org"].(int64)
		if account_type == 4 {
			if admin_org != branchID {
				c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
				return
			}
			operation += "团支部"
		}
		if account_type == 3 {
			sql = fmt.Sprintf("SELECT * FROM organization WHERE orgID=%d;", branchID)
			query_res := query(sql)[0]
			collegeID := query_res["higher_org"].(int64)
			if admin_org != collegeID {
				c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
				return
			}
			operation += "学院"
			score, _ = strconv.ParseFloat(c.PostForm("score"), 64)
		}
		if account_type == 0 || account_type == 1 {
			operation += "学校"
		}
		sql = fmt.Sprintf("SELECT ap.applianceID AS applianceID,ap.userID AS userID, item.name AS item, item.type AS type, ap.score AS score, ap.description AS description, ap.status AS status,ap.record AS record FROM appliance as ap,item WHERE ap.itemID=item.itemID AND ap.applianceID=%s;", applianceID)
		ap := query(sql)[0]

		record_str := ap["record"].(string)
		record := []map[string]any{}
		audit_status := c.PostForm("option")
		status := -1
		if audit_status == "1" {
			operation += "审核通过："
			if account_type == 0 || account_type == 1 {
				status = 5
			} else if account_type == 3 {
				status = 3
			} else if account_type == 4 {
				status = 1
			}
		} else {
			operation += "审核不通过："
			if account_type == 0 || account_type == 1 {
				status = 6
			} else if account_type == 3 {
				status = 4
			} else if account_type == 4 {
				status = 2
			}
		}
		audit_opinion := c.PostForm("opinion")
		operation += audit_opinion
		json.Unmarshal([]byte(record_str), &record)
		record = append(record, map[string]any{
			"operator":  userID,
			"time":      strconv.Itoa(int(time.Now().Unix())),
			"operation": operation,
		})
		json, _ := json.Marshal(record)
		record_str = string(json)
		if account_type == 3 {
			sql = fmt.Sprintf("UPDATE appliance SET status=%d,record='%s',score=%.1f WHERE applianceID=%s;", status, record_str, score, applianceID)
		} else {
			sql = fmt.Sprintf("UPDATE appliance SET status=%d,record='%s' WHERE applianceID=%s;", status, record_str, applianceID)
		}

		exec(sql)
		c.Redirect(http.StatusTemporaryRedirect, "audit_basic.html")
	})

	r.GET("/add_item.html", Midware_Auth, Authorities(0b001100), func(c *gin.Context) {
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		orgID := query(sql)[0]["belonging_org"].(int64)
		sql = fmt.Sprintf("SELECT * FROM item WHERE create_org=%d", orgID)
		items := query(sql)
		for _, item := range items {
			item["status"] = item_status[item["status"].(int64)]
			item["type"] = item_types[item["type"].(int64)]
		}
		c.HTML(http.StatusOK, "add_item.html", gin.H{
			"added": items,
		})
	})

	r.POST("/add_activity_item", Midware_Auth, Authorities(0b001100), func(c *gin.Context) {
		userID := c.GetString("userID")
		var msg string
		name := c.PostForm("name")
		sql := fmt.Sprintf("SELECT * FROM user WHERE userID=\"%s\";", userID)
		orgID := query(sql)[0]["belonging_org"].(int64)
		sql = fmt.Sprintf("SELECT * FROM item WHERE name=\"%s\";", name)
		if len(query(sql)) == 0 {
			tp := c.PostForm("type")
			score_lower_range, _ := strconv.ParseFloat(c.PostForm("score_lower_range"), 64)
			score_higher_range, _ := strconv.ParseFloat(c.PostForm("score_higher_range"), 64)
			description := c.PostForm("description")
			time := int(time.Now().Unix())
			record := []map[string]any{}
			temp := map[string]any{
				"operator":  userID,
				"time":      strconv.Itoa(time),
				"operation": "添加项目：" + name,
			}
			record = append(record, temp)
			json, _ := json.Marshal(record)
			sql = fmt.Sprintf("INSERT INTO item VALUES(NULL,%s,1,\"%s\", %.2f, %.2f, %d,\"%s\",%d,'%s');", tp, name, score_lower_range, score_higher_range, orgID, description, time, string(json))
			ok := exec(sql)
			fmt.Println(sql)
			if ok {
				form, _ := c.MultipartForm()
				files := form.File
				path := fmt.Sprintf("upload/activity/%d/%d/", orgID, time)
				_, err := os.Stat(path)
				if os.IsNotExist(err) {
					os.MkdirAll(path, os.ModePerm)
				}
				for _, file := range files {
					f, _ := file[0].Open()
					defer f.Close()
					c.SaveUploadedFile(file[0], path+file[0].Filename)
				}
				msg = "添加成功！"
			} else {
				msg = "添加失败。"
			}
		} else {
			msg = "添加失败：项目名称重复。"
		}

		sql = fmt.Sprintf("SELECT * FROM item WHERE create_org=%d", orgID)
		items := query(sql)
		for _, item := range items {
			item["status"] = item_status[item["status"].(int64)]
			item["type"] = item_types[item["type"].(int64)]
		}
		c.HTML(http.StatusOK, "add_item.html", gin.H{
			"msg":   msg,
			"added": items,
		})

	})

	r.GET("/added_item_detail", Midware_Auth, Authorities(0b001100), func(c *gin.Context) {
		itemID := c.Query("itemID")
		userID := c.GetString("userID")
		sql := fmt.Sprintf("SELECT belonging_org FROM user WHERE userID=\"%s\";", userID)
		orgID := query(sql)[0]["belonging_org"].(int64)
		sql = fmt.Sprintf("SELECT * FROM item WHERE itemID=%s", itemID)
		item := query(sql)
		if len(item) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"项目不存在！\"}")
			return
		}
		create_org := item[0]["create_org"].(int64)
		if orgID != create_org {
			c.AbortWithStatusJSON(http.StatusNotFound, "{\"error\":\"权限不足！\"}")
			return
		}
		sql = fmt.Sprintf("SELECT name FROM organization WHERE orgID=%d", create_org)
		item[0]["create_org"] = query(sql)[0]["name"].(string)
		item[0]["status"] = item_status[item[0]["status"].(int64)]

		time := item[0]["time_unix"].(int64)
		path := "upload/activity/" + strconv.Itoa(int(create_org)) + "/" + strconv.Itoa(int(time)) + "/"
		dir, _ := os.ReadDir(path)
		paths := []string{}
		for _, file := range dir {
			if !file.IsDir() {
				paths = append(paths, path+file.Name())
			}
		}

		record_str := item[0]["record"].(string)
		records := []map[string]any{}
		json.Unmarshal([]byte(record_str), &records)

		c.HTML(http.StatusOK, "added_item_detail.html", gin.H{
			"item":    item[0],
			"paths":   paths,
			"records": records,
		})

	})

	//todo : 立项审核

	r.Run(":4203") // Listening at http://localhost:4203
}
