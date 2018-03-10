package models

import (
	"github.com/globalsign/mgo/bson"
)

const (
	BiasGameTable            MongoDbCollection = "biasgame"
	BiasGameSuggestionsTable MongoDbCollection = "biasgamesuggestions"
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

type BiasGameSuggestionEntry struct {
	ID         bson.ObjectId `bson:"_id,omitempty"`
	UserID     string        // user who made the message
	Name       string
	GrouopName string
	Gender     string
	ImageURL   string
	ChannelID  string // channel suggestion was made in
	Status     string
	GroupMatch bool
	IdolMatch  bool
}
