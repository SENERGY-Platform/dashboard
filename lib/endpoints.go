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
	"errors"
	"net/http"
	"strings"
	"time"

	_ "github.com/SENERGY-Platform/dashboard/docs"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
)

type ErrorResponse string

// getRootEndpoint godoc
// @Summary Health check
// @Description Returns service availability.
// @Tags status
// @Produce json
// @Success 200 {object} Response
// @Router / [get]
func getRootEndpoint(c *gin.Context) {
	c.JSON(http.StatusOK, Response{"OK"})
}

// swaggerDocHandler godoc
// @Summary Get OpenAPI document
// @Description Returns the generated Swagger document for this service.
// @Tags documentation
// @Produce json
// @Success 200 {string} string
// @Failure 500 {string} ErrorResponse
// @Router /doc [get]
func swaggerDocHandler(c *gin.Context) {
	doc, err := swag.ReadDoc()
	if err != nil {
		_ = c.Error(errors.Join(err, ErrInternalServerError))
		return
	}
	// Remove empty host to let downstream tooling inject the correct target.
	doc = strings.Replace(doc, `"host": "",`, "", 1)
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(doc))
}

// createDashboardEndpoint godoc
// @Summary Create dashboard
// @Description Creates a new dashboard for the current user.
// @Tags dashboards
// @Accept json
// @Produce json
// @Param dashboard body Dashboard true "Dashboard payload"
// @Success 200 {object} Dashboard
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /dashboards [post]
func createDashboardEndpoint(c *gin.Context) {
	var dashReq Dashboard
	if err := c.ShouldBind(&dashReq); err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Could not decode Dashboard Request data"), err))
		return
	}
	result, err := createDashboard(c.Request.Context(), dashReq, getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while creating dashboard"), err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// getDashboardEndpoint godoc
// @Summary Get dashboard
// @Description Returns a dashboard by id.
// @Tags dashboards
// @Produce json
// @Param id path string true "Dashboard ID"
// @Success 200 {object} Dashboard
// @Success 304 {string} string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /dashboards/{id} [get]
func getDashboardEndpoint(c *gin.Context) {
	ctx := c.Request.Context()
	t := parseModifiedSince(c)
	modified, dashboard, err := getDashboard(t, c.Param("id"), getUserId(c), ctx)
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while reading dashboard"), err))
		return
	}
	if t != nil && !modified {
		c.Status(http.StatusNotModified)
		return
	}
	addCacheControlHeaders(c, dashboard.UpdatedAt)
	c.JSON(http.StatusOK, dashboard)
}

// getDashboardsEndpoint godoc
// @Summary List dashboards
// @Description Returns all dashboards for the current user.
// @Tags dashboards
// @Produce json
// @Success 200 {array} Dashboard
// @Success 304 {string} string
// @Failure 500 {object} ErrorResponse
// @Router /dashboards [get]
func getDashboardsEndpoint(c *gin.Context) {
	t := parseModifiedSince(c)
	modified, dashboards, err := getDashboards(c.Request.Context(), t, getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while reading dashboards"), err))
		return
	}
	if t != nil && !modified {
		c.Status(http.StatusNotModified)
		return
	}
	latest := time.Unix(0, 0)
	for _, dash := range dashboards {
		if dash.UpdatedAt.After(latest) {
			latest = dash.UpdatedAt
		}
	}
	latest = latest.Truncate(time.Second)
	addCacheControlHeaders(c, latest)
	c.JSON(http.StatusOK, &dashboards)
}

// deleteDashboardEndpoint godoc
// @Summary Delete dashboard
// @Description Deletes a dashboard by id.
// @Tags dashboards
// @Produce json
// @Param id path string true "Dashboard ID"
// @Success 200 {object} Response
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /dashboards/{id} [delete]
func deleteDashboardEndpoint(c *gin.Context) {
	result, err := deleteDashboard(c.Request.Context(), c.Param("id"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while deleting dashboard"), err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// editDashboardEndpoint godoc
// @Summary Update dashboard
// @Description Updates dashboard metadata by id.
// @Tags dashboards
// @Accept json
// @Produce json
// @Param id path string true "Dashboard ID"
// @Param dashboard body Dashboard true "Dashboard payload"
// @Success 200 {object} Dashboard
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /dashboards/{id} [put]
func editDashboardEndpoint(c *gin.Context) {
	var dashReq Dashboard
	if err := c.ShouldBind(&dashReq); err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Error while decoding dashboard"), err))
		return
	}

	dashboardId := c.Param("id")
	userId := getUserId(c)
	ctx := c.Request.Context()
	_, oldDashboard, err := getDashboard(nil, dashboardId, userId, ctx)
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while reading dashboard"), err))
		return
	}
	dashReq.Widgets = oldDashboard.Widgets

	dash, err := updateDashboard(dashReq, dashboardId, userId, ctx)
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while updating dashboard"), err))
		return
	}

	c.JSON(http.StatusOK, dash)
}

