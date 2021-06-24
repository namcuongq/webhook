package model

import (
	"net/http"
)

type Req struct {
	Id       string
	Date     string
	Header   http.Header
	Body     string
	Url      string
	ClientIp string
	Method   string
}
