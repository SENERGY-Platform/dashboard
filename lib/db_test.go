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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const replicaSetURL = "mongodb://mongo-0.mongo:27017,mongo-1.mongo:27017/?replicaSet=rs0"

// Port 1 refuses connections; the short selection timeout keeps a failing startup check fast.
const unreachableURL = "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=200"

var mongoEnv = []string{"MONGO_URL", "MONGO_USER", "MONGO_PASSWORD", "MONGO_AUTH_SOURCE", "MONGO_DATABASE", "MONGO_REPL_URL"}

// clearMongoEnv unsets the variables for this test; t.Setenv restores them afterwards.
func clearMongoEnv(t *testing.T) {
	for _, k := range mongoEnv {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMongoConfigFromEnvDefaults(t *testing.T) {
	clearMongoEnv(t)
	want := MongoConfig{Url: "mongodb://localhost:27017", AuthSource: "admin", Database: "dashboard"}
	if got := MongoConfigFromEnv(); got != want {
		t.Errorf("config = %+v, want %+v", got, want)
	}
}

func TestMongoConfigFromEnvEmptyCountsAsUnset(t *testing.T) {
	clearMongoEnv(t)
	for _, k := range mongoEnv {
		t.Setenv(k, "")
	}
	want := MongoConfig{Url: "mongodb://localhost:27017", AuthSource: "admin", Database: "dashboard"}
	if got := MongoConfigFromEnv(); got != want {
		t.Errorf("config = %+v, want %+v", got, want)
	}
}

func TestMongoConfigFromEnvNames(t *testing.T) {
	clearMongoEnv(t)
	t.Setenv("MONGO_URL", replicaSetURL)
	t.Setenv("MONGO_USER", "dashboard")
	t.Setenv("MONGO_PASSWORD", "s3cret")
	t.Setenv("MONGO_AUTH_SOURCE", "users")
	t.Setenv("MONGO_DATABASE", "other_db")
	// The old name must no longer be read.
	t.Setenv("MONGO_REPL_URL", "mongodb://old:27017")
	want := MongoConfig{Url: replicaSetURL, User: "dashboard", Password: "s3cret", AuthSource: "users", Database: "other_db"}
	if got := MongoConfigFromEnv(); got != want {
		t.Errorf("config = %+v, want %+v", got, want)
	}
}

func TestMongoConfigMasksPassword(t *testing.T) {
	const password = "pw-must-not-appear-5c1e"
	cfg := MongoConfig{Url: "mongodb://localhost:27017", User: "dashboard", Password: password, AuthSource: "admin", Database: "dashboard"}
	if cfg.Password.Value() != password {
		t.Fatal("Value must return the password")
	}
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var jsonLog, textLog bytes.Buffer
	slog.New(slog.NewJSONHandler(&jsonLog, nil)).Info("config", "mongo", cfg, "password", cfg.Password)
	slog.New(slog.NewTextHandler(&textLog, nil)).Info("config", "mongo", cfg, "password", cfg.Password)
	outputs := map[string]string{
		"%v":        fmt.Sprintf("%v", cfg),
		"%+v":       fmt.Sprintf("%+v", cfg),
		"%#v":       fmt.Sprintf("%#v", cfg),
		"%s":        fmt.Sprintf("%s", cfg.Password),
		"json":      string(jsonBytes),
		"slog json": jsonLog.String(),
		"slog text": textLog.String(),
	}
	for name, out := range outputs {
		if strings.Contains(out, password) {
			t.Errorf("%s leaks the password: %s", name, out)
		}
	}
}

func TestClientOptionsSetsAuthWhenUserGiven(t *testing.T) {
	opts := clientOptions(MongoConfig{Url: replicaSetURL, User: "dashboard", Password: "s3cret", AuthSource: "admin", Database: "dashboard"})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	want := &options.Credential{Username: "dashboard", Password: "s3cret", AuthSource: "admin"}
	if !reflect.DeepEqual(opts.Auth, want) {
		t.Errorf("auth = %+v, want %+v", opts.Auth, want)
	}
}

func TestClientOptionsNoAuthWithoutUser(t *testing.T) {
	// A password without a user must not switch auth on.
	opts := clientOptions(MongoConfig{Url: "mongodb://localhost:27017", Password: "ignored-without-user", AuthSource: "admin", Database: "dashboard"})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if opts.Auth != nil {
		t.Errorf("expected no credentials, got user %q", opts.Auth.Username)
	}
}

func TestClientOptionsConfiguredAuthOverridesURI(t *testing.T) {
	opts := clientOptions(MongoConfig{
		Url:        "mongodb://other:pw@localhost:27017/?authSource=other&authMechanism=SCRAM-SHA-1",
		User:       "dashboard",
		Password:   "s3cret",
		AuthSource: "admin",
		Database:   "dashboard",
	})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	// AuthMechanism empty means the driver negotiates SCRAM instead of keeping the one from the URI.
	want := &options.Credential{Username: "dashboard", Password: "s3cret", AuthSource: "admin"}
	if !reflect.DeepEqual(opts.Auth, want) {
		t.Errorf("auth = %+v, want %+v", opts.Auth, want)
	}
}

func TestClientOptionsPassesURIUnchanged(t *testing.T) {
	opts := clientOptions(MongoConfig{Url: replicaSetURL, Database: "dashboard"})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if got := opts.GetURI(); got != replicaSetURL {
		t.Errorf("uri = %q, want %q", got, replicaSetURL)
	}
	if want := []string{"mongo-0.mongo:27017", "mongo-1.mongo:27017"}; !reflect.DeepEqual(opts.Hosts, want) {
		t.Errorf("hosts = %v, want %v", opts.Hosts, want)
	}
	if opts.ReplicaSet == nil || *opts.ReplicaSet != "rs0" {
		t.Errorf("replica set not taken from the uri: %v", opts.ReplicaSet)
	}
	if opts.Registry == nil {
		t.Error("the custom registry was dropped")
	}
}

func TestClientOptionsDoesNotAddScheme(t *testing.T) {
	if err := clientOptions(MongoConfig{Url: "localhost:27017", Database: "dashboard"}).Validate(); err == nil {
		t.Fatal("expected an error for a url without scheme")
	}
}

func TestValidateMongoConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MongoConfig
		wantErr error
	}{
		{"no auth", MongoConfig{Database: "db"}, nil},
		{"user and password", MongoConfig{Database: "db", User: "u", Password: "p"}, nil},
		{"password without user", MongoConfig{Database: "db", Password: "p"}, nil},
		{"user without password", MongoConfig{Database: "db", User: "u"}, errMissingPassword},
		{"empty database", MongoConfig{}, errEmptyDatabase},
		{"blank database", MongoConfig{Database: " "}, errEmptyDatabase},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateMongoConfig(tt.cfg); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// keepGlobals restores the package state InitDB writes to.
func keepGlobals(t *testing.T) {
	prevDB, prevDatabase := DB, mongoDatabase
	t.Cleanup(func() { DB, mongoDatabase = prevDB, prevDatabase })
}

// The startup check would fail as well, so these check for the specific validation error.
func TestInitDBRejectsEmptyDatabase(t *testing.T) {
	keepGlobals(t)
	if err := InitDB(MongoConfig{Url: unreachableURL}); !errors.Is(err, errEmptyDatabase) {
		t.Fatalf("err = %v, want %v", err, errEmptyDatabase)
	}
}

func TestInitDBRejectsUserWithoutPassword(t *testing.T) {
	keepGlobals(t)
	if err := InitDB(MongoConfig{Url: unreachableURL, Database: "db", User: "dashboard"}); !errors.Is(err, errMissingPassword) {
		t.Fatalf("err = %v, want %v", err, errMissingPassword)
	}
}

func TestInitDBFailsWhenStartupCheckFails(t *testing.T) {
	const password = "pw-must-not-appear-7f3a"
	keepGlobals(t)
	DB, mongoDatabase = nil, "untouched"
	start := time.Now()
	err := InitDB(MongoConfig{Url: unreachableURL, User: "dashboard", Password: password, AuthSource: "admin", Database: "db"})
	if err == nil {
		t.Fatal("expected an error when the server is unreachable")
	}
	if !strings.HasPrefix(err.Error(), "mongo startup check failed: ") {
		t.Errorf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), password) {
		t.Error("error text contains the password")
	}
	if DB != nil || mongoDatabase != "untouched" {
		t.Error("globals must stay unchanged after a failed startup check")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("InitDB took %v, the selection timeout from the uri was not applied", elapsed)
	}
}

func TestConnectDisconnectsWhenStartupCheckFails(t *testing.T) {
	closed := make(chan struct{}, 1)
	opts := clientOptions(MongoConfig{Url: unreachableURL, Database: "db"}).
		SetServerMonitor(&event.ServerMonitor{TopologyClosed: func(*event.TopologyClosedEvent) { closed <- struct{}{} }})
	client, err := connect(opts, "db", 2*time.Second)
	if err == nil || !strings.HasPrefix(err.Error(), "mongo startup check failed: ") {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("expected no client on failure")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Error("the client was not disconnected")
	}
}

func TestInitDBMigratesConfiguredDatabase(t *testing.T) {
	if err := startTestMongo(); err != nil {
		t.Fatalf("could not start the test database: %v", err)
	}
	uri, err := testMongoContainer.ConnectionString(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	base := DB
	database := "dashboard_init_test_" + primitive.NewObjectID().Hex()
	t.Cleanup(func() { _ = base.Database(database).Drop(context.Background()) })
	id := primitive.NewObjectID()
	// A dashboard written before updatedAt, layout_mode and columns existed.
	if _, err = base.Database(database).Collection("dashboards").InsertOne(context.Background(), bson.M{"_id": id, "userid": "u", "index": 0}); err != nil {
		t.Fatal(err)
	}

	keepGlobals(t)
	t.Setenv("MIGRATE", "")
	if err = InitDB(MongoConfig{Url: uri, AuthSource: "admin", Database: database}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { disconnect(DB) })
	if DB == base || mongoDatabase != database {
		t.Fatalf("globals not set: database %q", mongoDatabase)
	}
	var migrated bson.M
	if err = base.Database(database).Collection("dashboards").FindOne(context.Background(), bson.M{"_id": id}).Decode(&migrated); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"updatedAt", "layout_mode", "columns"} {
		if _, ok := migrated[field]; !ok {
			t.Errorf("migration did not add %s in the configured database: %v", field, migrated)
		}
	}
}

// TestInitDBAuthenticates needs a throwaway server with access control; MONGO_AUTH_TEST_USER and
// MONGO_AUTH_TEST_PASSWORD are root credentials, used to create and remove the test users.
func TestInitDBAuthenticates(t *testing.T) {
	url, rootUser, rootPassword := os.Getenv("MONGO_AUTH_TEST_URL"), os.Getenv("MONGO_AUTH_TEST_USER"), os.Getenv("MONGO_AUTH_TEST_PASSWORD")
	if testing.Short() || url == "" || rootUser == "" || rootPassword == "" {
		t.Skip("needs MONGO_AUTH_TEST_URL, MONGO_AUTH_TEST_USER and MONGO_AUTH_TEST_PASSWORD, not in -short")
	}
	ctx := context.Background()
	root, err := mongo.Connect(ctx, options.Client().ApplyURI(url).SetAuth(options.Credential{Username: rootUser, Password: rootPassword, AuthSource: "admin"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Disconnect(ctx) })

	suffix := randomHex(t)
	testDB, otherDB := "dashboard_auth_test_"+suffix, "dashboard_auth_other_"+suffix
	svcUser, svcPassword := "dashboard-test-"+suffix, randomHex(t)
	otherUser, otherPassword := "dashboard-other-"+suffix, randomHex(t)
	readUser, readPassword := "dashboard-read-"+suffix, randomHex(t)
	createUser(t, root, svcUser, svcPassword, "readWrite", testDB)
	createUser(t, root, otherUser, otherPassword, "readWrite", otherDB)
	createUser(t, root, readUser, readPassword, "read", testDB)

	cases := []struct {
		name, user, password, wantErrPrefix string
	}{
		{"correct credentials", svcUser, svcPassword, ""},
		{"no credentials", "", "", "mongo startup check failed: "},
		{"user of another database", otherUser, otherPassword, "mongo startup check failed: "},
		{"wrong password", svcUser, svcPassword + "-wrong", "mongo startup check failed: "},
		// Passes the startup check but cannot write, so a migration fails after connecting.
		{"read-only user", readUser, readPassword, "could not migrate dashboard "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			keepGlobals(t)
			t.Setenv("MIGRATE", "")
			DB, mongoDatabase = nil, "untouched"
			err := InitDB(MongoConfig{Url: url, User: c.user, Password: secret(c.password), AuthSource: "admin", Database: testDB})
			if c.wantErrPrefix == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				disconnect(DB)
				if mongoDatabase != testDB {
					t.Errorf("database = %q, want %q", mongoDatabase, testDB)
				}
				return
			}
			if err == nil {
				disconnect(DB)
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), c.wantErrPrefix) {
				t.Errorf("unexpected error: %v", err)
			}
			if DB != nil || mongoDatabase != "untouched" {
				t.Error("globals must stay unchanged after a failed start")
			}
			for _, pw := range []string{svcPassword, otherPassword, readPassword, rootPassword} {
				if strings.Contains(err.Error(), pw) {
					t.Error("error text contains a password")
				}
			}
		})
	}
}

// createUser registers its cleanup first, so a half-finished creation is removed as well.
func createUser(t *testing.T, root *mongo.Client, user, password, role, db string) {
	t.Helper()
	admin := root.Database("admin")
	t.Cleanup(func() {
		_ = admin.RunCommand(context.Background(), bson.D{{Key: "dropUser", Value: user}}).Err()
		_ = root.Database(db).Drop(context.Background())
	})
	cmd := bson.D{
		{Key: "createUser", Value: user},
		{Key: "pwd", Value: password},
		{Key: "roles", Value: bson.A{bson.D{{Key: "role", Value: role}, {Key: "db", Value: db}}}},
	}
	if err := admin.RunCommand(context.Background(), cmd).Err(); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func randomHex(t *testing.T) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
