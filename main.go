package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"
	"github.com/haisum/recaptcha"
	"github.com/rifflock/lfshook"
	"net/http"
	"os"
	"strings"
)

var (
	cfile      = flag.String("f", defaultcfile, "Config file to use")
	setup      = flag.Bool("setup", false, "creates a config-file with key")
	logger     *logrus.Logger
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

	selectUserManager()
	selectPageSource()

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

func selectUserManager() {
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
			Admin:    true,
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
}
