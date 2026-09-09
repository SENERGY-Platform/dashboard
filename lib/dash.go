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
	"time"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func normalizeModelError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrBadRequest) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrInternalServerError) {
		return err
	}
	if errors.Is(err, primitive.ErrInvalidHex) {
		return errors.Join(ErrBadRequest, err)
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.Join(ErrNotFound, err)
	}
	return errors.Join(ErrInternalServerError, err)
}

func createDashboard(ctx context.Context, dash Dashboard, userId string) (result Dashboard, err error) {
	if err = dash.validate(); err != nil {
		return result, normalizeModelError(err)
	}
	dash.applyDefaults()
	dash.Id = primitive.NewObjectID()
	dash.UserId = userId
	dash.UpdatedAt = time.Now()
	_, err = Mongo().InsertOne(ctx, dash)
	if err != nil {
		log.Logger.Error("create dashboard failed", attributes.ErrorKey, err)
		return result, normalizeModelError(err)
	}
	return dash, nil
}

func getDashboard(ifNotModifiedSince *time.Time, id string, userId string, ctx context.Context) (modified bool, dash Dashboard, err error) {
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, dash, normalizeModelError(err)
	}

	err = Mongo().FindOne(ctx, bson.M{"_id": objectId, "userid": userId}).Decode(&dash)
	if err != nil {
		log.Logger.Error("find dashboard failed", attributes.ErrorKey, err)
		return false, dash, normalizeModelError(err)
	}
	dash.applyDefaults()
	modified = true
	if ifNotModifiedSince != nil {
		modified = dash.UpdatedAt.Truncate(time.Second).After(*ifNotModifiedSince)
	}
	return
}

func getDashboards(ctx context.Context, ifNotModifiedSince *time.Time, userId string) (modified bool, dashs []Dashboard, err error) {
	opts := options.Find().SetSort(bson.D{{Key: "index", Value: 1}})
	cur, err := Mongo().Find(ctx, bson.M{"userid": userId}, opts)
	if err != nil {
		return false, nil, normalizeModelError(err)
	}
	if err = cur.All(ctx, &dashs); err != nil {
		return false, nil, normalizeModelError(err)
	}
	for i := range dashs {
		dashs[i].applyDefaults()
	}

	if len(dashs) == 0 {
		log.Logger.Info("user has no dashboards, creating default")
		dash, err := createDefaultDashboard(ctx, userId)
		if err != nil {
			log.Logger.Error("create default dashboard failed", attributes.ErrorKey, err)
		} else {
			dashs = append(dashs, dash)
		}
	}
	modified = true
	if ifNotModifiedSince != nil {
		for _, dash := range dashs {
			modified = dash.UpdatedAt.Truncate(time.Second).After(*ifNotModifiedSince)
			if modified {
				break
			}
		}
	}
	return
}

func deleteDashboard(ctx context.Context, id string, userId string) (Response, error) {
	var old Dashboard
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return Response{}, normalizeModelError(err)
	}

	err = Mongo().FindOne(ctx, bson.M{"_id": objectId, "userid": userId}).Decode(&old)
	if err != nil {
		log.Logger.Error("read dashboard before delete failed", attributes.ErrorKey, err)
		return Response{}, normalizeModelError(err)
	}
	_, err = Mongo().DeleteOne(ctx, bson.M{"_id": objectId, "userid": userId})
	if err != nil {
		log.Logger.Error("delete dashboard failed", attributes.ErrorKey, err)
		return Response{}, normalizeModelError(err)
	}

	if old.Index != nil {
		// only the dashboards behind the deleted one move up: a twin at the same
		// index would otherwise sink below zero
		info, err := Mongo().UpdateMany(ctx,
			bson.M{"userid": userId, "index": bson.M{"$gt": *old.Index}}, bson.M{"$inc": bson.M{"index": -1}})
		if err != nil {
			log.Logger.Error("update dashboard indices after delete failed", attributes.ErrorKey, err)
			return Response{}, normalizeModelError(err)
		}
		log.Logger.Info("updated dashboard indices after delete", "modified_count", info.ModifiedCount)
	} else {
		log.Logger.Info("dashboard had no index, skipping update of other dashboards")
	}
	return Response{"ok"}, nil
}

