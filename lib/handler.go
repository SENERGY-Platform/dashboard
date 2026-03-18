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
	"net/http"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	gin_mw "github.com/SENERGY-Platform/gin-middleware"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -o ../docs --parseDependency -d .. -g lib/handler.go

// Start godoc
// @title Dashboard API
// @description Stores information about dashboards and their widgets.
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func CreateServer() {
	fmt.Println("Start Server")

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(
		gin_mw.StructLoggerHandlerWithDefaultGenerators(
			log.Logger.With(attributes.LogRecordTypeKey, attributes.HttpAccessLogRecordTypeVal),
			attributes.Provider,
			[]string{},
			nil,
		),
		requestid.New(requestid.WithCustomHeaderStrKey("X-Request-ID")),
		gin_mw.ErrorHandler(GetStatusCode, ", "),
		gin_mw.StructRecoveryHandler(log.Logger, gin_mw.DefaultRecoveryFunc),
	)

	router.GET("/", getRootEndpoint)
	router.GET("/doc", swaggerDocHandler)
	router.GET("/dashboards", getDashboardsEndpoint)
	router.POST("/dashboards", createDashboardEndpoint)
	router.GET("/dashboards/:id", getDashboardEndpoint)
	router.DELETE("/dashboards/:id", deleteDashboardEndpoint)
	router.PUT("/dashboards/:id", editDashboardEndpoint)

	router.PATCH("/widgets/positions", editWidgetPosition)
	router.GET("/widgets/:dashboardId/:widgetId", getWidgetEndpoint)
	router.POST("/widgets/:dashboardId", createWidgetEndpoint)
	router.DELETE("/widgets/:dashboardId/:widgetId", deleteWidgetEndpoint)

	router.PATCH("/widgets/name/:dashboardId/:widgetId", editWidgetNameEndpoint)
	router.PATCH("/widgets/properties/:dashboardId/:widgetId", editWidgetPropertyEndpoint)
	router.PATCH("/widgets/properties/:property/:dashboardId/:widgetId", editSingleWidgetPropertyEndpoint)

	log.Logger.Info("listen on port", "port", "8080")
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Logger.Error("listen and serve failed", attributes.ErrorKey, err)
		panic(err)
	}
}
