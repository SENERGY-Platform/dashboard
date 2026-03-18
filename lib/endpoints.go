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
	"time"

	"github.com/gin-gonic/gin"
)

func getRootEndpoint(c *gin.Context) {
	c.JSON(http.StatusOK, Response{"OK"})
}

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
func deleteDashboardEndpoint(c *gin.Context) {
	result, err := deleteDashboard(c.Request.Context(), c.Param("id"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while deleting dashboard"), err))
		return
	}
	c.JSON(http.StatusOK, result)
}
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
func deleteWidgetEndpoint(c *gin.Context) {
	err := deleteWidget(c.Request.Context(), c.Param("dashboardId"), c.Param("widgetId"), getUserId(c))
	if err != nil {
		_ = c.Error(errors.Join(GetError(GetStatusCode(err)), errors.New("Error while deleting widget"), err))
		return
	}
	c.JSON(http.StatusOK, Response{"OK"})
}