func updateDashboard(newDashboard Dashboard, dashboardId string, userId string, ctx context.Context) (Dashboard, error) {
	if err := newDashboard.validate(); err != nil {
		return Dashboard{}, normalizeModelError(err)
	}
	previouslyUpdated := newDashboard.UpdatedAt // for race detection: if the dashboard was updated in the meantime, the update will fail because the previouslyUpdated timestamp does not match the current updatedAt timestamp in the database anymore
	newDashboard.UpdatedAt = time.Now()
	update := bson.M{
		"$set": newDashboard,
	}

	id, err := primitive.ObjectIDFromHex(dashboardId)
	if err != nil {
		return Dashboard{}, normalizeModelError(err)
	}

	updateResult, err := Mongo().UpdateOne(ctx, bson.M{"_id": id, "userid": userId, "updatedAt": previouslyUpdated}, update)

	if err != nil {
		log.Logger.Error("update dashboard failed", attributes.ErrorKey, err)
		return Dashboard{}, normalizeModelError(err)
	}

	if updateResult.MatchedCount == 0 {
		return Dashboard{}, errors.New("dashboard was updated by another request, please retry")
	}
	return newDashboard, nil
}

func getWidget(ctx context.Context, ifNotModifiedSince *time.Time, dashboardId string, widgetId string, userId string) (modified bool, lastModified *time.Time, widget Widget, err error) {
	dash := Dashboard{}
	objectID, err := primitive.ObjectIDFromHex(dashboardId)
	if err != nil {
		return false, nil, Widget{}, normalizeModelError(err)
	}
	err = Mongo().FindOne(ctx, bson.M{"_id": objectID, "userid": userId}).Decode(&dash)
	if err != nil {
		log.Logger.Error("find dashboard for widget read failed", attributes.ErrorKey, err)
		return false, nil, Widget{}, normalizeModelError(err)
	}

	id, err := primitive.ObjectIDFromHex(widgetId)
	if err != nil {
		log.Logger.Error("parse widget id failed", attributes.ErrorKey, err)
		return false, nil, Widget{}, normalizeModelError(err)
	}

	_, widget, err = dash.GetWidget(id)
	if err != nil {
		log.Logger.Error("get widget from dashboard failed", attributes.ErrorKey, err)
		return false, nil, Widget{}, normalizeModelError(err)
	}
	modified = true
	if ifNotModifiedSince != nil {
		modified = dash.UpdatedAt.Truncate(time.Second).After(*ifNotModifiedSince)
	}
	lastModified = &dash.UpdatedAt
	return modified, lastModified, widget, nil
}

func createWidget(ctx context.Context, dashboardId string, widget Widget, userId string) (result Widget, err error) {
	_, dash, err := getDashboard(nil, dashboardId, userId, ctx)
	if err != nil {
		return Widget{}, err
	}
	widgetResult, err := dash.addWidget(widget)
	if err != nil {
		log.Logger.Error("create widget failed", attributes.ErrorKey, err)
		return result, err
	}
	_, err = updateDashboard(dash, dashboardId, userId, ctx)

	return widgetResult, err
}

func updateWidget(ctx context.Context, dashboardId string, value interface{}, propertyToChange string, widgetID string, userId string) (err error) {
	for range 3 { // retry up to 3 times in case of concurrent updates
		_, dash, err := getDashboard(nil, dashboardId, userId, ctx)
		if err != nil {
			return err
		}
		err = dash.updateWidget(value, propertyToChange, widgetID)
		if err != nil {
			log.Logger.Error("update widget failed", attributes.ErrorKey, err)
			return err
		}
		dash, err = updateDashboard(dash, dashboardId, userId, ctx)
		if err == nil {
			return nil
		}
	}

	return err

}

