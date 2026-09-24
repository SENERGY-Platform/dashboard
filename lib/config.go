/*
 * Copyright 2026 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lib

import (
	"encoding/json"
	"log/slog"
	"os"
)

// MongoConfig holds the connection settings. When User is set, User, Password and
// AuthSource replace any credentials, authSource and authMechanism given in Url.
type MongoConfig struct {
	Url        string
	User       string
	Password   secret
	AuthSource string
	Database   string
}

// MongoConfigFromEnv reads MONGO_URL, MONGO_USER, MONGO_PASSWORD, MONGO_AUTH_SOURCE and
// MONGO_DATABASE; like GetEnv, an empty variable counts as unset.
func MongoConfigFromEnv() MongoConfig {
	return MongoConfig{
		Url:        GetEnv("MONGO_URL", "mongodb://localhost:27017"),
		User:       os.Getenv("MONGO_USER"),
		Password:   secret(os.Getenv("MONGO_PASSWORD")),
		AuthSource: GetEnv("MONGO_AUTH_SOURCE", "admin"),
		Database:   GetEnv("MONGO_DATABASE", "dashboard"),
	}
}

// secret masks its value wherever it is formatted, marshalled or logged; Value returns it.
type secret string

const secretMask = "***"

func (s secret) Value() string { return string(s) }

func (s secret) String() string { return secretMask }

func (s secret) GoString() string { return secretMask }

func (s secret) LogValue() slog.Value { return slog.StringValue(secretMask) }

func (s secret) MarshalJSON() ([]byte, error) { return json.Marshal(secretMask) }
