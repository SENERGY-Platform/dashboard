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

	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"log"
	"reflect"
	"fmt"
)

type Response struct {
	Message string `json:"message,omitempty"`
}

type Dashboard struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `json:"name,omitempty"`
	UserId      string             `json:"user_id,omitempty"`
	RefreshTime uint16             `json:"refresh_time"`
	Widgets     []Widget           `json:"widgets,omitempty"`
	Index       *uint16            `json:"index,omitempty"`
}

type Widget struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name       string             `json:"name,omitempty"`
	Type       string             `json:"type,omitempty"`
	Properties interface{}        `json:"properties,omitempty"`
}

type WidgetPosition struct {
	Id    primitive.ObjectID `json:"id"`
	Index *int             `json:"index"`
	DashboardOrigin string `json:"dashboardOrigin"`
	DashboardDestination string `json:"dashboardDestination"`
}

func (this *Dashboard) GetWidget(id primitive.ObjectID) (index int, result Widget, err error) {
	for index, element := range this.Widgets {
		if element.Id == id {
			return index, element, nil
		}
	}
	return 0, result, errors.New("No widget with id:" + id.String())
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
				return Widget{}, errors.New(fmt.Sprintf("Property %s not found", property))
			}
			currentValue = temp.Interface()
		} 
	} 

	return widget, nil
}

func (this *Dashboard) updateWidget(newValue interface{}, propertyToChange string, widgetId string) (err error) {
	log.Printf("Update widget property: %s to value: %s", propertyToChange, newValue)

	widgets := []Widget{}
	updated := false

	widgetObjectId, err := primitive.ObjectIDFromHex(widgetId)
	if err != nil {
		return
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
		return errors.New("widget id is not matching")
	}

	this.Widgets = widgets
	return nil
}

func (this *Dashboard) SwapWidgetPosition(widgetPosition WidgetPosition) (err error) {
	oldPosition, widget, err := this.GetWidget(widgetPosition.Id)
	if err != nil {
		return err
	}
	err = this.removeWidgetAt(oldPosition)
	if err != nil {
		return err
	}

	if widgetPosition.Index != nil {
		err = this.insertWidgetAt(*widgetPosition.Index, widget)
	} else {
		err = this.insertWidgetAt(len(this.Widgets), widget)
	}
	if err != nil {
		return err
	}

	return nil
}

func (this *Dashboard) NewIndexIsInValid(index int) bool {
	return index > len(this.Widgets)-1 || index < 0 // widget can also be appened -> index > len()
}

func (this *Dashboard) insertWidgetAt(index int, widget Widget) (err error) {
	if this.NewIndexIsInValid(index) { 
		return errors.New("Index out of bounds")
	}
	this.Widgets = insertAt[Widget](this.Widgets, widget, index)
	return nil 
}

func (this *Dashboard) removeWidgetAt(index int) (err error) {
	if this.NewIndexIsInValid(index) {
		return errors.New("Index out of bounds")
	}
	this.Widgets = removeAt[Widget](this.Widgets, index)
	return nil
}

func (this *Dashboard) addWidget(widget Widget) (result Widget, err error) {
	widget.Id = primitive.NewObjectID()
	this.Widgets = append(this.Widgets, widget)

	return widget, nil
}

func (this *Dashboard) deleteWidget(widgetId string) (err error) {
	if len(widgetId) == 0 {
		return errors.New("widget id is empty")
	}

	widgets := []Widget{}
	deleted := false

	for _, element := range this.Widgets {
		id, err := primitive.ObjectIDFromHex(widgetId)
		if err != nil {
			return err
		}
		if element.Id == id {
			deleted = true
		} else {
			widgets = append(widgets, element)
		}
	}

	if !deleted {
		return errors.New("widget id is not matching")
	}

	this.Widgets = widgets
	return nil
}