func updateWidgetPositionInDashboard(positionUpdate WidgetPosition, userId string, ctx context.Context) (err error) {
	dashboardId := positionUpdate.DashboardOrigin
	_, dash, err := getDashboard(nil, dashboardId, userId, ctx)
	if err != nil {
		return err
	}

	i, _, err := dash.GetWidget(positionUpdate.Id)
	if err != nil {
		return err
	}

	dash.Widgets[i].X = positionUpdate.X
	dash.Widgets[i].Y = positionUpdate.Y
	dash.Widgets[i].W = positionUpdate.W
	dash.Widgets[i].H = positionUpdate.H

	_, err = updateDashboard(dash, dashboardId, userId, ctx)
	if err != nil {
		log.Logger.Error("update dashboard after widget position swap failed", attributes.ErrorKey, err)
		return err
	}
	return nil
}

func moveWidgetBetweenDashboards(positionUpdate WidgetPosition, userId string, ctx context.Context) (err error) {
	_, oldDash, err := getDashboard(nil, positionUpdate.DashboardOrigin, userId, ctx)
	if err != nil {
		return err
	}
	_, newDash, err := getDashboard(nil, positionUpdate.DashboardDestination, userId, ctx)
	if err != nil {
		return err
	}

	oldPosition, widget, err := oldDash.GetWidget(positionUpdate.Id)
	if err != nil {
		return err
	}

	err = oldDash.removeWidgetAt(oldPosition)
	if err != nil {
		return err
	}
	_, err = updateDashboard(oldDash, positionUpdate.DashboardOrigin, userId, ctx)
	if err != nil {
		log.Logger.Error("update source dashboard after widget move failed", attributes.ErrorKey, err)
		return err
	}

	widget.X = positionUpdate.X
	widget.Y = positionUpdate.Y
	widget.W = positionUpdate.W
	widget.H = positionUpdate.H
	err = newDash.insertWidgetAt(len(newDash.Widgets), widget)
	if err != nil {
		return err
	}

	_, err = updateDashboard(newDash, positionUpdate.DashboardDestination, userId, ctx)
	if err != nil {
		log.Logger.Error("update destination dashboard after widget move failed", attributes.ErrorKey, err)
		return err
	}

	return nil
}

func updateWidgetPositions(ctx context.Context, positionUpdates []WidgetPosition, userId string) (err error) {
	for _, positionUpdate := range positionUpdates {
		if positionUpdate.DashboardOrigin == positionUpdate.DashboardDestination {
			err = updateWidgetPositionInDashboard(positionUpdate, userId, ctx)
			if err != nil {
				return err
			}
		} else {
			err = moveWidgetBetweenDashboards(positionUpdate, userId, ctx)
			if err != nil {
				return err
			}
		}
	}

	return err
}

func deleteWidget(ctx context.Context, dashboardId string, widgetId string, userId string) (err error) {
	_, dash, err := getDashboard(nil, dashboardId, userId, ctx)
	if err != nil {
		return
	}
	err = dash.deleteWidget(widgetId)
	if err != nil {
		log.Logger.Error("delete widget failed", attributes.ErrorKey, err)
		return err
	}
	dash, err = updateDashboard(dash, dashboardId, userId, ctx)

	return err
}