// getWidgetEndpoint godoc
// @Summary Get widget
// @Description Returns a widget by dashboard and widget id.
// @Tags widgets
// @Produce json
// @Param dashboardId path string true "Dashboard ID"
// @Param widgetId path string true "Widget ID"
// @Success 200 {object} Widget
// @Success 304 {string} string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/{dashboardId}/{widgetId} [get]
func getWidgetEndpoint(c *gin.Context) {
	t := parseModifiedSince(c)
	modified, lastModified, widget, err := getWidget(c.Request.Context(), t, c.Param("dashboardId"), c.Param("widgetId"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while reading widget"), err))
		return
	}
	if t != nil && !modified {
		c.Status(http.StatusNotModified)
		return
	}
	addCacheControlHeaders(c, *lastModified)
	c.JSON(http.StatusOK, widget)
}

// editSingleWidgetPropertyEndpoint godoc
// @Summary Update one widget property
// @Description Updates a single widget property by property key.
// @Tags widgets
// @Accept json
// @Produce json
// @Param property path string true "Property name"
// @Param dashboardId path string true "Dashboard ID"
// @Param widgetId path string true "Widget ID"
// @Param value body object true "New property value"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/properties/{property}/{dashboardId}/{widgetId} [patch]
func editSingleWidgetPropertyEndpoint(c *gin.Context) {
	var newValue interface{}
	err := c.ShouldBind(&newValue)
	if err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Error while reading request body"), err))
		return
	}

	err = updateWidget(c.Request.Context(), c.Param("dashboardId"), newValue, c.Param("property"), c.Param("widgetId"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while updating widget"), err))
		return
	}
	c.JSON(http.StatusOK, Response{"OK"})
}

// editWidgetPropertyEndpoint godoc
// @Summary Update widget properties
// @Description Updates the complete properties object of a widget.
// @Tags widgets
// @Accept json
// @Produce json
// @Param dashboardId path string true "Dashboard ID"
// @Param widgetId path string true "Widget ID"
// @Param properties body object true "New properties object"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/properties/{dashboardId}/{widgetId} [patch]
func editWidgetPropertyEndpoint(c *gin.Context) {
	var newValue interface{}
	err := c.ShouldBind(&newValue)
	if err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Error while reading request body"), err))
		return
	}

	err = updateWidget(c.Request.Context(), c.Param("dashboardId"), newValue, "properties", c.Param("widgetId"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while updating widget"), err))
		return
	}
	c.JSON(http.StatusOK, Response{"OK"})
}

// editWidgetNameEndpoint godoc
// @Summary Update widget name
// @Description Updates the name of a widget.
// @Tags widgets
// @Accept json
// @Produce json
// @Param dashboardId path string true "Dashboard ID"
// @Param widgetId path string true "Widget ID"
// @Param name body string true "New widget name"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/name/{dashboardId}/{widgetId} [patch]
func editWidgetNameEndpoint(c *gin.Context) {
	var name string
	err := c.ShouldBind(&name)
	if err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Error while reading request body"), err))
		return
	}
	err = updateWidget(c.Request.Context(), c.Param("dashboardId"), name, "name", c.Param("widgetId"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while updating widget name"), err))
		return
	}
	c.JSON(http.StatusOK, Response{"OK"})
}

// editWidgetPosition godoc
// @Summary Update widget positions
// @Description Updates positions for multiple widgets.
// @Tags widgets
// @Accept json
// @Produce json
// @Param positions body []WidgetPosition true "Widget position updates"
// @Success 200 {object} Response
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/positions [patch]
func editWidgetPosition(c *gin.Context) {
	var widgetReq []WidgetPosition
	if err := c.ShouldBind(&widgetReq); err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Could not decode Widget Position Request data"), err))
		return
	}

	err := updateWidgetPositions(c.Request.Context(), widgetReq, getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while updating widget position"), err))
		return
	}
	c.JSON(http.StatusOK, Response{"OK"})
}

// createWidgetEndpoint godoc
// @Summary Create widget
// @Description Creates a new widget in a dashboard.
// @Tags widgets
// @Accept json
// @Produce json
// @Param dashboardId path string true "Dashboard ID"
// @Param widget body Widget true "Widget payload"
// @Success 200 {object} Widget
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/{dashboardId} [post]
func createWidgetEndpoint(c *gin.Context) {
	var widgetReq Widget
	if err := c.ShouldBind(&widgetReq); err != nil {
		_ = c.Error(errors.Join(GetError(http.StatusBadRequest), errors.New("Error while decoding widget data"), err))
		return
	}

	result, err := createWidget(c.Request.Context(), c.Param("dashboardId"), widgetReq, getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while creating widget"), err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// deleteWidgetEndpoint godoc
// @Summary Delete widget
// @Description Deletes a widget by dashboard and widget id.
// @Tags widgets
// @Produce json
// @Param dashboardId path string true "Dashboard ID"
// @Param widgetId path string true "Widget ID"
// @Success 200 {object} Response
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /widgets/{dashboardId}/{widgetId} [delete]
func deleteWidgetEndpoint(c *gin.Context) {
	err := deleteWidget(c.Request.Context(), c.Param("dashboardId"), c.Param("widgetId"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while deleting widget"), err))
		return
	}
	c.JSON(http.StatusOK, Response{"OK"})
}
