package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"log"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	loginPath = "/login"
)

var (
	pages    Pages
	renderer Renderer
	store    sessions.Store
)

func init() {
	pages = constPages
	renderer = defaultRenderer
}

func setupHandlers() http.Handler {
	store = sessions.NewCookieStore(config.Keys.AuthenticationKey, config.Keys.EncryptionKey)
	muxer := mux.NewRouter()

	muxer.Handle("/", LoggingMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.RootRedirect != nil {
			http.Redirect(w, r, *config.RootRedirect, http.StatusMovedPermanently)
		}
		mainHandler(w, r)
	})))
	muxer.MatcherFunc(wildcard).Handler(LoggingMiddleware{
		Wrapped: http.HandlerFunc(mainHandler),
		Message: "HTTP Proxied",
	}) //Both are necessary.

	proxymux := muxer.PathPrefix("/proxy").Subrouter()
	proxymux.Handle("/", LoggingMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderer.Render(w, pages.Get(MainMenuPage))
	})))

	{ // GET handlers
		m := proxymux.Methods("GET").Subrouter()
		m.PathPrefix("/login").Handler(LoggingMW(http.HandlerFunc(getLogin)))
		m.PathPrefix("/register").Handler(LoggingMW(http.HandlerFunc(getRegister)))
		m.PathPrefix("/logout").Handler(LoggingMW(http.HandlerFunc(getLogout)))
		if config.WebDirectory != nil { //Only put up this route if we have static content. Could all be served from CDN or similar.
			staticdir := filepath.Join(*config.WebDirectory, "static")
			m.PathPrefix("/static").Handler(LoggingMW(CacheMW{RewriteMW{
				Wrapped: http.FileServer(http.Dir(staticdir)),
				From:    "/proxy/static/(.*)",
				To:      "/$1",
			}}))
			logger.WithField("dir", staticdir).Info("Static route setup.")
		}
	}

	{ // POST handlers
		m := proxymux.Methods("POST").Subrouter()
		m.PathPrefix("/login").Handler(LoggingMW(http.HandlerFunc(postLogin)))
		m.PathPrefix("/register").Handler(LoggingMW(http.HandlerFunc(postRegister)))
	}

	return muxer
}

func wildcard(r *http.Request, rm *mux.RouteMatch) bool {
	return !strings.HasPrefix(r.URL.Path, "/proxy/")
}

type CacheMW struct{ Wrapped http.Handler }

func (c CacheMW) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("cache-control", "public,max-age:360000")
	c.Wrapped.ServeHTTP(w, r)
}

func LoggingMW(h http.Handler) http.Handler {
	return LoggingMiddleware{
		Message: "HTTP Request",
		Wrapped: h,
	}
}

type LoggingMiddleware struct {
	Message string
	Wrapped http.Handler
}

func (lm LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"method": r.Method,
		"url":    r.URL,
		"client": r.RemoteAddr,
	}).Info(lm.Message)
	lm.Wrapped.ServeHTTP(w, r)
}

type RewriteMW struct {
	Wrapped http.Handler
	From    string
	To      string
	exp     *regexp.Regexp
}

func (rw RewriteMW) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rw.exp == nil {
		var err error
		rw.exp, err = regexp.Compile(rw.From)
		if err != nil {
			logger.WithField("from", rw.From).Fatal("Error compiling Rewrite rule.")
		}
	}
	r.URL.Path = string(rw.exp.ReplaceAll([]byte(r.URL.Path), []byte(rw.To)))
	rw.Wrapped.ServeHTTP(w, r)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth")
	if loggedin, ok := session.Values["loggedin"].(bool); !(ok && loggedin) {
		if isWebsocket(r) {
			http.Error(w, "You need to login first.", http.StatusUnauthorized)
		}
		logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"url":      r.URL,
			"client":   r.RemoteAddr,
			"redirect": "/proxy/login",
			"status":   http.StatusTemporaryRedirect,
		}).Info("Client not logged in.")
		http.Redirect(w, r, "/proxy/login", http.StatusTemporaryRedirect)
	}

	if isWebsocket(r) {
		p := websocketProxy{}
		p.ServeHTTP(w, r)
		return
	}

	wlogger := logger.Writer()
	defer wlogger.Close()

	revProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = config.Destination
			logger.WithField("path", r.URL.Path).Debug("Directing reverse-proxy")
		},
		ErrorLog: log.New(wlogger, "", 0),
	}
	revProxy.ServeHTTP(w, r)
}

func getLogin(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth")
	if loggedin, ok := session.Values["loggedin"].(bool); ok && loggedin {
		//Already logged in, so redirect to mainpage.
		renderer.Render(w, Page{
			Title:   "Already Logged in",
			Content: "You are already logged in!",
		})
		return
	}
	renderer.Render(w, pages.Get(LoginPage)) //Not logged in. Serve login page
}

func postLogin(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth")
	//Temp code. Autologin.
	r.ParseForm()

	if users.Authenticate(r.PostFormValue("username"), r.PostFormValue("password")) {
		session.Values["loggedin"] = true
		session.Save(r, w)
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
			"user":   r.PostFormValue("username"),
		}).Info("Client logged in.")
		renderer.Render(w, pages.Get(LoginSuccessPage))
	} else {
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
			"user":   r.PostFormValue("username"),
		}).Info("Client failed to logged in.")
		renderer.Render(w, pages.Get(LoginPage))
	}
}

func getRegister(w http.ResponseWriter, r *http.Request) {
	//If they are logged in and want to register again, then fine.
	//Can add measures against this if it becomes and issue.
	renderer.Render(w, pages.Get(RegistrationPage)) //Serve register page
}

func postRegister(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username, password := r.PostFormValue("username"), r.PostFormValue("password")
	if !recaptcher.Verify(*r) {
		logger.WithFields(logrus.Fields{
			"user":  username,
			"error": recaptcher.LastError(),
		}).Error("Failed to verify reCaptcha during registration.")
		w.Write([]byte("Failed to verify the reCaptcha. Please verify that you are human and try again."))
		return
	}

	err := users.Register(username, password)
	switch err {
	case nil:
		//Success
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
			"user":   username,
		}).Info("User registration")
		renderer.Render(w, pages.Get(RegistrationSuccessPage))
	case ErrUserExists:
		http.Error(w, "The user already exists. Please try again with a different username.", http.StatusPreconditionFailed)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth")
	session.Values["loggedin"] = false
	session.Save(r, w)
	renderer.Render(w, pages.Get(LogoutPage))
}
