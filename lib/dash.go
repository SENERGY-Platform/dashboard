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
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

)

func createDashboard(dash Dashboard, userId string) (result Dashboard, err error) {
	ctx := context.TODO()
	dash.Id = primitive.NewObjectID()
	dash.UserId = userId
	_, err = Mongo().InsertOne(ctx, dash)
	if err != nil {
		fmt.Println("Error create:", err)
		return result, err
	}
	return dash, nil
}

func getDashboard(id string, userId string) (dash Dashboard, err error) {
	ctx := context.TODO()
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}

	err = Mongo().FindOne(ctx, bson.M{"_id": objectId, "userid": userId}).Decode(&dash)
	if err != nil {
		fmt.Println("Error find:", err)
	}
	return
}

func getDashboards(userId string) (dashs []Dashboard, err error) {
	ctx := context.TODO()
	opts := options.Find().SetSort(bson.D{{"index", 1}})
	cur, err := Mongo().Find(ctx, bson.M{"userid": userId}, opts)
	if err != nil {
		return nil, err
	}
	if err = cur.All(context.TODO(), &dashs); err != nil {
		return nil, err
	}

	if len(dashs) == 0 {
		fmt.Println("User has no dashboards, creating default")
		dash, err := createDefaultDashboard(userId)
		if err != nil {
			fmt.Println("ERROR: could not create default dashboard: ", err.Error())
		} else {
			dashs = append(dashs, dash)
		}
	}
	return
}

func deleteDashboard(id string, userId string) Response {
	var old Dashboard
	ctx := context.TODO()
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return Response{"ok"}
	}

	err = Mongo().FindOne(ctx, bson.M{"_id": objectId, "userid": userId}).Decode(&old)
	if err != nil {
		fmt.Println("Error remove:", err)
	}
	_, err = Mongo().DeleteOne(ctx, bson.M{"_id": objectId, "userid": userId})
	if err != nil {
		fmt.Println("Error remove:", err)
	}

	if old.Index != nil {
		// update indices
		info, err := Mongo().UpdateMany(ctx,
			bson.M{"userid": userId, "index": bson.M{"$gte": *old.Index}}, bson.M{"$inc": bson.M{"index": -1}})
		if err != nil {
			fmt.Println("Error remove:", err)
		}
		fmt.Println("Deletion of dashboard caused updating of indices of " + strconv.Itoa(int(info.ModifiedCount)) + " other dashboards")
	} else {
		fmt.Println("Dashboard had no index, skipping update of other dashboards")
	}
	return Response{"ok"}
}

func updateDashboard(newDashboard Dashboard, dashboardId string, userId string) (Dashboard, error) {
	ctx := context.TODO()

	wc := writeconcern.Majority()
	txnOptions := options.Transaction().SetWriteConcern(wc)
	// Starts a session on the client
	session, err := DB.StartSession()
	if err != nil {
		panic(err)
	}
	// Defers ending the session after the transaction is committed or ended
	defer session.EndSession(ctx)

	update := bson.M{
		"$set": newDashboard,
	}

	id, err := primitive.ObjectIDFromHex(dashboardId)
	if err != nil {
		return Dashboard{}, err
	}

	_, err = session.WithTransaction(context.TODO(), func(ctx mongo.SessionContext) (interface{}, error) {
		_, err = Mongo().UpdateOne(ctx, bson.M{"_id": id, "userid": userId}, update)
		return nil, err
	}, txnOptions)

	if err != nil {
		fmt.Println("Error update:", err)
		return Dashboard{}, err
	}
	return newDashboard, nil
}

func getWidget(dashboardId string, widgetId string, userId string) (widget Widget) {
	dash := Dashboard{}
	ctx := context.TODO()
	objectID, err := primitive.ObjectIDFromHex(dashboardId)
	if err != nil {
		return Widget{}
	}
	err = Mongo().FindOne(ctx, bson.M{"_id": objectID, "userid": userId}).Decode(&dash)
	if err != nil {
		fmt.Println("Error find:", err)
		return
	}

	id, err := primitive.ObjectIDFromHex(widgetId)
	if err != nil {
		fmt.Println("Error get id from hex: ", err)
	}

	_, widget, err = dash.GetWidget(id)
	if err != nil {
		fmt.Println("Error getWidget: ", err)
	}
	return
}

func createWidget(dashboardId string, widget Widget, userId string) (result Widget, err error) {
	dash, err := getDashboard(dashboardId, userId)
	if err != nil {
		return Widget{}, err
	}
	widgetResult, err := dash.addWidget(widget)
	if err != nil {
		fmt.Println("Error createWidget: ", err)
		return result, err
	}
	_, err = updateDashboard(dash, dashboardId, userId)

	return widgetResult, err
}

func updateWidget(dashboardId string, value interface{}, propertyToChange string, widgetID string, userId string) (err error) {	
	dash, err := getDashboard(dashboardId, userId)
	if err != nil {
		return err
	}
	err = dash.updateWidget(value, propertyToChange, widgetID)
	if err != nil {
		fmt.Println("Error updateWidget: ", err)
		return err
	}
	dash, err = updateDashboard(dash, dashboardId, userId)

	return err
}

func updateWidgetPositions(dashboardId string, widget []WidgetPosition, userId string) (err error) {
	dash, err := getDashboard(dashboardId, userId)
	if err != nil {
		return err
	}
	err = dash.updateWidgetPositions(widget)
	if err != nil {
		fmt.Println("Error updateWidgetPostition: ", err)
		return err
	}
	dash, err = updateDashboard(dash, dashboardId, userId)

	return err
}

func deleteWidget(dashboardId string, widgetId string, userId string) (err error) {
	dash, err := getDashboard(dashboardId, userId)
	if err != nil {
		return
	}
	err = dash.deleteWidget(widgetId)
	if err != nil {
		fmt.Println("Error deleteWidget: ", err)
		return err
	}
	dash, err = updateDashboard(dash, dashboardId, userId)

	return err
}

func migrateDashboardIndices() (err error) {
	fmt.Println("Adding indices to dashboards when needed")
	var dashs []Dashboard
	ctx := context.TODO()
	opts := options.Find().SetSort(bson.D{{"userid", 1}})
	cur, err := Mongo().Find(ctx, bson.M{}, opts)
	if err != nil {
		return
	}
	if err = cur.All(ctx, &dashs); err != nil {
		return err
	}
	lastUserId := ""
	userIndex := uint16(0)
	for _, dash := range dashs {
		if dash.UserId != lastUserId {
			userIndex = 0
		}
		lastUserId = dash.UserId
		if dash.Index == nil {
			dash.Index = &userIndex
			fmt.Println("Adding index " + strconv.Itoa(int(userIndex)) + " to dashboard " + dash.Id.Hex() + " of user " + dash.UserId)
			updateDashboard(dash, dash.Id.Hex(), dash.UserId)
		}
		userIndex++
	}
	return nil
}

func createDefaultDashboard(userId string) (result Dashboard, err error) {
	result.Id = primitive.NewObjectID()
	uZero := uint16(0)
	result.Index = &uZero
	result.UserId = userId
	result.Name = "System"
	result.RefreshTime = 0
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

	ctx := context.TODO()
	_, err = Mongo().InsertOne(ctx, result)
	if err != nil {
		fmt.Println("Error create:", err)
		return result, err
	}
	return result, nil
}
