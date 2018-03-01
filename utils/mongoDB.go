package utils

import (
	"errors"
	"reflect"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/models"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// MongoDBInsert is a generic insert function that will insert the given data assuming an ID field is passed in the data
func MongoDBInsert(collection models.MongoDbCollection, rawData interface{}) (recordID bson.ObjectId, err error) {

	// convert the raw interface data to its actual type
	recordData := reflect.ValueOf(rawData).Elem()

	// confirm data has an ID field
	idField := recordData.FieldByName("ID")
	if !idField.IsValid() {
		return bson.ObjectId(""), errors.New("invalid data")
	}

	// if the records id field isn't empty, give it an id
	newID := idField.String()
	if newID == "" {
		newID = string(bson.NewObjectId())
		idField.SetString(newID)
	}

	// insert record
	err = cache.GetMongoDB().C(collection.String()).Insert(recordData.Interface())
	if err != nil {
		return bson.ObjectId(""), err
	}

	return bson.ObjectId(newID), nil
}

// MongoDBSearch generic search function for
func MongoDBSearch(collection models.MongoDbCollection, selection interface{}) (query *mgo.Query) {
	return cache.GetMongoDB().C(collection.String()).Find(selection)
}
