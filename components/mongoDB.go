package components

import (
	"github.com/Snakeyesz/snek-bot/utils"

	"github.com/globalsign/mgo"

	"github.com/Snakeyesz/snek-bot/cache"
)

// ConnectMongoDB connects and logs in to mongo database.
//  then caches the session and database
func ConnectMongoDB() {

	// get host info
	dbHostAddr := cache.GetAppConfig().Path("mongo_db.hosts.localhost.db_address").Data().(string)
	dbName := cache.GetAppConfig().Path("mongo_db.hosts.localhost.db_name").Data().(string)

	// connect to db
	session, err := mgo.DialWithInfo(&mgo.DialInfo{
		Addrs: []string{
			dbHostAddr,
		},
	})
	utils.PanicCheck(err)

	// save session and database
	cache.SetMongoDBSession(session)
	cache.SetMongoDB(session.DB(dbName))
}
