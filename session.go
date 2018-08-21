package gosession

import (
	"strings"
	"errors"
	"net/http"
	"net/url"
	"time"
	"log"
	"crypto/sha256"
	"encoding/hex"
	"crypto/rand"
	mathRand "math/rand"
)

var(
	ErrSessionValueNotExist = errors.New("value does not exist")
)

var provides = make(map[string]SessionProvider)

type SessionProvider interface {
	SessionInit(maxLifetime int64, savePath string) error
	Open(sessionId string) (SessionHandler, error)
	Destroy(sessionId string) error
	GC()
}

type SessionHandler interface {
	Add(key string, value interface{}) error //添加一个Session值，如果已存在则覆盖
	Get(key string) (interface{}, error)     //获取一个Session值
	Remove(key string) error                 //移除一个Session
	SessionId() string                       //获取当前SessionId
	Clear()                                  //清空所有值
}

func NewSessionId() string {
	b := make([]byte, 256)

	//如果使用真随机生成失败，则转换为伪随机
	if _, err := rand.Read(b); err != nil {
		bytes := []byte("0123456789abcdefghijklmnopqrstuvwxyz`!@#$%^&*()_+=:'/?.>,<")

		r := mathRand.New(mathRand.NewSource(time.Now().UnixNano()))
		for i := 0 ;i < 256 ;i++ {
			b[i] = bytes[r.Intn(len(bytes))]
		}
	}

	hash := sha256.New()
	hash.Write(b)
	md := hash.Sum(nil)
	return hex.EncodeToString(md)
}

func Register(providerName string, provider SessionProvider) {
	if provider == nil {
		panic("session provider cannot be nil")
	}
	if _, dup := provides[providerName]; dup {
		panic("session provider already exists: " + providerName)
	}
	provides[providerName] = provider
}

type SessionConfig struct {
	CookieName      string `json:"cookieName"`
	EnableSetCookie bool   `json:"enableSetCookie,omitempty"`
	Gclifetime      int64  `json:"gclifetime"`
	Maxlifetime     int64  `json:"maxLifetime"`
	Secure          bool   `json:"secure"`
	CookieLifeTime  int    `json:"cookieLifeTime"`
	ProviderConfig  string `json:"providerConfig"`
	Domain          string `json:"domain"`
}

type SessionManager struct {
	provider SessionProvider
	config   *SessionConfig
}

func NewSessionManager(providerName string, config SessionConfig) (*SessionManager, error) {
	provider, ok := provides[providerName]
	if ok == false {
		return nil, errors.New("Session 提供程序未找到")
	}
	if (config.Maxlifetime == 0) {
		config.Maxlifetime = config.Gclifetime
	}

	err := provider.SessionInit(config.Maxlifetime, config.ProviderConfig)
	if (err != nil) {
		return nil, err
	}
	return &SessionManager{
		provider: provider,
		config:   &config,
	}, nil
}

//获取当前请求的SessionId
func (manager *SessionManager) getSessionId(r *http.Request) (string, error) {
	cookie, err := r.Cookie(manager.config.CookieName)
	if err == nil && cookie.Value != "" && cookie.MaxAge >= 0 {

		return cookie.Value, nil
	} else if strings.EqualFold(r.Method, "POST") {
		err := r.ParseForm()
		if (err != nil) {
			return "", err
		}
		sid := r.FormValue(manager.config.CookieName)
		return sid, nil
	} else if (cookie != nil) {
		//支持URL传递SessionId
		return url.QueryUnescape(cookie.Value)
	}
	return "", nil
}

func (manager *SessionManager) SessionStart(w http.ResponseWriter, r *http.Request) (SessionHandler, error) {

	sessionId, err := manager.getSessionId(r)
	log.Print("SessionStart", sessionId)

	if (err != nil) {
		return nil, err
	}
	if (sessionId == "") {
		sessionId = NewSessionId()
		cookie := &http.Cookie{
			Name:     manager.config.CookieName,
			Value:    url.QueryEscape(sessionId),
			Path:     "/",
			HttpOnly: true,
			Secure:   manager.isSecure(r),
			Domain:   manager.config.Domain,
		}
		if manager.config.CookieLifeTime > 0 {
			cookie.MaxAge = manager.config.CookieLifeTime
			cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
		}
		if manager.config.EnableSetCookie {
			http.SetCookie(w, cookie)
		}
		r.AddCookie(cookie)
	}

	return manager.provider.Open(sessionId)
}

func (manager *SessionManager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	manager.provider.Destroy(sid)
	if manager.config.EnableSetCookie {
		expiration := time.Now()
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Path: "/",
			HttpOnly: true,
			Expires: expiration,
			MaxAge: -1}

		http.SetCookie(w, cookie)
	}
}

func (manager *SessionManager) GetSessionHandler(sessionId string) (SessionHandler, error) {
	return manager.provider.Open(sessionId)
}

func (manager *SessionManager) GC() {
	manager.provider.GC()
	time.AfterFunc(time.Duration(manager.config.Gclifetime)*time.Second, func() {
		manager.GC()
	})
}

// Set cookie with https.
func (manager *SessionManager) isSecure(req *http.Request) bool {
	if !manager.config.Secure {
		return false
	}
	if req.URL.Scheme != "" {
		return strings.EqualFold(req.URL.Scheme, "https")
	}
	if req.TLS == nil {
		return false
	}
	return true
}
