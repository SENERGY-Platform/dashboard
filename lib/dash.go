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
	"fmt"
	"github.com/globalsign/mgo/bson"
	"strconv"
)

func createDashboard(dash Dashboard, userId string) (result Dashboard, err error) {
	dash.Id = bson.NewObjectId()
	dash.UserId = userId
	err = Mongo().Insert(dash)
	if err != nil {
		fmt.Println("Error create:", err)
		return result, err
	}
	return dash, nil
}

func getDashboard(id string, userId string) (dash Dashboard) {
	err := Mongo().Find(bson.M{"_id": bson.ObjectIdHex(id), "userid": userId}).One(&dash)
	if err != nil {
		fmt.Println("Error find:", err)
	}
	return
}

func getDashboards(userId string) (dashs []Dashboard) {
	Mongo().Find(bson.M{"userid": userId}).Sort("index").All(&dashs)
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
	err := Mongo().Find(bson.M{"_id": bson.ObjectIdHex(id), "userid": userId}).One(&old)
	if err != nil {
		fmt.Println("Error remove:", err)
	}
	err = Mongo().Remove(bson.M{"_id": bson.ObjectIdHex(id), "userid": userId})
	if err != nil {
		fmt.Println("Error remove:", err)
	}

	// update indices
	info, err := Mongo().UpdateAll(bson.M{"userid": userId, "index": bson.M{"$gte": *old.Index}}, bson.M{"$inc": bson.M{"index": -1}})
	if err != nil {
		fmt.Println("Error remove:", err)
	}
	fmt.Println("Deletion of dashboard caused updating of indices of " + strconv.Itoa(info.Updated) + " other dashboards")
	return Response{"ok"}
}

func updateDashboard(dash Dashboard, userId string) Dashboard {
	for index, widget := range dash.Widgets {
		if !widget.Id.Valid() {
			dash.Widgets[index].Id = bson.NewObjectId()
		}
	}

	err := Mongo().Update(bson.M{"_id": bson.ObjectId(dash.Id), "userid": userId}, dash)
	if err != nil {
		fmt.Println("Error update:", err)
	}
	return dash
}

func getWidget(dashboardId string, widgetId string, userId string) (widget Widget) {
	dash := Dashboard{}
	err := Mongo().Find(bson.M{"_id": bson.ObjectIdHex(dashboardId), "userid": userId}).One(&dash)
	if err != nil {
		fmt.Println("Error find:", err)
		return
	}
	widget, err = dash.GetWidget(bson.ObjectIdHex(widgetId))
	if err != nil {
		fmt.Println("Error getWidget: ", err)
	}
	return
}

func createWidget(dashboardId string, widget Widget, userId string) (result Widget, err error) {
	dash := getDashboard(dashboardId, userId)
	widgetResult, err := dash.addWidget(widget)
	if err != nil {
		fmt.Println("Error createWidget: ", err)
		return result, err
	}
	updateDashboard(dash, userId)

	return widgetResult, nil
}

func updateWidget(dashboardId string, widget Widget, userId string) (err error) {
	dash := getDashboard(dashboardId, userId)
	err = dash.updateWidget(widget)
	if err != nil {
		fmt.Println("Error updateWidget: ", err)
		return err
	}
	updateDashboard(dash, userId)

	return nil
}

func deleteWidget(dashboardId string, widgetId string, userId string) (err error) {
	dash := getDashboard(dashboardId, userId)
	err = dash.deleteWidget(widgetId)
	if err != nil {
		fmt.Println("Error deleteWidget: ", err)
		return err
	}
	updateDashboard(dash, userId)

	return nil
}

func migrateDashboardIndices() (err error) {
	fmt.Println("Adding indices to dashboards when needed")
	var dashs []Dashboard
	err = Mongo().Find(bson.M{}).Sort("userid").All(&dashs)
	if err != nil {
		return
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
			updateDashboard(dash, dash.UserId)
		}
		userIndex++
	}
	return nil
}

func createDefaultDashboard(userId string) (result Dashboard, err error) {
	result.Id = bson.NewObjectId()
	uZero := uint16(0)
	result.Index = &uZero
	result.UserId = userId
	result.Name = "System"
	result.RefreshTime = 0
	result.Widgets = []Widget{
		{
			Id:         bson.NewObjectId(),
			Name:       "Prozesse",
			Type:       "process_state",
			Properties: map[string]interface{}{},
		},

		{
			Id:         bson.NewObjectId(),
			Name:       "Letzte Prozesse",
			Type:       "process_model_list",
			Properties: map[string]interface{}{},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Prozessausführungen",
			Type:       "charts_process_instances",
			Properties: map[string]interface{}{},
		},
		{
			Id:   bson.NewObjectId(),
			Name: "Prozessprobleme",
			Type: "process_incident_list",
			Properties: map[string]interface{}{
				"limit": 10,
			},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Prozessausführungen pro Tag",
			Type:       "charts_process_deployments",
			Properties: map[string]interface{}{},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Gerätestatus",
			Type:       "devices_state",
			Properties: map[string]interface{}{},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Geräte pro Hub",
			Type:       "charts_device_per_gateway",
			Properties: map[string]interface{}{},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Ausfallquote pro Hub (Letzte 7 Tage)",
			Type:       "charts_device_downtime_rate_per_gateway",
			Properties: map[string]interface{}{},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Geräteausfallquote (Heute)",
			Type:       "charts_device_total_downtime",
			Properties: map[string]interface{}{},
		},
		{
			Id:         bson.NewObjectId(),
			Name:       "Geräteausfälle (Letzte 7 Tage)",
			Type:       "device_downtime_list",
			Properties: map[string]interface{}{},
		},
	}

	err = Mongo().Insert(result)
	if err != nil {
		fmt.Println("Error create:", err)
		return result, err
	}
	return result, nil
}
