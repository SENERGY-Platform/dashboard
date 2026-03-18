package lib

import (
	"context"
	"os"
	"reflect"
	"time"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var StandaloneDB *mongo.Client
var ReplicaDB *mongo.Client

func InitSyncDBs() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tM := reflect.TypeOf(bson.M{})
	reg := bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).Build()
	clientOpts := options.Client().ApplyURI("mongodb://" + os.Getenv("MONGO")).SetRegistry(reg)

	client, err := mongo.Connect(ctx, clientOpts)

	if err != nil {
		panic("database connect failed: " + err.Error())
	} else {
		log.Logger.Info("successfully connected to standalone db")
	}
	StandaloneDB = client
	log.Logger.Info("try to connect to replica db", "uri", os.Getenv("MONGO_REPL_URL"))
	clientOpts = options.Client().ApplyURI(os.Getenv("MONGO_REPL_URL")).SetRegistry(reg)

	client, err = mongo.Connect(ctx, clientOpts)

	if err != nil {
		panic("database connect failed: " + err.Error())
	} else {
		log.Logger.Info("successfully connected to replica db")
	}
	ReplicaDB = client
}

func GetOldDashs() []interface{} {
	standaloneCollection := StandaloneDB.Database("dashboard").Collection("dashboards")
	ctx := context.TODO()

	opts := options.Find()
	cur, err := standaloneCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		log.Logger.Error("find dashboards in standalone db failed", attributes.ErrorKey, err)
	}
	var dashs []interface{}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		log.Logger.Error("decode dashboards from standalone db failed", attributes.ErrorKey, err)
	}
	log.Logger.Info("old data sync scan complete", "dashboard_count", len(dashs))
	return dashs
}

func Insert(dashs []interface{}) {
	replicaCollection := ReplicaDB.Database("dashboard").Collection("dashboards")
	if len(dashs) == 0 {
		log.Logger.Info("no data to sync")
		return
	}

	log.Logger.Info("insert dashboards into replicaset", "count", len(dashs))
	_, err := replicaCollection.InsertMany(context.TODO(), dashs)
	if err != nil {
		panic(err)
	}
}

func Sync() {
	InitSyncDBs()
	dashs := GetOldDashs()
	Insert(dashs)
	CheckReplica()
}

func CheckReplica() {
	collection := ReplicaDB.Database("dashboard").Collection("dashboards")
	ctx := context.TODO()

	opts := options.Find()
	cur, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		log.Logger.Error("find dashboards in replica db failed", attributes.ErrorKey, err)
	}
	var dashs []interface{}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		log.Logger.Error("decode dashboards in replica db failed", attributes.ErrorKey, err)
	}
	log.Logger.Info("replica sync result", "dashboard_count", len(dashs))
}
