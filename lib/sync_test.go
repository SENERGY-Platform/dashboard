/*
 * Copyright 2026 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lib

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// keepSyncGlobals restores the clients Sync writes to.
func keepSyncGlobals(t *testing.T) {
	prevStandalone, prevReplica := StandaloneDB, ReplicaDB
	t.Cleanup(func() { StandaloneDB, ReplicaDB = prevStandalone, prevReplica })
}

// Connect to the standalone source is lazy, so an unreachable MONGO only fails once it is queried.
const unreachableHost = "127.0.0.1:1/?serverSelectionTimeoutMS=200"

func TestSyncReturnsStartupCheckError(t *testing.T) {
	const password = "pw-must-not-appear-91d2"
	keepSyncGlobals(t)
	StandaloneDB, ReplicaDB = nil, nil
	t.Setenv("MONGO", unreachableHost)
	err := Sync(MongoConfig{Url: unreachableURL, User: "dashboard", Password: password, AuthSource: "admin", Database: "db"})
	if err == nil || !strings.HasPrefix(err.Error(), "mongo startup check failed: ") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), password) {
		t.Error("error text contains the password")
	}
	if StandaloneDB != nil || ReplicaDB != nil {
		t.Error("clients must stay unset after a failed start")
	}
}

func TestSyncRejectsInvalidConfig(t *testing.T) {
	keepSyncGlobals(t)
	t.Setenv("MONGO", unreachableHost)
	if err := Sync(MongoConfig{Url: unreachableURL}); !errors.Is(err, errEmptyDatabase) {
		t.Fatalf("err = %v, want %v", err, errEmptyDatabase)
	}
}

func TestSyncReturnsStandaloneConnectError(t *testing.T) {
	keepSyncGlobals(t)
	t.Setenv("MONGO", "127.0.0.1:not-a-port")
	err := Sync(MongoConfig{Url: unreachableURL, Database: "db"})
	if err == nil || !strings.HasPrefix(err.Error(), "standalone database connect failed: ") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncCopiesIntoConfiguredDatabase(t *testing.T) {
	if err := startTestMongo(); err != nil {
		t.Fatalf("could not start the test database: %v", err)
	}
	uri, err := testMongoContainer.ConnectionString(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	keepSyncGlobals(t)
	// The source database name is fixed; this container is throwaway, so writing to it is safe.
	source := DB.Database("dashboard").Collection("dashboards")
	target := "dashboard_sync_test_" + primitive.NewObjectID().Hex()
	t.Cleanup(func() {
		_ = DB.Database("dashboard").Drop(context.Background())
		_ = DB.Database(target).Drop(context.Background())
	})
	id := primitive.NewObjectID()
	if _, err = source.InsertOne(context.Background(), bson.M{"_id": id, "userid": "u"}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MONGO", strings.TrimPrefix(uri, "mongodb://"))
	if err = Sync(MongoConfig{Url: uri, AuthSource: "admin", Database: target}); err != nil {
		t.Fatal(err)
	}
	n, err := DB.Database(target).Collection("dashboards").CountDocuments(context.Background(), bson.M{"_id": id})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("copied %d dashboards into %s, want 1", n, target)
	}
}
