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
	"reflect"
	"strconv"
	"strings"
)

func parseBoolean(keyname string, v interface{}) (bool, error) {
	switch t := v.(type) {
	case bool:
		return bool(t), nil
	case string:
		sv := v.(string)
		return strings.ToLower(sv) == "true", nil
	default:
		return false, fmt.Errorf("error parsing %s option %v", keyname, v)
	}
}

func parseInt(keyname string, v interface{}) (int64, error) {
	i := 0
	var err error
	switch v := v.(type) {
	case int:
		i = int(v)
	case int8:
		i = int(v)
	case int16:
		i = int(v)
	case int32:
		i = int(v)
	case int64:
		i = int(v)
	case string:
		i, err = strconv.Atoi(string(v))
		if err != nil {
			err = fmt.Errorf("unable to parse integer %v for key %s", v, keyname)
		}
	default:
		err = fmt.Errorf("unable to parse integer %v for key %s", v, keyname)
	}
	return int64(i), err
}

func parseFloat(keyname string, v interface{}) (float64, error) {
	var err error
	i := 0.0
	switch v := v.(type) {
	case float32:
		i = float64(v)
	case float64:
		i = float64(v)
	case string:
		i, err = strconv.ParseFloat(string(v), 64)
		if err != nil {
			err = fmt.Errorf("unable to parse float %v for key %s", v, keyname)
		}
	default:
		err = fmt.Errorf("unable to parse float %v for key %s", v, keyname)
	}
	return i, err
}

func parseString(keyName string, v interface{}) (string, error) {
	sv, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("error parsing %s option %v", keyName, v)
	}
	return sv, nil
}

func parsePrimitiveArray(keyName string, t reflect.Type, v interface{}) (reflect.Value, error) {
	buf := reflect.MakeSlice(reflect.SliceOf(t), 0, 0)
	ia, iaok := v.([]interface{})

	if iaok {
		for _, e := range ia {
			sv := reflect.ValueOf(e)

			if sv.Type().ConvertibleTo(t) {
				buf = reflect.Append(buf, sv.Convert(t))
			} else {
				return buf, fmt.Errorf("error parsing %s option %v, contents are not convertable", keyName, v)
			}
		}
		return buf, nil
	}

	if reflect.TypeOf(v).ConvertibleTo(t) {
		sv := reflect.ValueOf(v)
		buf = reflect.Append(buf, sv.Convert(t))
		return buf, nil
	}

	return buf, fmt.Errorf("error parsing %s option %v, single element is not convertable", keyName, v)
}

func parseStructs(keyName string, t reflect.Type, v interface{}, strict bool) (reflect.Value, error) {
	buf := reflect.MakeSlice(reflect.SliceOf(t), 0, 0)
	ia, iaok := v.([]interface{})
	if iaok {
		for _, e := range ia {
			sv, sok := e.(map[string]interface{})
			if sok {
				val := reflect.New(t)
				err := parseStruct(sv, val, strict)
				if err != nil {
					return buf, err
				}
				buf = reflect.Append(buf, val.Elem())
			} else {
				return buf, fmt.Errorf("struct array contained invalid value")
			}
		}
		return buf, nil
	}

	sv, sok := v.(map[string]interface{})
	if sok {
		val := reflect.New(t)
		err := parseStruct(sv, val, strict)
		if err != nil {
			return buf, err
		}
		buf = reflect.Append(buf, val.Elem())
		return buf, nil
	}

	return buf, fmt.Errorf("error parsing %s option %v", keyName, v)
}

func get(data map[string]interface{}, key string, confTag string) interface{} {
	if len(key) == 0 || data == nil {
		return nil
	}

	if confTag != "" {
		val, ok := data[confTag]
		if ok {
			return val
		}
	}

	val, ok := data[key]

	if ok {
		return val
	}

	// try lower case
	key = strings.ToLower(key)

	val, ok = data[key]

	if ok {
		return val
	}

	// Worst case, lower case all the possibles
	for k, v := range data {
		if strings.ToLower(k) == key {
			return v
		}
	}

	return nil
}

func parseStruct(data map[string]interface{}, config interface{}, strict bool) error {
	var err error
	var fields reflect.Value
	var maybeFields reflect.Value

	mapStringInterfaceType := reflect.TypeOf(map[string]interface{}{})

	// Get all the fields in the config struct
	if reflect.TypeOf(config).ConvertibleTo(reflect.TypeOf(reflect.Value{})) {
		maybeFields = config.(reflect.Value)

	} else {
		maybeFields = reflect.ValueOf(config)
	}

	fields = reflect.Indirect(maybeFields)
	dataType := fields.Type()

	// Loop over the fields in the struct
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		// Skip what we can't write
		if !field.CanSet() {
			if strict {
				return fmt.Errorf("unsettable field in configuration struct %s", dataType.Field(i).Name)
			}
			continue
		}

		fieldType := dataType.Field(i)
		fieldName := fieldType.Name
		fieldTag := fieldType.Tag
		confTag := fieldTag.Get("conf")
		configVal := get(data, fieldName, confTag)

		if configVal == nil {
			if strict {
				return fmt.Errorf("missing field in configuration file %s", fieldName)
			}
			continue
		}

		switch field.Type().Kind() {
		case reflect.Bool:
			var v bool
			if configVal != nil {
				v, err = parseBoolean(fieldName, configVal)
				if err != nil {
					return err
				}
				field.SetBool(v)
			}
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			var v int64
			if configVal != nil {
				v, err = parseInt(fieldName, configVal)
				if err != nil {
					return err
				}
				field.SetInt(v)
			}
		case reflect.Float64, reflect.Float32:
			var v float64
			if configVal != nil {
				v, err = parseFloat(fieldName, configVal)
				if err != nil {
					return err
				}
				field.SetFloat(v)
			}
		case reflect.String:
			var v string
			if configVal != nil {
				v, err = parseString(fieldName, configVal)
				if err != nil {
					return err
				}
				field.SetString(v)
			}
		case reflect.Map:
			configData, ok := configVal.(map[string]interface{})
			if !ok {
				return fmt.Errorf("map field %s doesn't have a matching map in the config file", fieldName)
			}
			if !field.Type().AssignableTo(mapStringInterfaceType) {
				return fmt.Errorf("only map[string]interface{} fields are supported")
			}
			field.Set(reflect.ValueOf(configData))
		case reflect.Array, reflect.Slice:
			switch fieldType.Type.Elem().Kind() {
			case reflect.String, reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Float64, reflect.Float32:
				theArray, err := parsePrimitiveArray(fieldName, fieldType.Type.Elem(), configVal)
				if err != nil {
					return err
				}
				field.Set(theArray)
			case reflect.Struct:
				var structs reflect.Value
				structs, err = parseStructs(fieldName, fieldType.Type.Elem(), configVal, strict)
				if err != nil {
					return err
				}

				field.Set(structs)
			default:
				if strict {
					return fmt.Errorf("unknown field type in configuration %s, bool, int, float, string and arrays/structs of those are supported", fieldName)
				}
			}
		case reflect.Struct:
			if configVal != nil {
				configData, ok := configVal.(map[string]interface{})
				if !ok {
					return fmt.Errorf("struct field %s doesn't have a matching map in the config file", fieldName)
				}

				err = parseStruct(configData, field, strict)
				if err != nil {
					return err
				}
			}
		default:
			if strict {
				return fmt.Errorf("unknown field type in configuration %s, bool, int, float, string, hostport and arrays/structs of those are supported", fieldName)
			}
		}

	}

	return nil
}
