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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Client

const (
	startupTimeout    = 10 * time.Second
	disconnectTimeout = 5 * time.Second
)

var (
	errEmptyDatabase   = errors.New("mongo database name must not be empty")
	errMissingPassword = errors.New("mongo password must not be empty when a mongo user is set")
)

func mongoRegistry() *bsoncodec.Registry {
	tM := reflect.TypeOf(bson.M{})
	return bson.NewRegistryBuilder().RegisterTypeMapEntry(bsontype.EmbeddedDocument, tM).Build()
}

// InitDB connects and runs the migrations; on any error the client is disconnected
// and DB and mongoDatabase keep their previous values.
func InitDB(cfg MongoConfig) error {
	client, err := openDB(cfg, startupTimeout)
	if err != nil {
		return err
	}
	log.Logger.Info("successfully connected to db", "database", cfg.Database)
	previousDB, previousDatabase := DB, mongoDatabase
	DB, mongoDatabase = client, cfg.Database
	if err = migrateDB(); err != nil {
		disconnect(client)
		DB, mongoDatabase = previousDB, previousDatabase
		return err
	}
	return nil
}

func migrateDB() error {
	migrate, err := strconv.ParseBool(GetEnv("MIGRATE", "true"))
	if err != nil {
		log.Logger.Warn("invalid value for MIGRATE environment variable, defaulting to true", attributes.ErrorKey, err)
		migrate = true
	}
	if !migrate {
		return nil
	}
	if err = migrateDashboardIndices(); err != nil {
		return fmt.Errorf("could not migrate dashboard indices: %w", err)
	}
	if err = migrateUpdatedAt(); err != nil {
		return fmt.Errorf("could not migrate dashboard updatedAt: %w", err)
	}
	if err = migrateLayoutMode(); err != nil {
		return fmt.Errorf("could not migrate dashboard layout_mode: %w", err)
	}
	if err = migrateColumns(); err != nil {
		return fmt.Errorf("could not migrate dashboard columns: %w", err)
	}
	return nil
}

// openDB validates the config before anything touches the network.
func openDB(cfg MongoConfig, timeout time.Duration) (*mongo.Client, error) {
	if err := validateMongoConfig(cfg); err != nil {
		return nil, err
	}
	return connect(clientOptions(cfg), cfg.Database, timeout)
}

// connect runs listCollections on the database because Connect is lazy and ping
// needs no authentication; wrong or missing credentials fail here, not later.
func connect(opts *options.ClientOptions, database string, timeout time.Duration) (*mongo.Client, error) {
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), timeout)
	defer cancelConnect()
	client, err := mongo.Connect(connectCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("database connect failed: %w", err)
	}
	checkCtx, cancelCheck := context.WithTimeout(context.Background(), timeout)
	defer cancelCheck()
	listOpts := options.ListCollections().SetNameOnly(true).SetAuthorizedCollections(true)
	if _, err = client.Database(database).ListCollectionNames(checkCtx, bson.D{}, listOpts); err != nil {
		disconnect(client)
		return nil, fmt.Errorf("mongo startup check failed: %w", err)
	}
	return client, nil
}

func validateMongoConfig(cfg MongoConfig) error {
	if strings.TrimSpace(cfg.Database) == "" {
		return errEmptyDatabase
	}
	if cfg.User != "" && cfg.Password.Value() == "" {
		return errMissingPassword
	}
	return nil
}

// clientOptions sets the credentials after the URI, so they replace any given in MONGO_URL.
func clientOptions(cfg MongoConfig) *options.ClientOptions {
	opts := options.Client().ApplyURI(cfg.Url).SetRegistry(mongoRegistry())
	if cfg.User != "" {
		opts.SetAuth(options.Credential{
			Username:   cfg.User,
			Password:   cfg.Password.Value(),
			AuthSource: cfg.AuthSource,
		})
	}
	return opts
}

// disconnect uses a fresh context, because the caller's may already have expired.
func disconnect(client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		log.Logger.Warn("could not disconnect from db", attributes.ErrorKey, err)
	}
}

// mongoDatabase is the database every collection lives in. InitDB sets it from the
// config; tests point it at a throwaway database so a run can never touch real data.
var mongoDatabase = "dashboard"

func Mongo() *mongo.Collection {
	return DB.Database(mongoDatabase).Collection("dashboards")

}

func CloseDB() {
	ctx, cf := context.WithTimeout(context.Background(), 10*time.Second)
	defer cf()
	if err := DB.Disconnect(ctx); err != nil {
		panic(err)
	}
}
