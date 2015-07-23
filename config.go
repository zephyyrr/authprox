package main

import (
	"encoding/base64"
	"errors"
	"github.com/BurntSushi/toml"
	"github.com/Sirupsen/logrus"
	"os"
)

type Config struct {
	Address         string
	Destination     string
	Logfile         string
	StaticResources *string
	RootRedirect    *string

	Keys struct {
		AuthenticationKey Key
		EncryptionKey     Key
		ReCaptcha         string
	}

	Database struct {
		Type     string
		Location string
	}
}

func loadConfig() {
	_, err := toml.DecodeFile(*cfile, &config)
	if err != nil && !*setup {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("Unable to read config file.")
	}

	logger.WithFields(logrus.Fields{
		"bind-address": config.Address,
		"destination":  config.Destination,
		"logfile":      config.Logfile,
		"db-type":      config.Database.Type,
		"db-location":  config.Database.Location,
	}).Info("Loaded config")
}

func saveConfig() {
	f, err := os.OpenFile(*cfile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		logger.Println("Unable to open config file to write.", err)
	}
	enc := toml.NewEncoder(f)
	err = enc.Encode(&config)
	if err != nil {
		logger.Println("Unable to write config to file.", err)
	}
	f.Close()
}

type Key []byte

var (
	ErrInvalidKey = errors.New("Invalid Key length. Expected 32 or 64 bytes.")
)

func (k Key) MarshalText() (res []byte, err error) {
	res = make([]byte, base64.StdEncoding.EncodedLen(len(k)))
	base64.StdEncoding.Encode(res, k)
	return
}

func (k *Key) UnmarshalText(text []byte) error {
	*k = make(Key, base64.StdEncoding.DecodedLen(len(text)))
	n, err := base64.StdEncoding.Decode([]byte(*k), text)
	if err != nil {
		return err //Error decoding
	}
	*k = (*k)[:n]

	if !(len(*k) == 32 || len(*k) == 64) {
		return ErrInvalidKey
	}
	return nil
}

var (
	config = Config{ //Default values
		Address:     "0.0.0.0:80",
		Destination: "localhost:8080",
		Logfile:     defaultlogfile,
		Database: struct {
			Type     string
			Location string
		}{
			Type:     "bolt",
			Location: defaultDBfile,
		},
	}
)
