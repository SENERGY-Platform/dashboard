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
	"net/http"
	"strings"
	"time"
)

func getUserId(r *http.Request) (userId string) {
	userId = r.Header.Get("X-UserId")
	if userId == "" {
		userId = "testUser"
	}
	strings.Replace(userId, "\"", "", -1)
	return
}

func removeAt[T any](list []T, index int) []T {
	return append(list[:index], list[index+1:]...)
}

func insertAt[T any](list []T, value T, index int) []T {
	listWithValue := append([]T{value}, list[index:]...)
	return append(list[:index], listWithValue...)
}

func parseModifiedSince(req *http.Request) *time.Time {
	str := req.Header.Get("If-Modified-Since")
	if len(str) == 0 {
		return nil
	}
	t, err := time.Parse(http.TimeFormat, str)
	if err != nil {
		return nil
	}
	return &t
}

func addCacheControlHeaders(w http.ResponseWriter, t time.Time) {
	w.Header().Add("Last-Modified", t.Format(http.TimeFormat))
	w.Header().Add("Cache-Control", "no-store")
}
