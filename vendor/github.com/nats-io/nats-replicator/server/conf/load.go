/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package conf

import (
	"fmt"
	"io/ioutil"

	"github.com/nats-io/nats-server/v2/conf"
)

// LoadConfigFromFile - given a struct, load a config from a file and fill in the struct
// If strict is true, all of the fields in the config struct must be in the file
// otherwise, the fields in the config struct will act as defaults if the file doesn't contain them
// Strict will also force an error if the struct contains any fields which are not settable with reflection
func LoadConfigFromFile(configFile string, configStruct interface{}, strict bool) error {
	configString, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading configuration file: %s", err.Error())
	}

	return LoadConfigFromString(string(configString), configStruct, strict)
}

// LoadConfigFromString - like LoadConfigFromFile but uses a string
func LoadConfigFromString(configString string, configStruct interface{}, strict bool) error {
	m, err := conf.Parse(string(configString))
	if err != nil {
		return err
	}

	return parseStruct(m, configStruct, strict)
}

// LoadConfigFromMap load a config struct from a map, this is useful if the type of a config isn't known at
// load time.
func LoadConfigFromMap(m map[string]interface{}, configStruct interface{}, strict bool) error {
	return parseStruct(m, configStruct, strict)
}
