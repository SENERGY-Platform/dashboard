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

package main

import (
	"os"

	"github.com/SENERGY-Platform/dashboard/lib/log"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"

	"github.com/SENERGY-Platform/dashboard/lib"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Logger.Warn("Error loading .env file", attributes.ErrorKey, err)
	}

	mongoConfig := lib.MongoConfigFromEnv()
	if os.Getenv("SYNC") == "true" {
		if err = lib.Sync(mongoConfig); err != nil {
			log.Logger.Error("database sync failed", attributes.ErrorKey, err)
			os.Exit(1)
		}
	}

	if err = lib.InitDB(mongoConfig); err != nil {
		log.Logger.Error("database init failed", attributes.ErrorKey, err)
		os.Exit(1)
	}
	defer lib.CloseDB()
	lib.CreateServer()
}
