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
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
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

func (e *Endpoint) putDashboardEndpoint(w http.ResponseWriter, req *http.Request) {
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
	json.NewEncoder(w).Encode(getDashboard(vars["id"], getUserId(req)))
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

func (e *Endpoint) postDashboardEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var dashReq Dashboard
	err := decoder.Decode(&dashReq)
	if err != nil {
		fmt.Println("Could not decode Dashboard Request data." + err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(updateDashboard(dashReq, getUserId(req)))
}

func (e *Endpoint) getWidgetEndpoint(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(getWidget(vars["dashboardId"], vars["widgetId"], getUserId(req)))
}

func (e *Endpoint) postWidgetEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var widgetReq Widget
	err := decoder.Decode(&widgetReq)
	if err != nil {
		fmt.Println("Could not decode Widget Request data." + err.Error())
	}

	vars := mux.Vars(req)
	err = updateWidget(vars["dashboardId"], widgetReq, getUserId(req))
	if err != nil {
		http.Error(w, "Error while updating widget: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(Response{"OK"})
}

func (e *Endpoint) putWidgetEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var widgetReq Widget
	err := decoder.Decode(&widgetReq)
	if err != nil {
		fmt.Println("Could not decode Widget Request data." + err.Error())
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
