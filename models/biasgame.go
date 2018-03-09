package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	BiasGameTable MongoDbCollection = "biasgame"
)

type BiasEntry struct {
	Name      string
	GroupName string
	Gender    string
}

type SingleBiasGameEntry struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	UserID       string
	GuildID      string
	GameWinner   BiasEntry
	RoundWinners []BiasEntry
	RoundLosers  []BiasEntry
	Gender       string // girl, boy, mixed
}
