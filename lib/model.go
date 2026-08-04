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
	"time"

	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Response struct {
	Message string `json:"message,omitempty"`
}

// LayoutMode describes how a dashboard lays out its widgets. The service only
// stores and returns the value, the web ui is the only component acting on it.
type LayoutMode string

const (
	LayoutModeCompact   LayoutMode = "compact"
	LayoutModeList      LayoutMode = "list"
	LayoutModeMoveScale LayoutMode = "moveScale"
	LayoutModeMove      LayoutMode = "move"
	LayoutModeScale     LayoutMode = "scale"
	LayoutModeNone      LayoutMode = "none"
)

// layoutModes lists every accepted value once, so validation and the error
// message telling a client what it may send cannot drift apart.
var layoutModes = []LayoutMode{LayoutModeCompact, LayoutModeList, LayoutModeMoveScale, LayoutModeMove, LayoutModeScale, LayoutModeNone}

// defaultLayoutMode is what a dashboard without a stored layout_mode reports and
// what the migration writes into documents predating the field.
func defaultLayoutMode() LayoutMode {
	return LayoutModeCompact
}

func (this LayoutMode) valid() bool {
	return slices.Contains(layoutModes, this)
}

func layoutModeNames() string {
	names := make([]string, 0, len(layoutModes))
	for _, mode := range layoutModes {
		names = append(names, string(mode))
	}
	return strings.Join(names, ", ")
}

const (
	// ColumnsAuto lets the web ui pick the number of grid columns itself.
	ColumnsAuto = 0
	// MaxColumns is the widest grid a dashboard may ask for.
	MaxColumns = 12
)

// defaultColumns is what a dashboard without a stored columns value reports and
// what the migration writes into documents predating the field.
func defaultColumns() int {
	return ColumnsAuto
}

type Dashboard struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `json:"name,omitempty"`
	UserId      string             `json:"user_id,omitempty"`
	RefreshTime uint16             `json:"refresh_time"`
	Widgets     []Widget           `json:"widgets"`
	Index       *uint16            `json:"index,omitempty"`
	// LayoutMode selects how the web ui lays out the widgets of this dashboard.
	// An update that omits the field keeps the stored value.
	LayoutMode *LayoutMode `bson:"layout_mode,omitempty" json:"layout_mode" enums:"compact,list,moveScale,move,scale,none"`
	// Columns is the number of grid columns the web ui uses, 0 letting it pick the
	// number itself. An update that omits the field keeps the stored value.
	Columns   *int      `bson:"columns,omitempty" json:"columns" minimum:"0" maximum:"12"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt,omitempty"`
}

// applyDefaults fills fields that dashboards written before those fields existed
// do not carry, so a client never sees a missing or null value. Those fields are
// pointers so that a nil can mean "not sent by the client" while a request is
// being decoded, which is what keepUnset relies on.
func (this *Dashboard) applyDefaults() {
	if this.LayoutMode == nil {
		mode := defaultLayoutMode()
		this.LayoutMode = &mode
	}
	if this.Columns == nil {
		columns := defaultColumns()
		this.Columns = &columns
	}
}

// keepUnset takes the fields the client did not send from the stored dashboard.
// Clients send the whole dashboard object on update and older ones do not know
// these fields at all, so an absent field has to leave the stored value alone
// instead of resetting the user's setting to the default.
// The values are copied rather than the pointers, so the result does not alias
// the dashboard read from the database.
func (this *Dashboard) keepUnset(stored Dashboard) {
	if this.LayoutMode == nil && stored.LayoutMode != nil {
		mode := *stored.LayoutMode
		this.LayoutMode = &mode
	}
	if this.Columns == nil && stored.Columns != nil {
		columns := *stored.Columns
		this.Columns = &columns
	}
}

// validate rejects unknown values instead of storing them verbatim or silently
// coercing them to the default, so a typo stays visible.
func (this *Dashboard) validate() error {
	if this.LayoutMode != nil && !this.LayoutMode.valid() {
		return errors.Join(ErrBadRequest, fmt.Errorf("invalid layout_mode %q, expected one of %s", string(*this.LayoutMode), layoutModeNames()))
	}
	if this.Columns != nil && (*this.Columns < ColumnsAuto || *this.Columns > MaxColumns) {
		return errors.Join(ErrBadRequest, fmt.Errorf("invalid columns %d, expected %d for automatic or 1 to %d", *this.Columns, ColumnsAuto, MaxColumns))
	}
	return nil
}