func migrateDashboardIndices() (err error) {
	log.Logger.Info("adding indices to dashboards when needed")
	var dashs []Dashboard
	ctx := context.TODO()
	opts := options.Find().SetSort(bson.D{{Key: "userid", Value: 1}})
	cur, err := Mongo().Find(ctx, bson.M{}, opts)
	if err != nil {
		return
	}
	if err = cur.All(ctx, &dashs); err != nil {
		return err
	}
	lastUserId := ""
	userIndex := 0
	for _, dash := range dashs {
		if dash.UserId != lastUserId {
			userIndex = 0
		}
		lastUserId = dash.UserId
		// a negative index is what an earlier delete left behind and is repaired
		// like a missing one
		if dash.Index == nil || *dash.Index < 0 {
			index := userIndex
			dash.Index = &index
			log.Logger.Info("adding dashboard index", "index", userIndex, "dashboard_id", dash.Id.Hex(), "user_id", dash.UserId)
			updateDashboard(dash, dash.Id.Hex(), dash.UserId, context.TODO())
		}
		userIndex++
	}
	return nil
}

func migrateUpdatedAt() (err error) {
	log.Logger.Info("adding updatedAt to dashboards when needed")
	ctx := context.TODO()
	_, err = Mongo().UpdateMany(ctx, bson.M{"updatedAt": bson.M{"$exists": false}}, bson.M{"$currentDate": bson.M{"updatedAt": bson.M{"$type": "timestamp"}}})
	return err
}

// migrateFieldDefault backfills a field that was added to the dashboard model.
// updatedAt is left alone on purpose: the default is the behaviour these
// dashboards already had, so no client cache needs to be invalidated.
func migrateFieldDefault(field string, value interface{}) (err error) {
	log.Logger.Info("adding field to dashboards when needed", "field", field)
	info, err := Mongo().UpdateMany(context.TODO(), bson.M{field: bson.M{"$exists": false}}, bson.M{"$set": bson.M{field: value}})
	if err != nil {
		return err
	}
	log.Logger.Info("added field to dashboards", "field", field, "modified_count", info.ModifiedCount)
	return nil
}

func migrateLayoutMode() (err error) {
	return migrateFieldDefault("layout_mode", defaultLayoutMode())
}

func migrateColumns() (err error) {
	return migrateFieldDefault("columns", defaultColumns())
}

func createDefaultDashboard(ctx context.Context, userId string) (result Dashboard, err error) {
	result.Id = primitive.NewObjectID()
	uZero := 0
	result.UpdatedAt = time.Now()
	result.Index = &uZero
	result.UserId = userId
	result.Name = "System"
	result.RefreshTime = 0
	result.applyDefaults()
	result.Widgets = []Widget{
		{
			Id:         primitive.NewObjectID(),
			Name:       "Prozesse",
			Type:       "process_state",
			Properties: map[string]interface{}{},
		},

		{
			Id:         primitive.NewObjectID(),
			Name:       "Letzte Prozesse",
			Type:       "process_model_list",
			Properties: map[string]interface{}{},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Prozessausführungen",
			Type:       "charts_process_instances",
			Properties: map[string]interface{}{},
		},
		{
			Id:   primitive.NewObjectID(),
			Name: "Prozessprobleme",
			Type: "process_incident_list",
			Properties: map[string]interface{}{
				"limit": 10,
			},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Prozessausführungen pro Tag",
			Type:       "charts_process_deployments",
			Properties: map[string]interface{}{},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Gerätestatus",
			Type:       "devices_state",
			Properties: map[string]interface{}{},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Geräte pro Hub",
			Type:       "charts_device_per_gateway",
			Properties: map[string]interface{}{},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Ausfallquote pro Hub (Letzte 7 Tage)",
			Type:       "charts_device_downtime_rate_per_gateway",
			Properties: map[string]interface{}{},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Geräteausfallquote (Heute)",
			Type:       "charts_device_total_downtime",
			Properties: map[string]interface{}{},
		},
		{
			Id:         primitive.NewObjectID(),
			Name:       "Geräteausfälle (Letzte 7 Tage)",
			Type:       "device_downtime_list",
			Properties: map[string]interface{}{},
		},
	}

	_, err = Mongo().InsertOne(ctx, result)
	if err != nil {
		log.Logger.Error("create default dashboard failed", attributes.ErrorKey, err)
		return result, err
	}
	return result, nil
}
