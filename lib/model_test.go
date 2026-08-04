/*
 *    Copyright 2026 InfAI (CC SES)
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package lib

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestDashboardLayoutModeDecode(t *testing.T) {
	t.Run("absent stays nil", func(t *testing.T) {
		dash := Dashboard{}
		if err := json.Unmarshal([]byte(`{"name":"a","widgets":[]}`), &dash); err != nil {
			t.Fatal(err)
		}
		if dash.LayoutMode != nil {
			t.Errorf("expected nil layout mode for absent key, got %q", *dash.LayoutMode)
		}
	})

	t.Run("explicit value is kept", func(t *testing.T) {
		dash := Dashboard{}
		if err := json.Unmarshal([]byte(`{"name":"a","layout_mode":"moveScale"}`), &dash); err != nil {
			t.Fatal(err)
		}
		if dash.LayoutMode == nil || *dash.LayoutMode != LayoutModeMoveScale {
			t.Errorf("expected %q, got %v", LayoutModeMoveScale, dash.LayoutMode)
		}
	})
}

func TestDashboardColumnsDecode(t *testing.T) {
	t.Run("absent stays nil", func(t *testing.T) {
		dash := Dashboard{}
		if err := json.Unmarshal([]byte(`{"name":"a","widgets":[]}`), &dash); err != nil {
			t.Fatal(err)
		}
		if dash.Columns != nil {
			t.Errorf("expected nil columns for absent key, got %d", *dash.Columns)
		}
	})

	t.Run("explicit value is kept", func(t *testing.T) {
		dash := Dashboard{}
		if err := json.Unmarshal([]byte(`{"name":"a","columns":6}`), &dash); err != nil {
			t.Fatal(err)
		}
		if dash.Columns == nil || *dash.Columns != 6 {
			t.Errorf("expected 6, got %v", dash.Columns)
		}
	})

	// the default is zero, so this is the case a plain int field would lose
	t.Run("an explicit zero is not the same as absent", func(t *testing.T) {
		dash := Dashboard{}
		if err := json.Unmarshal([]byte(`{"name":"a","columns":0}`), &dash); err != nil {
			t.Fatal(err)
		}
		if dash.Columns == nil {
			t.Fatal("expected an explicit zero to be decoded, got nil")
		}
		if *dash.Columns != ColumnsAuto {
			t.Errorf("expected %d, got %d", ColumnsAuto, *dash.Columns)
		}
	})
}

func TestDashboardKeepUnset(t *testing.T) {
	stored := Dashboard{}
	stored.applyDefaults()
	*stored.LayoutMode = LayoutModeMoveScale
	*stored.Columns = 6

	t.Run("absent fields are taken from the stored dashboard", func(t *testing.T) {
		update := Dashboard{Name: "renamed"}
		update.keepUnset(stored)
		if update.LayoutMode == nil || *update.LayoutMode != LayoutModeMoveScale {
			t.Errorf("expected %q, got %v", LayoutModeMoveScale, update.LayoutMode)
		}
		if update.Columns == nil || *update.Columns != 6 {
			t.Errorf("expected 6, got %v", update.Columns)
		}
	})

	t.Run("sent fields win, including a zero", func(t *testing.T) {
		mode := LayoutModeList
		columns := ColumnsAuto
		update := Dashboard{LayoutMode: &mode, Columns: &columns}
		update.keepUnset(stored)
		if *update.LayoutMode != LayoutModeList {
			t.Errorf("expected %q, got %q", LayoutModeList, *update.LayoutMode)
		}
		if *update.Columns != ColumnsAuto {
			t.Errorf("expected %d, got %d", ColumnsAuto, *update.Columns)
		}
	})

	t.Run("the result does not alias the stored dashboard", func(t *testing.T) {
		update := Dashboard{}
		update.keepUnset(stored)
		*update.LayoutMode = LayoutModeNone
		*update.Columns = 12
		if *stored.LayoutMode != LayoutModeMoveScale || *stored.Columns != 6 {
			t.Fatalf("writing to the update changed the stored dashboard: %q, %d", *stored.LayoutMode, *stored.Columns)
		}
	})

	t.Run("nil stored values stay nil", func(t *testing.T) {
		update := Dashboard{}
		update.keepUnset(Dashboard{})
		if update.LayoutMode != nil || update.Columns != nil {
			t.Errorf("expected nils, got %v and %v", update.LayoutMode, update.Columns)
		}
	})
}

func TestDashboardApplyDefaults(t *testing.T) {
	t.Run("nil becomes the default", func(t *testing.T) {
		dash := Dashboard{}
		dash.applyDefaults()
		if dash.LayoutMode == nil || *dash.LayoutMode != defaultLayoutMode() {
			t.Fatalf("expected %q, got %v", defaultLayoutMode(), dash.LayoutMode)
		}
	})

	t.Run("nil columns becomes the default", func(t *testing.T) {
		dash := Dashboard{}
		dash.applyDefaults()
		if dash.Columns == nil || *dash.Columns != defaultColumns() {
			t.Fatalf("expected %d, got %v", defaultColumns(), dash.Columns)
		}
	})

	t.Run("the defaults are accepted values", func(t *testing.T) {
		if !defaultLayoutMode().valid() {
			t.Errorf("%q is not in %s", defaultLayoutMode(), layoutModeNames())
		}
		dash := Dashboard{}
		dash.applyDefaults()
		if err := dash.validate(); err != nil {
			t.Errorf("a dashboard holding only defaults does not validate: %v", err)
		}
	})

	t.Run("an existing value is not overwritten", func(t *testing.T) {
		mode := LayoutModeNone
		dash := Dashboard{LayoutMode: &mode}
		dash.applyDefaults()
		if *dash.LayoutMode != LayoutModeNone {
			t.Fatalf("expected %q, got %q", LayoutModeNone, *dash.LayoutMode)
		}
	})

	t.Run("marshals as a value, never as null", func(t *testing.T) {
		dash := Dashboard{}
		dash.applyDefaults()
		encoded, err := json.Marshal(dash)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]interface{}
		if err = json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["layout_mode"] != string(defaultLayoutMode()) {
			t.Fatalf("expected %q, got %v in %s", defaultLayoutMode(), decoded["layout_mode"], encoded)
		}
		if decoded["columns"] != float64(defaultColumns()) {
			t.Fatalf("expected %d, got %v in %s", defaultColumns(), decoded["columns"], encoded)
		}
	})
}

func TestDashboardValidateLayoutMode(t *testing.T) {
	cases := map[string]struct {
		layoutMode *LayoutMode
		valid      bool
	}{
		"absent":  {layoutMode: nil, valid: true},
		"unknown": {layoutMode: layoutModePtr("nonsense"), valid: false},
		"empty":   {layoutMode: layoutModePtr(""), valid: false},
		"casing":  {layoutMode: layoutModePtr("MoveScale"), valid: false},
		// a name that only looks like one of the accepted values
		"prefix": {layoutMode: layoutModePtr("move-scale"), valid: false},
	}
	for _, mode := range layoutModes {
		cases[string(mode)] = struct {
			layoutMode *LayoutMode
			valid      bool
		}{layoutMode: layoutModePtr(mode), valid: true}
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := (&Dashboard{LayoutMode: c.layoutMode}).validate()
			if c.valid {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("expected a bad request error, got %v", err)
			}
			if GetStatusCode(err) != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, GetStatusCode(err))
			}
			// the client has to be able to tell from the message what it may send
			if !strings.Contains(err.Error(), layoutModeNames()) {
				t.Fatalf("expected the error to list %s, got %v", layoutModeNames(), err)
			}
		})
	}
}

func TestDashboardValidateColumns(t *testing.T) {
	cases := map[string]struct {
		columns *int
		valid   bool
	}{
		"absent":         {columns: nil, valid: true},
		"auto":           {columns: columnsPtr(ColumnsAuto), valid: true},
		"one":            {columns: columnsPtr(1), valid: true},
		"max":            {columns: columnsPtr(MaxColumns), valid: true},
		"above max":      {columns: columnsPtr(MaxColumns + 1), valid: false},
		"far above max":  {columns: columnsPtr(100), valid: false},
		"negative":       {columns: columnsPtr(-1), valid: false},
		"far below zero": {columns: columnsPtr(-100), valid: false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := (&Dashboard{Columns: c.columns}).validate()
			if c.valid {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if GetStatusCode(err) != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, GetStatusCode(err))
			}
			if !strings.Contains(err.Error(), "columns") {
				t.Fatalf("expected the error to name the field, got %v", err)
			}
		})
	}

	t.Run("every value in the range is accepted", func(t *testing.T) {
		for columns := ColumnsAuto; columns <= MaxColumns; columns++ {
			if err := (&Dashboard{Columns: &columns}).validate(); err != nil {
				t.Errorf("columns %d: %v", columns, err)
			}
		}
	})
}

func columnsPtr(columns int) *int {
	return &columns
}

func layoutModePtr(mode LayoutMode) *LayoutMode {
	return &mode
}
