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
	"github.com/globalsign/mgo/bson"
)

type Response struct {
	Message string `json:"message,omitempty"`
}

type Dashboard struct {
	Id        bson.ObjectId `bson:"_id,omitempty" json:"id"`
	Name string `json:"name,omitempty"`
	UserId string `json:"user_id,omitempty"`
	RefreshTime uint16 `json:"refresh_time"`
	Widgets [] Widget `json:"widgets,omitempty"`
}

type Widget struct {
	Id bson.ObjectId `bson:"_id,omitempty" json:"id"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Properties interface{} `json:"properties,omitempty"`
}

func (this *Dashboard) GetWidget(id bson.ObjectId) (result Widget, err error) {
	for _, element:= range this.Widgets {
		if element.Id == id {
			return element, nil
		}
	}
	return result, errors.New("No widget with id:" + id.String())
}

func (this *Dashboard) updateWidget(widget Widget) (err error) {
	if !widget.Id.Valid() {
		return errors.New("widget id is not valid")
	}

	widgets := []Widget{}
	updated := false

	for _, element:= range this.Widgets {
		if element.Id == widget.Id {
			updated = true
			widgets = append(widgets, widget)
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

func (this *Dashboard) addWidget(widget Widget) (result Widget, err error) {

	if widget.Id.Valid() {
		return result, errors.New("widget id is not empty")
	}

	widget.Id = bson.NewObjectId()
	this.Widgets = append(this.Widgets, widget)

	return widget, nil
}

func (this *Dashboard) deleteWidget(widgetId string) (err error) {
	if len(widgetId) == 0 {
		return errors.New("widget id is empty")
	}

	widgets := []Widget{}
	deleted := false

	for _, element:= range this.Widgets {
		if element.Id == bson.ObjectIdHex(widgetId) {
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