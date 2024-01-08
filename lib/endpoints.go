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
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

type Endpoint struct {
}

func NewEndpoint() *Endpoint {
	return &Endpoint{}
}

func (e *Endpoint) getRootEndpoint(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}

func (e *Endpoint) createDashboardEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var dashReq Dashboard
	err := decoder.Decode(&dashReq)
	if err != nil {
		http.Error(w, "Could not decode Dashboard Request data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	result, err := createDashboard(dashReq, getUserId(req))
	if err != nil {
		http.Error(w, "Error while creating dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(result)
}

func (e *Endpoint) getDashboardEndpoint(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	dashboard, err := getDashboard(vars["id"], getUserId(req))
	if err != nil {
		w.WriteHeader(404)
	}
	json.NewEncoder(w).Encode(dashboard)
}

func (e *Endpoint) getDashboardsEndpoint(w http.ResponseWriter, req *http.Request) {
	dashboards, err := getDashboards(getUserId(req))
	if err != nil {
		http.Error(w, "Error while reading dashboards: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(&dashboards)
}

func (e *Endpoint) deleteDashboardEndpoint(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(deleteDashboard(vars["id"], getUserId(req)))
}

func (e *Endpoint) editDashboardEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var dashReq Dashboard
	err := decoder.Decode(&dashReq)
	if err != nil {
		fmt.Println("Could not decode Dashboard Request data." + err.Error())
	}

	vars := mux.Vars(req)
	dashboardId := vars["id"]
	userId := getUserId(req)
	oldDashboard, err := getDashboard(dashboardId, userId)
	dashReq.Widgets = oldDashboard.Widgets

	dash, err := updateDashboard(dashReq, dashboardId, userId, context.TODO())
	if err != nil {
		http.Error(w, "Error while updating dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(dash)
}

func (e *Endpoint) getWidgetEndpoint(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	widget := getWidget(vars["dashboardId"], vars["widgetId"], getUserId(req))
	json.NewEncoder(w).Encode(widget)
}

func (e *Endpoint) editSingleWidgetPropertyEndpoint(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Error while reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var newValue interface{} 
	err = json.Unmarshal(payload, &newValue)
	if err != nil {
		newValue = string(payload)
	}
	
	vars := mux.Vars(req)
	err = updateWidget(vars["dashboardId"], newValue, vars["property"], vars["widgetId"], getUserId(req))
	if err != nil {
		http.Error(w, "Error while updating widget: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}

func (e *Endpoint) editWidgetPropertyEndpoint(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Error while reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var newValue interface{} 
	err = json.Unmarshal(payload, &newValue)
	if err != nil {
		newValue = string(payload)
	}
	
	vars := mux.Vars(req)
	err = updateWidget(vars["dashboardId"], newValue, "properties", vars["widgetId"], getUserId(req))
	if err != nil {
		http.Error(w, "Error while updating widget: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}

func (e *Endpoint) editWidgetNameEndpoint(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Error while reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newValue := string(payload)
	vars := mux.Vars(req)
	err = updateWidget(vars["dashboardId"], newValue, "name", vars["widgetId"], getUserId(req))
	if err != nil {
		http.Error(w, "Error while updating widget name: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}

func (e *Endpoint) editWidgetPosition(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var widgetReq []WidgetPosition
	err := decoder.Decode(&widgetReq)
	if err != nil {
		fmt.Println("Could not decode Widget Position Request data." + err.Error())
	}

	vars := mux.Vars(req)
	err = updateWidgetPositions(vars["dashboardId"], widgetReq, getUserId(req))
	if err != nil {
		http.Error(w, "Error while updating widget position: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}

func (e *Endpoint) createWidgetEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var widgetReq Widget
	err := decoder.Decode(&widgetReq)
	if err != nil {
		fmt.Println("Could not decode Widget Request data." + err.Error())
		http.Error(w, "Error while decoding widget data: "+err.Error(), http.StatusInternalServerError)
		return 
	}

	vars := mux.Vars(req)
	result, err := createWidget(vars["dashboardId"], widgetReq, getUserId(req))
	if err != nil {
		http.Error(w, "Error while creating widget: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(result)
}

func (e *Endpoint) deleteWidgetEndpoint(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	err := deleteWidget(vars["dashboardId"], vars["widgetId"], getUserId(req))
	if err != nil {
		http.Error(w, "Error while updating widget: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}
