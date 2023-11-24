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
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func CreateServer() {
	fmt.Println("Start Server")
	router := mux.NewRouter()
	e := NewEndpoint()
	router.HandleFunc("/", e.getRootEndpoint).Methods("GET")
	router.HandleFunc("/dashboards", e.getDashboardsEndpoint).Methods("GET")
	router.HandleFunc("/dashboards", e.createDashboardEndpoint).Methods("POST")
	router.HandleFunc("/dashboards/{id}", e.getDashboardEndpoint).Methods("GET")
	router.HandleFunc("/dashboards/{id}", e.deleteDashboardEndpoint).Methods("DELETE")
	router.HandleFunc("/dashboards/{id}", e.editDashboardEndpoint).Methods("PUT")

	router.HandleFunc("/widgets/positions/{dashboardId}", e.editWidgetPosition).Methods("PATCH")
	router.HandleFunc("/widgets/{dashboardId}/{widgetId}", e.getWidgetEndpoint).Methods("GET")
	router.HandleFunc("/widgets/{dashboardId}", e.createWidgetEndpoint).Methods("POST")
	router.HandleFunc("/widgets/{dashboardId}/{widgetId}", e.deleteWidgetEndpoint).Methods("DELETE")
	
	//router.HandleFunc("/widgets/{dashboardId}/{widgetId}", e.editWidgetEndpoint).Methods("PUT")
	router.HandleFunc("/widgets/{property}/{dashboardId}/{widgetId}", e.editWidgetEndpoint).Methods("PATCH")

	c := cors.New(
		cors.Options{
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "OPTIONS", "PATCH"},
		})
	handler := c.Handler(router)
	logger := NewLogger(handler)
	log.Fatal(http.ListenAndServe(":8080", logger))
}
