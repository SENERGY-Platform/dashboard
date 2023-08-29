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
	Index uint16             `json:"index"`
}

func (this *Dashboard) GetWidget(id primitive.ObjectID) (index int, result Widget, err error) {
	for index, element := range this.Widgets {
		if element.Id == id {
			return index, element, nil
		}
	}
	return 0, result, errors.New("No widget with id:" + id.String())
}

func (this *Dashboard) updateWidget(widget Widget) (err error) {
	widgets := []Widget{}
	updated := false

	for _, element := range this.Widgets {
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

func (this *Dashboard) updateWidgetPositions(widgetPositions []WidgetPosition) (err error) {
	// TODO range check
	for _, widgetPositionUpdate := range widgetPositions {
		oldPosition, widget, err := this.GetWidget(widgetPositionUpdate.Id)
		if err != nil {
			return err
		}

		this.Widgets = removeAt[Widget](this.Widgets, oldPosition)
		this.Widgets = insertAt[Widget](this.Widgets, widget, int(widgetPositionUpdate.Index))
	}

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
