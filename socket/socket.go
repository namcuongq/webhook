package socket

import (
	"fmt"
	"strings"
	"sync"
	"webhook/constant"

	"github.com/olahol/melody"
)

type SessionMgr struct {
	mux      sync.RWMutex
	sessions map[interface{}]*melody.Session
}

type SessionDetail struct {
	Id   string
	Ip   string
	Date string
}

func NewMgr() *SessionMgr {
	return &SessionMgr{
		sessions: map[interface{}]*melody.Session{},
	}
}

func (mgr *SessionMgr) Get(id interface{}) (SessionDetail, bool) {
	mgr.mux.RLock()
	defer mgr.mux.RUnlock()
	sess, ok := mgr.sessions[id]
	if !ok {
		return SessionDetail{}, ok
	}
	return SessionDetail{
		Id:   fmt.Sprintf("%v", id),
		Ip:   sess.Request.Header.Get(constant.HEADER_VICTIM_IP),
		Date: sess.Request.Header.Get(constant.HEADER_VICTIM_DATE),
	}, ok
}

func (mgr *SessionMgr) GetAll() []SessionDetail {
	mgr.mux.RLock()
	defer mgr.mux.RUnlock()
	results := []SessionDetail{}
	for id, sess := range mgr.sessions {

		results = append(results, SessionDetail{
			Id:   fmt.Sprintf("%v", id),
			Ip:   sess.Request.Header.Get(constant.HEADER_VICTIM_IP),
			Date: sess.Request.Header.Get(constant.HEADER_VICTIM_DATE),
		})
	}
	return results
}

func (mgr *SessionMgr) Set(id interface{}, sess *melody.Session) {
	mgr.mux.Lock()
	defer mgr.mux.Unlock()
	mgr.sessions[id] = sess
}

func (mgr *SessionMgr) Delete(id interface{}) {
	mgr.mux.Lock()
	defer mgr.mux.Unlock()
	sess, found := mgr.sessions[id]
	if !found {
		return
	}
	sess.Close()
	delete(mgr.sessions, id)
}

func (mgr *SessionMgr) Exist(id interface{}) bool {
	mgr.mux.RLock()
	defer mgr.mux.RUnlock()
	_, found := mgr.sessions[id]
	return found
}

func (mgr *SessionMgr) ForEach(filter func(sess *melody.Session) bool, handler func(sess *melody.Session)) {
	mgr.mux.RLock()
	defer mgr.mux.RUnlock()
	for _, sess := range mgr.sessions {
		if filter(sess) {
			handler(sess)
		}
	}
}

var (
	sessionMgr = SessionMgr{
		sessions: map[interface{}]*melody.Session{},
	}
)

func GetIDFromSession(sess *melody.Session) string {
	str := sess.Request.URL.Path
	str = strings.Replace(str, "/ws", "", 1)
	i := strings.LastIndex(str, "/")
	if i <= 0 {
		return ""
	}

	return str[i+1:]
}