type Widget struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	X          *int               `json:"x,omitempty"`
	Y          *int               `json:"y,omitempty"`
	W          *int               `json:"w,omitempty"`
	H          *int               `json:"h,omitempty"`
	Name       string             `json:"name,omitempty"`
	Type       string             `json:"type,omitempty"`
	Properties interface{}        `json:"properties,omitempty"`
}

type WidgetPosition struct {
	Id                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	X                    *int               `json:"x,omitempty"`
	Y                    *int               `json:"y,omitempty"`
	W                    *int               `json:"w,omitempty"`
	H                    *int               `json:"h,omitempty"`
	DashboardOrigin      string             `json:"dashboardOrigin"`
	DashboardDestination string             `json:"dashboardDestination"`
}

func (this *Dashboard) GetWidget(id primitive.ObjectID) (index int, result Widget, err error) {
	for index, element := range this.Widgets {
		if element.Id == id {
			return index, element, nil
		}
	}
	return 0, result, errors.Join(ErrNotFound, errors.New("No widget with id:"+id.String()))
}

func updateWidgetProperty(widget Widget, propertyToChange string, newValue interface{}) (Widget, error) {
	propertyPath := strings.Split(propertyToChange, ".")
	i_last_prop := len(propertyPath) - 1
	var currentValue interface{}
	currentValue = widget.Properties

	for i, property := range propertyPath {
		val := reflect.ValueOf(currentValue)

		if val.Kind() == reflect.Map {
			if i == i_last_prop {
				val.SetMapIndex(reflect.ValueOf(property), reflect.ValueOf(newValue))
				break
			}

			temp := val.MapIndex(reflect.ValueOf(property)) // why interface?
			if !temp.IsValid() {
				return Widget{}, errors.Join(ErrNotFound, fmt.Errorf("Property %s not found", property))
			}
			currentValue = temp.Interface()
		}
	}

	return widget, nil
}

func (this *Dashboard) updateWidget(newValue interface{}, propertyToChange string, widgetId string) (err error) {
	log.Logger.Debug("update widget property",
		"property", propertyToChange,
		"value", newValue,
	)

	widgets := []Widget{}
	updated := false

	widgetObjectId, err := primitive.ObjectIDFromHex(widgetId)
	if err != nil {
		return errors.Join(ErrBadRequest, err)
	}

	for _, element := range this.Widgets {
		if element.Id == widgetObjectId {
			updated = true

			if propertyToChange == "name" {
				element.Name = newValue.(string)
				widgets = append(widgets, element)
				continue
			}

			if propertyToChange == "properties" {
				element.Properties = newValue
				widgets = append(widgets, element)
				continue
			}

			updatedWidget, err := updateWidgetProperty(element, propertyToChange, newValue)
			if err != nil {
				return err
			}
			widgets = append(widgets, updatedWidget)

		} else {
			widgets = append(widgets, element)
		}
	}

	if !updated {
		return errors.Join(ErrNotFound, errors.New("widget id is not matching"))
	}

	this.Widgets = widgets
	return nil
}

func (this *Dashboard) NewIndexIsInValid(index int) bool {
	return index > len(this.Widgets) || index < 0 // widget can also be appened -> index > len()
}

func (this *Dashboard) insertWidgetAt(index int, widget Widget) (err error) {
	if this.NewIndexIsInValid(index) {
		return errors.Join(ErrBadRequest, fmt.Errorf("Insert Index %d out of bounds", index))
	}
	this.Widgets = insertAt(this.Widgets, widget, index)
	return nil
}

func (this *Dashboard) removeWidgetAt(index int) (err error) {
	if this.NewIndexIsInValid(index) {
		return errors.Join(ErrBadRequest, fmt.Errorf("Removal Index %d out of bounds", index))
	}
	this.Widgets = removeAt(this.Widgets, index)
	return nil
}

func (this *Dashboard) addWidget(widget Widget) (result Widget, err error) {
	widget.Id = primitive.NewObjectID()
	this.Widgets = append(this.Widgets, widget)

	return widget, nil
}

func (this *Dashboard) deleteWidget(widgetId string) (err error) {
	if len(widgetId) == 0 {
		return errors.Join(ErrBadRequest, errors.New("widget id is empty"))
	}

	widgets := []Widget{}
	deleted := false

	for _, element := range this.Widgets {
		id, err := primitive.ObjectIDFromHex(widgetId)
		if err != nil {
			return errors.Join(ErrBadRequest, err)
		}
		if element.Id == id {
			deleted = true
		} else {
			widgets = append(widgets, element)
		}
	}

	if !deleted {
		return errors.Join(ErrNotFound, errors.New("widget id is not matching"))
	}

	this.Widgets = widgets
	return nil
}
