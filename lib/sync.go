package lib

import (
	 "context"
	 "fmt"
	 "reflect"
	 "time"
	 "os"
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
		 fmt.Println("Successfully connected to DB!")
	 }
	 StandaloneDB = client
	 fmt.Println("Try to connect to " + os.Getenv("MONGO_REPL_URL"))
	 clientOpts = options.Client().ApplyURI(os.Getenv("MONGO_REPL_URL")).SetRegistry(reg)
 
	 client, err = mongo.Connect(ctx, clientOpts)
 
	 if err != nil {
		 panic("database connect failed: " + err.Error())
	 } else {
		 fmt.Println("Successfully connected to DB!")
	 }
	 ReplicaDB = client
}

func GetOldDashs() []interface{} {
	standaloneCollection := StandaloneDB.Database("dashboard").Collection("dashboards")
	ctx := context.TODO()

	opts := options.Find()
	cur, err := standaloneCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		fmt.Println("Error find:", err)
	}
	var dashs []interface{}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		fmt.Println("Error cur:", err)
	}
	fmt.Println("OLD DATA:")
	fmt.Printf("%d dashboards need to be synced!\n", len(dashs))
	return dashs
}

func Insert(dashs []interface{}) {
	replicaCollection := ReplicaDB.Database("dashboard").Collection("dashboards")
	if len(dashs) == 0 {
		fmt.Println("No data. Done!")
		return
	}

	fmt.Println("Insert into replicaset")
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
		fmt.Println("Error find:", err)
	}
	var dashs []interface{}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		fmt.Println("Error cur:", err)
	}
	fmt.Println("NEW DATA:")
	fmt.Printf("%d dashboards were synced\n", len(dashs))
}

