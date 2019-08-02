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
)

func createDashboard(dash Dashboard, userId string) (result Dashboard, err error) {
	dash.Id = bson.NewObjectId()
	dash.UserId = userId
	err = Mongo().Insert(dash)
	if err != nil{
		fmt.Println("Error create:", err)
		return result, err
	}
	return dash, nil
}

func getDashboard(id string, userId string) (dash Dashboard){
	err := Mongo().Find(bson.M{"_id": bson.ObjectIdHex(id), "userid": userId}).One(&dash)
	if err != nil {
		fmt.Println("Error find:", err)
	}
	return
}

func getDashboards(userId string)(dashs [] Dashboard){
	Mongo().Find(bson.M{"userid": userId}).All(&dashs)
	return
}

func deleteDashboard(id string, userId string) Response{
	err := Mongo().Remove(bson.M{"_id": bson.ObjectIdHex(id), "userid": userId})
	if err != nil {
		fmt.Println("Error remove:", err)
	}
	return Response{"ok"}
}

func updateDashboard(dash Dashboard, userId string) Dashboard{
	for index, widget := range dash.Widgets{
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

func getWidget(dashboardId string, widgetId string,userId string) (widget Widget){
	dash:= Dashboard{}
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

func createWidget(dashboardId string, widget Widget, userId string) (result Widget, err error){
	dash := getDashboard(dashboardId, userId)
	widgetResult, err := dash.addWidget(widget)
	if err != nil {
		fmt.Println("Error createWidget: ", err)
		return result, err
	}
	updateDashboard(dash, userId)

	return widgetResult, nil
}

func updateWidget(dashboardId string, widget Widget, userId string) (err error){
	dash := getDashboard(dashboardId, userId)
	err = dash.updateWidget(widget)
	if err != nil {
		fmt.Println("Error updateWidget: ", err)
		return err
	}
	updateDashboard(dash, userId)

	return nil
}

func deleteWidget(dashboardId string, widgetId string, userId string) (err error){
	dash := getDashboard(dashboardId, userId)
	err = dash.deleteWidget(widgetId)
	if err != nil {
		fmt.Println("Error deleteWidget: ", err)
		return err
	}
	updateDashboard(dash, userId)

	return nil
}