package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDashboard_SwapWidgetPosition(t *testing.T) {
	dashboardID := primitive.NewObjectID()
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
	firstIndex := 1
	thirdIndex := 3

	tests := []struct {
		name    string
		dashboard  Dashboard
		widgetPosition    WidgetPosition
		wantErr bool
		expectedWidgets []Widget
	}{
		{
			name: "Swap",
			dashboard: Dashboard{
				Id:          dashboardID,
				Name:        "",
				UserId:      "",
				RefreshTime: 1,
				Widgets:     []Widget{widget0, widget1},
			},
			widgetPosition: WidgetPosition{Index: &firstIndex, Id: id0, DashboardOrigin: dashboardID.String(), DashboardDestination: dashboardID.String()},
			expectedWidgets: []Widget{widget1, widget0},
		},
		{
			name: "Insert out off bounds",
			dashboard: Dashboard{
				Id:          dashboardID,
				Name:        "",
				UserId:      "",
				RefreshTime: 1,
				Widgets:     []Widget{widget0, widget1},
			},
			expectedWidgets: []Widget{widget0, widget1},
			widgetPosition: WidgetPosition{Index: &thirdIndex, Id: id0, DashboardOrigin: dashboardID.String(), DashboardDestination: dashboardID.String()},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.dashboard.SwapWidgetPosition(tt.widgetPosition); tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.EqualValues(t, tt.expectedWidgets, tt.dashboard.Widgets)

		})
	}
}
