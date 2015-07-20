package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/haisum/recaptcha"
	"github.com/rifflock/lfshook"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/gorilla/context"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

var (
	cfile      = flag.String("f", defaultcfile, "Config file to use")
	setup      = flag.Bool("setup", false, "creates a config-file with key")
	logger     *logrus.Logger
	store      sessions.Store
	users      UserManager
	recaptcher recaptcha.R
)

func init() {
	logger = logrus.New()
	logger.Out = os.Stderr
	logger.Formatter = &logrus.TextFormatter{}
	logger.Level = logrus.InfoLevel
}

func main() {
	flag.Parse()
	loadConfig()
	setupLogger()
	if *setup {
		config.Keys.AuthenticationKey = securecookie.GenerateRandomKey(64)
		config.Keys.EncryptionKey = securecookie.GenerateRandomKey(32)
		saveConfig()
		return
	}

	switch config.Database.Type {
	case "dummy":
		usr_data := strings.Split(config.Database.Location, " ")
		if len(usr_data) < 2 {
			logger.WithFields(logrus.Fields{
				"expected": "<username> <password>",
				"found":    config.Database.Location,
			}).Panic("Invalid dummy database configuration.")
		}

		users = &DummyUserManager{
			Name:     usr_data[0],
			Passhash: HashAndSalt(usr_data[1], Key{1}),
			Salt:     Key{1},
		}
	case "bolt":
		var err error
		users, err = NewBoltUserManager(config.Database.Location)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"type":     config.Database.Type,
				"location": config.Database.Location,
			}).Fatal("Unable to create database connection.")
		}
	}

	recaptcher = recaptcha.R{
		Secret: config.Keys.ReCaptcha,
	}

	handler := setupHandlers()
	http.ListenAndServe(config.Address, handler)
}

func setupLogger() {
	logger.Hooks.Add(lfshook.NewHook(lfshook.PathMap{
		logrus.ErrorLevel: config.Logfile,
	}))
}

const (
	loginPath = "/proxy/login"
)

func setupHandlers() http.Handler {
	store = sessions.NewCookieStore(config.Keys.AuthenticationKey, config.Keys.EncryptionKey)
	muxer := http.NewServeMux()
	w := logger.Writer()
	//FIXME Should somehow close w.
	revProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = config.Destination
		},
		ErrorLog: log.New(w, "", 0),
	}

	muxer.Handle("/", context.ClearHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
		}).Info()
		session, _ := store.Get(r, "auth")

		if loggedin, ok := session.Values["loggedin"].(bool); !(ok && loggedin) {
			if isWebsocket(r) {
				http.Error(w, "You need to login first.", http.StatusUnauthorized)
			}
			logger.WithFields(logrus.Fields{
				"method":   r.Method,
				"url":      r.URL,
				"client":   r.RemoteAddr,
				"redirect": loginPath,
				"status":   http.StatusTemporaryRedirect,
			}).Info("Client not logged in.")
			http.Redirect(w, r, loginPath, http.StatusTemporaryRedirect)
		}

		if isWebsocket(r) {
			p := websocketProxy{}
			p.ServeHTTP(w, r)
			return
		}
		revProxy.ServeHTTP(w, r)
	})))

	muxer.Handle(loginPath, context.ClearHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
		}).Info("Request to login handler")
		switch r.Method {
		case "GET":
			getLogin(w, r)
		case "POST":
			postLogin(w, r)
		}
	})))

	muxer.Handle("/proxy/register", context.ClearHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL,
			"client": r.RemoteAddr,
		}).Info("Request to register handler")

		switch r.Method {
		case "GET":
			getRegister(w, r)
		case "POST":
			postRegister(w, r)
		}
	})))

	return muxer
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
