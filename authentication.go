package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/gob"
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/gorilla/securecookie"
	"io"
)

type Authenticator interface {
	Authenticate(user, password string) bool
}

type User struct {
	Name     string
	Admin    bool
	Passhash Key
	Salt     Key
}

type UserManager interface {
	Authenticator
	Register(username, password string) error
	Update(user User) error
	Get(username string) (User, error)
}

var (
	ErrUnknownUser   = errors.New("User does not exist")
	ErrUserExists    = errors.New("User already exists")
	ErrWrongPassword = errors.New("Wrong password")
)

//A Dummy User Manager implements the User Manager interface in the simplest way possible.
//It stores a single user in memory and authenticates is compared to an equal.
//The Register operation replaces the stored user.
//It is considered useful for testing purposes and is not expected to remain after release.
type DummyUserManager User

func (d DummyUserManager) Authenticate(user, password string) bool {
	if user == d.Name && sameBytes(HashAndSalt(password, d.Salt), d.Passhash) {
		return true
	}
	return false
}

func (d DummyUserManager) Get(user string) (User, error) {
	if d.Name == user {
		return User(d), nil
	}
	return User{}, ErrUnknownUser
}

func (d *DummyUserManager) Register(user, password string) error {
	d.Name, d.Passhash = user, HashAndSalt(password, d.Salt)
	return nil
}

func (d *DummyUserManager) Update(user User) error {
	if d.Name == user.Name {
		d.Passhash = user.Passhash
	}
	return ErrUnknownUser
}

//A JsonUserManager is a User Manager backed by a JSON datastore on file.
//The contents of the file is kept in memory during operation, but all changes are saved back to file immediately.
type JsonUserManager struct {
	cache map[string]User
	file  io.WriteSeeker
}

func (um JsonUserManager) Authenticate(username, password string) (authenticated bool) {
	user, ok := um.cache[username]
	if !ok {
		return false
	}
	hashsum := HashAndSalt(password, user.Salt)
	if sameBytes(user.Passhash, hashsum) { //Compare to stored hash
		authenticated = true
	}
	return
}

//HashAndSalt takes a user-supplied password and a server generated salt and creates the corresponding hash.
// The hash-algorithm used can be changed and is left as an implementation detail.
func HashAndSalt(password string, salt []byte) []byte {
	hasher := sha512.New()
	hasher.Write([]byte(password))
	hasher.Write(salt)                              // Add a dash of salt for good flavour (and practice)
	return hasher.Sum(make([]byte, 0, sha512.Size)) //Calculate sum
}

// sameBytes performs a deep comparison of two slices of bytes.
// It attempts to take the same time to execute regardless of result.
// It returns true if the lengths and contents of the slices are equal.
// False otherwise.
func sameBytes(a, b []byte) (res bool) {
	if len(a) != len(b) {
		return false
	}
	res = true
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			res = false
		}
	}
	return
}

func init() {
	gob.Register(User{})
}

type BoltUserManager struct {
	*bolt.DB
}

var (
	BoltBucketUsers = []byte("users")
)

func NewBoltUserManager(file string) (bum *BoltUserManager, err error) {
	db, err := bolt.Open(file, 0600, nil)
	if err != nil {
		logger.WithFields(logrus.Fields{"err": err, "file": file}).Error("Opening Bolt DB")
		return
	}
	err = db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists(BoltBucketUsers)
		return
	})
	if err != nil {
		logger.WithFields(logrus.Fields{"err": err, "file": file}).Error("Creating top-level buckets")
		return
	}
	logger.WithFields(logrus.Fields{"file": file}).Debug("Bolt-user manager initialized")

	bum = &BoltUserManager{DB: db}
	return
}

func (bum BoltUserManager) Authenticate(username, password string) bool {
	err := bum.View(func(tx *bolt.Tx) error {
		user, err := bum.fetchUser(tx, username)
		if err != nil {
			return err
		}
		hash := HashAndSalt(password, user.Salt)
		if !sameBytes(hash, user.Passhash) {
			return ErrWrongPassword
		}
		return nil
	})
	return err == nil
}

func (bum BoltUserManager) Register(user, password string) (err error) {
	err = bum.DB.Update(func(tx *bolt.Tx) error {
		if _, err := bum.fetchUser(tx, user); err == nil {
			//User already exists
			return ErrUserExists
		}
		b := tx.Bucket(BoltBucketUsers)
		salt := securecookie.GenerateRandomKey(32)
		err := b.Put([]byte(user), bum.encodeUser(User{
			Name:     user,
			Passhash: HashAndSalt(password, salt),
			Salt:     salt,
		}))
		return err
	})
	return
}

func (bum BoltUserManager) Update(user User) (err error) {
	err = bum.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(BoltBucketUsers)
		b.Put([]byte(user.Name), bum.encodeUser(user))
		return nil
	})
	return
}

func (bum BoltUserManager) Get(user string) (u User, err error) {
	err = bum.View(func(tx *bolt.Tx) error {
		u, err = bum.fetchUser(tx, user)
		return err
	})
	return
}

func (bum BoltUserManager) fetchUser(tx *bolt.Tx, username string) (u User, err error) {
	b := tx.Bucket(BoltBucketUsers)
	data := b.Get([]byte(username))
	if data == nil {
		err = ErrUnknownUser
		return
	}
	dec := gob.NewDecoder(bytes.NewReader(data))
	err = dec.Decode(&u)
	return
}

func (BoltUserManager) encodeUser(user User) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(user)
	return buf.Bytes()
}
