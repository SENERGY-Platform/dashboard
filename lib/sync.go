package lib

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var StandaloneDB *mongo.Client
var ReplicaDB *mongo.Client

// InitSyncDBs connects to the legacy standalone mongo in MONGO and, like InitDB, to the configured one.
func InitSyncDBs(cfg MongoConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tM := reflect.TypeOf(bson.M{})
	reg := bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).Build()
	clientOpts := options.Client().ApplyURI("mongodb://" + os.Getenv("MONGO")).SetRegistry(reg)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("standalone database connect failed: %w", err)
	}
	log.Logger.Info("successfully connected to standalone db")
	log.Logger.Info("try to connect to replica db", "database", cfg.Database)
	replica, err := openDB(cfg, startupTimeout)
	if err != nil {
		disconnect(client)
		return err
	}
	log.Logger.Info("successfully connected to replica db")
	StandaloneDB, ReplicaDB = client, replica
	return nil
}

func GetOldDashs() ([]interface{}, error) {
	standaloneCollection := StandaloneDB.Database("dashboard").Collection("dashboards")
	ctx := context.TODO()

	opts := options.Find()
	cur, err := standaloneCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find dashboards in standalone db failed: %w", err)
	}
	var dashs []interface{}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		return nil, fmt.Errorf("decode dashboards from standalone db failed: %w", err)
	}
	log.Logger.Info("old data sync scan complete", "dashboard_count", len(dashs))
	return dashs, nil
}

func Insert(dashs []interface{}, database string) error {
	replicaCollection := ReplicaDB.Database(database).Collection("dashboards")
	if len(dashs) == 0 {
		log.Logger.Info("no data to sync")
		return nil
	}

	log.Logger.Info("insert dashboards into replicaset", "count", len(dashs))
	if _, err := replicaCollection.InsertMany(context.TODO(), dashs); err != nil {
		return fmt.Errorf("insert dashboards into replica db failed: %w", err)
	}
	return nil
}

// Sync copies the dashboards from the legacy standalone mongo into the configured one.
func Sync(cfg MongoConfig) error {
	if err := InitSyncDBs(cfg); err != nil {
		return err
	}
	defer disconnect(StandaloneDB)
	defer disconnect(ReplicaDB)
	dashs, err := GetOldDashs()
	if err != nil {
		return err
	}
	if err = Insert(dashs, cfg.Database); err != nil {
		return err
	}
	return CheckReplica(cfg.Database)
}

func CheckReplica(database string) error {
	collection := ReplicaDB.Database(database).Collection("dashboards")
	ctx := context.TODO()

	opts := options.Find()
	cur, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return fmt.Errorf("find dashboards in replica db failed: %w", err)
	}
	var dashs []interface{}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		return fmt.Errorf("decode dashboards in replica db failed: %w", err)
	}
	log.Logger.Info("replica sync result", "dashboard_count", len(dashs))
	return nil
}
