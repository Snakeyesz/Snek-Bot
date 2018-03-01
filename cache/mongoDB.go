package cache

import (
	"errors"
	"sync"

	"github.com/globalsign/mgo"
)

var (
	mongoDB        *mgo.Database
	mongoDBSession *mgo.Session
	mongoDBMutex   sync.RWMutex
)

func SetMongoDBSession(session *mgo.Session) {
	mongoDBMutex.Lock()
	defer mongoDBMutex.Unlock()

	mongoDBSession = session
}

func GetMongoDBSession() *mgo.Session {
	mongoDBMutex.RLock()
	defer mongoDBMutex.RUnlock()

	if mongoDBSession == nil {
		panic(errors.New("MongoDB session was not set before use"))
	}

	return mongoDBSession
}

func SetMongoDB(db *mgo.Database) {
	mongoDBMutex.Lock()
	defer mongoDBMutex.Unlock()

	mongoDB = db
}

func GetMongoDB() *mgo.Database {
	mongoDBMutex.RLock()
	defer mongoDBMutex.RUnlock()

	if mongoDB == nil {
		panic(errors.New("MongoDB session was not set before use"))
	}

	return mongoDB
}
