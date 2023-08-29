package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDashboard_updateWidgetPositions(t *testing.T) {
	id0 := primitive.NewObjectID()
	widget0 := Widget{
		Id:         id0,
		Name:       "0",
		Type:       "",
		Properties: nil,
	}

	id1 := primitive.NewObjectID()
	widget1 := Widget{
		Id:         id1,
		Name:       "1",
		Type:       "",
		Properties: nil,
	}

	dashboard := &Dashboard{
		Id:          primitive.NewObjectID(),
		Name:        "",
		UserId:      "",
		RefreshTime: 1,
		Widgets:     []Widget{widget0, widget1},
	}

	expectedWidgets := []Widget{widget1, widget0}

	widgetPositions := []WidgetPosition{
		{Index: 1, Id: id0},
	}
	dashboard.updateWidgetPositions(widgetPositions)

	assert.Equal(t, expectedWidgets, dashboard.Widgets)
}
