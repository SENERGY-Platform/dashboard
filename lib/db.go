/*
 *
 *  Copyright 2019 InfAI (CC SES)
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package lib

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Client

func InitDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tM := reflect.TypeOf(bson.M{})
	reg := bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).Build()
	clientOpts := options.Client().ApplyURI(GetEnv("MONGO_REPL_URL", "mongodb://localhost:27017")).SetRegistry(reg)

	client, err := mongo.Connect(ctx, clientOpts)

	if err != nil {
		panic("database connect failed: " + err.Error())
	} else {
		fmt.Println("Successfully connected to DB!")
	}
	DB = client
	err = migrateDashboardIndices()
	if err != nil {
		panic("could not migrate dashboard indices: " + err.Error())
	}
	err = migrateUpdatedAt()
	if err != nil {
		panic("could not migrate dashboard updatedAt: " + err.Error())
	}
}

func Mongo() *mongo.Collection {
	return DB.Database("dashboard").Collection("dashboards")

}

func CloseDB() {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	if err := DB.Disconnect(ctx); err != nil {
		panic(err)
	}
}
