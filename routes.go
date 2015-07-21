package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"log"
	"net/http"
	"net/http/httputil"
)

const (
	loginPath = "/login"
)

var (
	store sessions.Store
)

func setupHandlers() http.Handler {
	store = sessions.NewCookieStore(config.Keys.AuthenticationKey, config.Keys.EncryptionKey)
	muxer := mux.NewRouter()

	muxer.Handle("/", LoggingMiddleware{http.HandlerFunc(mainHandler)})
	muxer.Handle("/{x:.*}", LoggingMiddleware{http.HandlerFunc(mainHandler)}) //Both are necessary.

	proxymux := muxer.PathPrefix("/proxy").Subrouter()
	proxymux.Handle("/", LoggingMiddleware{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("AuthProx - Menu"))
	})})

	{ // GET handlers
		m := proxymux.Methods("GET").Subrouter()
		m.PathPrefix("/login").Handler(LoggingMiddleware{http.HandlerFunc(getLogin)})
		m.PathPrefix("/register").Handler(LoggingMiddleware{http.HandlerFunc(getRegister)})
		m.PathPrefix("/logout").Handler(LoggingMiddleware{http.HandlerFunc(getLogout)})
	}

	{ // POST handlers
		m := proxymux.Methods("POST").Subrouter()
		m.PathPrefix("/login").Handler(LoggingMiddleware{http.HandlerFunc(postLogin)})
		m.PathPrefix("/register").Handler(LoggingMiddleware{http.HandlerFunc(postRegister)})
	}

	return muxer
}

type LoggingMiddleware struct {
	Wrapped http.Handler
}

func (lm LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"method": r.Method,
		"url":    r.URL,
		"client": r.RemoteAddr,
	}).Info("HTTP Request")
	lm.Wrapped.ServeHTTP(w, r)
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
		logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"url":      r.URL,
			"client":   r.RemoteAddr,
			"redirect": "/",
			"status":   http.StatusTemporaryRedirect,
		}).Info("Client already logged in.")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	w.Write([]byte(loginPage)) //Not logged in. Serve login page
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
		w.Write([]byte(loginSuccessPage))
	} else {
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
			"user":   r.PostFormValue("username"),
		}).Info("Client failed to logged in.")
		w.Write([]byte(loginPage))
	}
}

func getRegister(w http.ResponseWriter, r *http.Request) {
	//If they are logged in and want to register again, then fine.
	//Can add measures against this if it becomes and issue.
	w.Write([]byte(registrationPage)) //Serve register page
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
		w.Write([]byte(registrationSuccessPage))
	case ErrUserExists:
		http.Error(w, "The user already exists. Please try again with a different username.", http.StatusPreconditionFailed)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
