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
	"fmt"
	"github.com/globalsign/mgo"
)

var DB *mgo.Session

func InitDB() {
	session, err := mgo.Dial(GetEnv("MONGO", "localhost:27017"))
	if err != nil {
		panic("database connect failed: " + err.Error())
	} else {
		fmt.Println("Successfully connected to DB!")
	}
	DB = session
	err = migrateDashboardIndices()
	if err != nil {
		panic("could not migrate dashboard indices: " + err.Error())
	}
}

func Mongo() *mgo.Collection {
	return DB.DB("dashboard").C("dashboards")

}

func CloseDB() {
	DB.Close()
}
