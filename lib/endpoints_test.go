/*
 *    Copyright 2026 InfAI (CC SES)
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// testMongoImage is pinned so a run cannot start passing or failing because the
// mongo tag moved underneath it.
const testMongoImage = "mongo:7"

var (
	testMongoOnce      sync.Once
	testMongoContainer *mongodb.MongoDBContainer
	testMongoErr       error
)

func TestMain(m *testing.M) {
	code := m.Run()
	stopTestMongo()
	os.Exit(code)
}

// startTestMongo brings up the throwaway mongo the first time a test asks for a
// database. Doing it lazily rather than in TestMain keeps the tests that never
// touch mongo fast, and lets them run on a machine without docker.
func startTestMongo() error {
	testMongoOnce.Do(func() {
		ctx := context.Background()
		container, err := mongodb.Run(ctx, testMongoImage)
		if err != nil {
			testMongoErr = err
			return
		}
		testMongoContainer = container

		uri, err := container.ConnectionString(ctx)
		if err != nil {
			testMongoErr = err
			return
		}
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetRegistry(mongoRegistry()))
		if err != nil {
			testMongoErr = fmt.Errorf("connect to %s: %w", uri, err)
			return
		}
		if err = client.Ping(ctx, nil); err != nil {
			testMongoErr = fmt.Errorf("ping %s: %w", uri, err)
			return
		}
		DB = client
	})
	return testMongoErr
}

func stopTestMongo() {
	if DB != nil {
		if err := DB.Disconnect(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, "could not disconnect from the test database:", err)
		}
	}
	if testMongoContainer != nil {
		if err := testcontainers.TerminateContainer(testMongoContainer); err != nil {
			fmt.Fprintln(os.Stderr, "could not terminate the test database:", err)
		}
	}
}

// testEnv drives the real router against the throwaway mongo.
type testEnv struct {
	t      *testing.T
	router *gin.Engine
	userId string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	if err := startTestMongo(); err != nil {
		// without a database the test proves nothing, so it fails rather than
		// skipping itself into a green run
		t.Fatalf("could not start the test database: %v", err)
	}

	// Every test gets its own database, so a collection-wide write - which is
	// exactly what the migrations do - cannot reach another test's documents.
	// mongoDatabase is package state, so tests must not call t.Parallel.
	previous := mongoDatabase
	database := "dashboard_test_" + primitive.NewObjectID().Hex()
	mongoDatabase = database
	t.Cleanup(func() {
		mongoDatabase = previous
		if err := DB.Database(database).Drop(context.Background()); err != nil {
			t.Logf("could not drop test database %s: %v", database, err)
		}
	})

	return &testEnv{t: t, router: NewRouter(), userId: "test-" + primitive.NewObjectID().Hex()}
}

func (this *testEnv) request(method string, path string, body interface{}, header map[string]string) *httptest.ResponseRecorder {
	this.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			this.t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UserId", this.userId)
	for key, value := range header {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	this.router.ServeHTTP(rec, req)
	return rec
}

func (this *testEnv) decode(rec *httptest.ResponseRecorder, target interface{}) {
	this.t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		this.t.Fatalf("could not decode %q: %v", rec.Body.String(), err)
	}
}

// createDashboard posts a dashboard and returns it as the client sees it.
func (this *testEnv) createDashboard(payload map[string]interface{}) map[string]interface{} {
	this.t.Helper()
	rec := this.request(http.MethodPost, "/dashboards", payload, nil)
	if rec.Code != http.StatusOK {
		this.t.Fatalf("create dashboard: expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	result := map[string]interface{}{}
	this.decode(rec, &result)
	return result
}

func (this *testEnv) getDashboard(id string) map[string]interface{} {
	this.t.Helper()
	rec := this.request(http.MethodGet, "/dashboards/"+id, nil, nil)
	if rec.Code != http.StatusOK {
		this.t.Fatalf("get dashboard: expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	result := map[string]interface{}{}
	this.decode(rec, &result)
	return result
}

func (this *testEnv) listDashboards(ifModifiedSince string) (*httptest.ResponseRecorder, []map[string]interface{}) {
	this.t.Helper()
	header := map[string]string{}
	if ifModifiedSince != "" {
		header["If-Modified-Since"] = ifModifiedSince
	}
	rec := this.request(http.MethodGet, "/dashboards", nil, header)
	if rec.Code != http.StatusOK {
		return rec, nil
	}
	result := []map[string]interface{}{}
	this.decode(rec, &result)
	return rec, result
}

// putDashboard mirrors the web ui, which sends the whole dashboard object back.
func (this *testEnv) putDashboard(id string, mutate func(payload map[string]interface{})) *httptest.ResponseRecorder {
	this.t.Helper()
	payload := this.getDashboard(id)
	if mutate != nil {
		mutate(payload)
	}
	return this.request(http.MethodPut, "/dashboards/"+id, payload, nil)
}

// fieldOf reads one field back through the api, failing when the key is missing
// entirely - which is what a client reads as "the backend does not support this".
func (this *testEnv) fieldOf(id string, field string) interface{} {
	this.t.Helper()
	dash := this.getDashboard(id)
	value, ok := dash[field]
	if !ok {
		this.t.Fatalf("dashboard %s has no %s: %v", id, field, dash)
	}
	return value
}

func (this *testEnv) idOf(dash map[string]interface{}) string {
	this.t.Helper()
	id, ok := dash["id"].(string)
	if !ok || id == "" {
		this.t.Fatalf("dashboard has no id: %v", dash)
	}
	return id
}

// setUpdatedAt puts a known write time on a dashboard. The cache tests need a
// dashboard whose last write is old enough to have an uncapped validator, and
// waiting for that in real time would make them slow and clock-dependent.
func (this *testEnv) setUpdatedAt(id string, at time.Time) {
	this.t.Helper()
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		this.t.Fatal(err)
	}
	if _, err = Mongo().UpdateOne(context.Background(), bson.M{"_id": objectId}, bson.M{"$set": bson.M{"updatedAt": at}}); err != nil {
		this.t.Fatal(err)
	}
}

func (this *testEnv) lastModified(rec *httptest.ResponseRecorder) time.Time {
	this.t.Helper()
	header := rec.Header().Get("Last-Modified")
	if header == "" {
		this.t.Fatal("expected a Last-Modified header")
	}
	parsed, err := time.Parse(http.TimeFormat, header)
	if err != nil {
		this.t.Fatalf("could not parse Last-Modified %q: %v", header, err)
	}
	return parsed
}

// anHourAgo is a write time far enough back that cacheValidator hands it out
// unchanged instead of capping it to the previous second.
func anHourAgo() time.Time {
	return time.Now().Add(-time.Hour).Truncate(time.Second)
}

// settingDefaults are the values a dashboard reports for the settings it was
// created without, in the shape json decoding produces.
var settingDefaults = map[string]interface{}{
	"layout_mode": string(defaultLayoutMode()),
	"columns":     float64(defaultColumns()),
}

// asNumber makes values comparable across the wire formats: json decodes numbers
// into float64, mongo hands them back as int32 or int64.
func asNumber(value interface{}) interface{} {
	switch typed := value.(type) {
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case int:
		return float64(typed)
	}
	return value
}

// acceptance check 1: a create without the setting reads back as the default,
// everywhere a client can read it and in the stored document
func TestCreateDashboardDefaultsSettings(t *testing.T) {
	env := newTestEnv(t)
	created := env.createDashboard(map[string]interface{}{"name": "no settings", "widgets": []interface{}{}})
	id := env.idOf(created)

	_, list := env.listDashboards("")
	if len(list) != 1 {
		t.Fatalf("expected 1 dashboard, got %d", len(list))
	}
	stored := bson.M{}
	if err := Mongo().FindOne(context.Background(), bson.M{"userid": env.userId}).Decode(&stored); err != nil {
		t.Fatal(err)
	}

	for field, expected := range settingDefaults {
		t.Run(field, func(t *testing.T) {
			if asNumber(created[field]) != expected {
				t.Errorf("create response: expected %v, got %v", expected, created[field])
			}
			if got := asNumber(env.fieldOf(id, field)); got != expected {
				t.Errorf("get dashboard: expected %v, got %v", expected, got)
			}
			if asNumber(list[0][field]) != expected {
				t.Errorf("list dashboards: expected %v, got %v", expected, list[0][field])
			}
			// the default has to be persisted, not only added on the way out
			if asNumber(stored[field]) != expected {
				t.Errorf("stored document: expected %v, got %v", expected, stored[field])
			}
		})
	}
}

func TestCreateDashboardAcceptsSettings(t *testing.T) {
	env := newTestEnv(t)
	created := env.createDashboard(map[string]interface{}{
		"name":        "explicit settings",
		"layout_mode": string(LayoutModeMoveScale),
		"columns":     6,
	})
	id := env.idOf(created)
	if got := env.fieldOf(id, "layout_mode"); got != string(LayoutModeMoveScale) {
		t.Errorf("expected %q, got %v", LayoutModeMoveScale, got)
	}
	if got := env.fieldOf(id, "columns"); got != float64(6) {
		t.Errorf("expected 6, got %v", got)
	}
}

// settingChanges are values differing from the default, one per setting.
var settingChanges = map[string]interface{}{
	"layout_mode": string(LayoutModeMoveScale),
	"columns":     float64(6),
}

// acceptance check 2: an update persists the new value
func TestUpdateDashboardPersistsSettings(t *testing.T) {
	for field, value := range settingChanges {
		t.Run(field, func(t *testing.T) {
			env := newTestEnv(t)
			id := env.idOf(env.createDashboard(map[string]interface{}{"name": "to be changed"}))

			rec := env.putDashboard(id, func(payload map[string]interface{}) {
				payload[field] = value
			})
			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
			}
			if got := asNumber(env.fieldOf(id, field)); got != value {
				t.Errorf("expected %v, got %v", value, got)
			}
		})
	}
}

// acceptance check 3: an older client that does not know the field must not reset it
func TestUpdateDashboardWithoutSettingKeepsStoredValue(t *testing.T) {
	for field, value := range settingChanges {
		t.Run(field, func(t *testing.T) {
			env := newTestEnv(t)
			id := env.idOf(env.createDashboard(map[string]interface{}{"name": "changed", field: value}))

			rec := env.putDashboard(id, func(payload map[string]interface{}) {
				delete(payload, field)
				payload["name"] = "renamed by an old client"
			})
			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
			}
			response := map[string]interface{}{}
			env.decode(rec, &response)
			if asNumber(response[field]) != value {
				t.Errorf("update response: expected %v, got %v", value, response[field])
			}

			dash := env.getDashboard(id)
			if asNumber(dash[field]) != value {
				t.Errorf("expected %v, got %v", value, dash[field])
			}
			if dash["name"] != "renamed by an old client" {
				t.Errorf("the rest of the update was not applied: %v", dash["name"])
			}
		})
	}
}

// columns is 0 by default, so it is the field most likely to be swallowed by a
// bson omitempty on the way to mongo
func TestUpdateDashboardPersistsColumnsBackToAuto(t *testing.T) {
	env := newTestEnv(t)
	id := env.idOf(env.createDashboard(map[string]interface{}{"name": "six columns", "columns": 6}))

	rec := env.putDashboard(id, func(payload map[string]interface{}) {
		payload["columns"] = ColumnsAuto
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := env.fieldOf(id, "columns"); got != float64(ColumnsAuto) {
		t.Errorf("expected %d, got %v", ColumnsAuto, got)
	}
	stored := bson.M{}
	if err := Mongo().FindOne(context.Background(), bson.M{"userid": env.userId}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if asNumber(stored["columns"]) != float64(ColumnsAuto) {
		t.Errorf("stored document: expected %d, got %v", ColumnsAuto, stored["columns"])
	}
}

// the persistence layer must leave stored settings alone when it is handed nil
// values, which is what the index migration does with legacy documents
func TestUpdateDashboardWithNilSettingsDoesNotClearThem(t *testing.T) {
	env := newTestEnv(t)
	id := env.idOf(env.createDashboard(map[string]interface{}{
		"name":        "valid",
		"layout_mode": settingChanges["layout_mode"],
		"columns":     settingChanges["columns"],
	}))
	ctx := context.Background()

	_, dash, err := getDashboard(nil, id, env.userId, ctx)
	if err != nil {
		t.Fatal(err)
	}
	dash.LayoutMode = nil
	dash.Columns = nil
	dash.Name = "renamed"
	if _, err = updateDashboard(dash, id, env.userId, ctx); err != nil {
		t.Fatal(err)
	}

	stored := bson.M{}
	if err = Mongo().FindOne(ctx, bson.M{"_id": dash.Id}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	for field, expected := range settingChanges {
		if asNumber(stored[field]) != expected {
			t.Errorf("expected the stored %s %v to survive, got %v", field, expected, stored[field])
		}
	}
	if stored["name"] != "renamed" {
		t.Errorf("the rest of the update was not applied: %v", stored["name"])
	}
}

// invalidSettings are values the service has to refuse rather than store verbatim
// or quietly coerce to the default.
var invalidSettings = []struct {
	field string
	value interface{}
}{
	{field: "layout_mode", value: "nonsense"},
	{field: "layout_mode", value: "MoveScale"},
	{field: "layout_mode", value: ""},
	{field: "columns", value: MaxColumns + 1},
	{field: "columns", value: -1},
	{field: "columns", value: 100},
	{field: "columns", value: "six"},
}

// acceptance check 4: a typo is rejected and changes nothing
func TestUpdateDashboardRejectsInvalidSettings(t *testing.T) {
	for _, c := range invalidSettings {
		t.Run(fmt.Sprintf("%s=%v", c.field, c.value), func(t *testing.T) {
			env := newTestEnv(t)
			id := env.idOf(env.createDashboard(map[string]interface{}{
				"name":        "valid",
				"layout_mode": settingChanges["layout_mode"],
				"columns":     settingChanges["columns"],
			}))

			rec := env.putDashboard(id, func(payload map[string]interface{}) {
				payload[c.field] = c.value
			})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
			}
			if !bytes.Contains(bytes.ToLower(rec.Body.Bytes()), bytes.ToLower([]byte(c.field))) {
				t.Errorf("expected the error to name the field, got %q", rec.Body.String())
			}
			for field, expected := range settingChanges {
				if got := asNumber(env.fieldOf(id, field)); got != expected {
					t.Errorf("stored %s changed: expected %v, got %v", field, expected, got)
				}
			}
		})
	}
}

func TestCreateDashboardRejectsInvalidSettings(t *testing.T) {
	for _, c := range invalidSettings {
		t.Run(fmt.Sprintf("%s=%v", c.field, c.value), func(t *testing.T) {
			env := newTestEnv(t)
			rec := env.request(http.MethodPost, "/dashboards", map[string]interface{}{"name": "bad", c.field: c.value}, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
			}
			count, err := Mongo().CountDocuments(context.Background(), bson.M{"userid": env.userId})
			if err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("expected no dashboard to be stored, found %d", count)
			}
		})
	}
}

// the boundaries of the accepted range have to stay accepted
func TestUpdateDashboardAcceptsColumnsRange(t *testing.T) {
	env := newTestEnv(t)
	id := env.idOf(env.createDashboard(map[string]interface{}{"name": "columns"}))
	for columns := ColumnsAuto; columns <= MaxColumns; columns++ {
		rec := env.putDashboard(id, func(payload map[string]interface{}) {
			payload["columns"] = columns
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("columns %d: expected %d, got %d: %s", columns, http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := env.fieldOf(id, "columns"); got != float64(columns) {
			t.Errorf("expected %d, got %v", columns, got)
		}
	}
}

// acceptance check 5: rows written before these settings existed report the default
func TestDashboardWrittenBeforeSettingsExisted(t *testing.T) {
	env := newTestEnv(t)
	id := primitive.NewObjectID()
	index := 0
	_, err := Mongo().InsertOne(context.Background(), bson.M{
		"_id":         id,
		"userid":      env.userId,
		"name":        "legacy",
		"refreshtime": 0,
		"index":       index,
		"widgets":     []interface{}{},
		"updatedAt":   time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, list := env.listDashboards("")
	if len(list) != 1 {
		t.Fatalf("expected 1 dashboard, got %d", len(list))
	}
	for field, expected := range settingDefaults {
		if got := asNumber(env.fieldOf(id.Hex(), field)); got != expected {
			t.Errorf("get dashboard: expected %s %v, got %v", field, expected, got)
		}
		if asNumber(list[0][field]) != expected {
			t.Errorf("list dashboards: expected %s %v, got %v", field, expected, list[0][field])
		}
	}

	// the migrations write the same defaults into the stored document
	if err = migrateLayoutMode(); err != nil {
		t.Fatal(err)
	}
	if err = migrateColumns(); err != nil {
		t.Fatal(err)
	}
	stored := bson.M{}
	if err = Mongo().FindOne(context.Background(), bson.M{"_id": id}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	for field, expected := range settingDefaults {
		if asNumber(stored[field]) != expected {
			t.Errorf("migrated document: expected %s %v, got %v", field, expected, stored[field])
		}
	}

	// a legacy dashboard can still be switched over
	rec := env.putDashboard(id.Hex(), func(payload map[string]interface{}) {
		for field, value := range settingChanges {
			payload[field] = value
		}
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	for field, expected := range settingChanges {
		if got := asNumber(env.fieldOf(id.Hex(), field)); got != expected {
			t.Errorf("expected %s %v, got %v", field, expected, got)
		}
	}
}

// acceptance check 6: a client caching the list on Last-Modified must not keep a
// stale copy after an update that only touches one setting.
func TestSettingOnlyUpdateInvalidatesCachedDashboardList(t *testing.T) {
	for field, value := range settingChanges {
		t.Run(field, func(t *testing.T) {
			env := newTestEnv(t)
			id := env.idOf(env.createDashboard(map[string]interface{}{"name": "cached"}))
			env.setUpdatedAt(id, anHourAgo())

			rec, list := env.listDashboards("")
			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
			}
			cached := rec.Header().Get("Last-Modified")
			if cached == "" {
				t.Fatal("expected a Last-Modified header")
			}
			if asNumber(list[0][field]) != settingDefaults[field] {
				t.Fatalf("expected %v, got %v", settingDefaults[field], list[0][field])
			}

			putRec := env.putDashboard(id, func(payload map[string]interface{}) {
				payload[field] = value
			})
			if putRec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d: %s", http.StatusOK, putRec.Code, putRec.Body.String())
			}

			rec, list = env.listDashboards(cached)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d after the update, got %d: the client would keep serving its stale cache", http.StatusOK, rec.Code)
			}
			if asNumber(list[0][field]) != value {
				t.Errorf("expected %v, got %v", value, list[0][field])
			}
		})
	}
}

// The dangerous case for the above is a write landing in the same second as the
// read that produced the validator, because Last-Modified carries whole seconds
// and could not tell the two apart. Reproducing that second would mean racing the
// clock, so this asserts the invariant that makes it safe instead: the validator
// handed out never names the current second, so any write in that second is
// strictly newer than it. TestCacheValidator covers the arithmetic itself.
func TestDashboardListValidatorNeverNamesTheCurrentSecond(t *testing.T) {
	env := newTestEnv(t)
	env.createDashboard(map[string]interface{}{"name": "just written"})

	rec, _ := env.listDashboards("")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	validator := env.lastModified(rec)
	if !validator.Before(startOfSecond(time.Now())) {
		t.Fatalf("validator %s names the current second: a write following this read would be reported as not modified", validator)
	}
}

// capping the validator must not disable conditional requests altogether
func TestDashboardListStillReportsNotModified(t *testing.T) {
	env := newTestEnv(t)
	id := env.idOf(env.createDashboard(map[string]interface{}{"name": "cached"}))
	// a write old enough that its validator is handed out uncapped
	env.setUpdatedAt(id, anHourAgo())

	rec, _ := env.listDashboards("")
	lastModified := rec.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Fatal("expected a Last-Modified header")
	}

	rec, _ = env.listDashboards(lastModified)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("expected %d for an unchanged list, got %d", http.StatusNotModified, rec.Code)
	}
}

func startOfSecond(t time.Time) time.Time {
	return t.Truncate(time.Second)
}

// acceptance check 7: none of this touches widgets, which have their own endpoints
func TestUpdateDashboardLeavesWidgetsAlone(t *testing.T) {
	env := newTestEnv(t)
	id := env.idOf(env.createDashboard(map[string]interface{}{"name": "with widgets"}))

	rec := env.request(http.MethodPost, "/widgets/"+id, map[string]interface{}{
		"name":       "a widget",
		"type":       "process_state",
		"properties": map[string]interface{}{"limit": 10},
		"x":          1, "y": 2, "w": 3, "h": 4,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("create widget: expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	before := env.getDashboard(id)["widgets"]

	// the web ui sends the whole widget array along; the update must ignore it
	// exactly as it did before these settings existed
	putRec := env.putDashboard(id, func(payload map[string]interface{}) {
		for field, value := range settingChanges {
			payload[field] = value
		}
		payload["widgets"] = []interface{}{map[string]interface{}{"name": "sent by the client", "type": "devices_state"}}
	})
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, putRec.Code, putRec.Body.String())
	}

	after := env.getDashboard(id)
	if !reflect.DeepEqual(before, after["widgets"]) {
		t.Errorf("widgets changed\nbefore: %v\nafter:  %v", before, after["widgets"])
	}
	for field, expected := range settingChanges {
		if asNumber(after[field]) != expected {
			t.Errorf("expected %s %v, got %v", field, expected, after[field])
		}
	}
}

func TestWidgetUpdatesKeepDashboardSettings(t *testing.T) {
	env := newTestEnv(t)
	id := env.idOf(env.createDashboard(map[string]interface{}{
		"name":        "valid",
		"layout_mode": settingChanges["layout_mode"],
		"columns":     settingChanges["columns"],
	}))

	rec := env.request(http.MethodPost, "/widgets/"+id, map[string]interface{}{"name": "a widget", "type": "process_state"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("create widget: expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	widget := map[string]interface{}{}
	env.decode(rec, &widget)

	rec = env.request(http.MethodPatch, "/widgets/properties/"+id+"/"+widget["id"].(string), map[string]interface{}{"limit": 5}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("update widget properties: expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	for field, expected := range settingChanges {
		if got := asNumber(env.fieldOf(id, field)); got != expected {
			t.Errorf("widget updates reset %s: expected %v, got %v", field, expected, got)
		}
	}
}

// Two dashboards may share an index - imports and the default dashboard wrote
// fixed values - and a delete must shift only the ones behind the deleted one:
// with `$gte` the twin sank below zero, where a uint16 decode made every read
// of the account fail.
func TestDeleteDashboardShiftsOnlyTheDashboardsBehindIt(t *testing.T) {
	env := newTestEnv(t)
	first := env.createDashboard(map[string]interface{}{"name": "first", "index": 0, "widgets": []interface{}{}})
	twin := env.createDashboard(map[string]interface{}{"name": "twin", "index": 0, "widgets": []interface{}{}})
	behind := env.createDashboard(map[string]interface{}{"name": "behind", "index": 1, "widgets": []interface{}{}})

	rec := env.request(http.MethodDelete, "/dashboards/"+env.idOf(first), nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := asNumber(env.fieldOf(env.idOf(twin), "index")); got != float64(0) {
		t.Errorf("the twin at the same index has to stay at 0, got %v", got)
	}
	if got := asNumber(env.fieldOf(env.idOf(behind), "index")); got != float64(0) {
		t.Errorf("the dashboard behind has to move up to 0, got %v", got)
	}
	rec, list := env.listDashboards("")
	if rec.Code != http.StatusOK || len(list) != 2 {
		t.Fatalf("list after delete: expected 200 with 2 dashboards, got %d with %d: %s", rec.Code, len(list), rec.Body.String())
	}
}

// A document that already carries a negative index has to stay readable, and
// the start-up migration puts it back into the sequence of its user.
func TestANegativeIndexIsReadableAndRepaired(t *testing.T) {
	env := newTestEnv(t)
	id := primitive.NewObjectID()
	_, err := Mongo().InsertOne(context.Background(), bson.M{
		"_id":         id,
		"userid":      env.userId,
		"name":        "sunk",
		"refreshtime": 0,
		"index":       -1,
		"widgets":     []interface{}{},
		"updatedAt":   time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, list := env.listDashboards("")
	if rec.Code != http.StatusOK || len(list) != 1 {
		t.Fatalf("list: expected 200 with 1 dashboard, got %d with %d: %s", rec.Code, len(list), rec.Body.String())
	}
	if err = migrateDashboardIndices(); err != nil {
		t.Fatal(err)
	}
	if got := asNumber(env.fieldOf(id.Hex(), "index")); got != float64(0) {
		t.Errorf("migrated index: expected 0, got %v", got)
	}
}

func TestCreateDashboardRejectsANegativeIndex(t *testing.T) {
	env := newTestEnv(t)
	rec := env.request(http.MethodPost, "/dashboards", map[string]interface{}{"name": "x", "index": -1, "widgets": []interface{}{}}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
