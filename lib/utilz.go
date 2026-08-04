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

	"github.com/gin-gonic/gin"
)

func getUserId(c *gin.Context) (userId string) {
	userId = c.GetHeader("X-UserId")
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

func parseModifiedSince(c *gin.Context) *time.Time {
	str := c.GetHeader("If-Modified-Since")
	if len(str) == 0 {
		return nil
	}
	t, err := time.Parse(http.TimeFormat, str)
	if err != nil {
		return nil
	}
	return &t
}

func addCacheControlHeaders(c *gin.Context, t time.Time) {
	c.Header("Last-Modified", cacheValidator(t, time.Now()).Format(http.TimeFormat))
	c.Header("Cache-Control", "no-store")
}

// cacheValidator picks the Last-Modified value handed to the client. Last-Modified
// carries whole seconds only, so a timestamp inside the second the response is
// built in cannot tell "unchanged" apart from "changed again a few milliseconds
// after this read": a client sending it back as If-Modified-Since would get a 304
// and keep serving a stale copy. Capping the validator at the previous full
// second costs a 304 for writes in the current second and keeps caching intact
// for everything older.
// The result is in UTC, because http.TimeFormat writes a literal GMT and would
// otherwise label a local timestamp as GMT.
func cacheValidator(lastModified time.Time, now time.Time) time.Time {
	lastModified = lastModified.UTC().Truncate(time.Second)
	newestCacheable := now.UTC().Truncate(time.Second).Add(-time.Second)
	if lastModified.After(newestCacheable) {
		return newestCacheable
	}
	return lastModified
}
