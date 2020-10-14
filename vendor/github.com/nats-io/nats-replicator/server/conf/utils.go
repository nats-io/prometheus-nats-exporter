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
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// validatePathExists checks that the provided path exists and is a dir if requested
func validatePathExists(path string, dir bool) (string, error) {
	if path == "" {
		return "", errors.New("path is not specified")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error parsing path [%s]: %v", abs, err)
	}

	var finfo os.FileInfo
	if finfo, err = os.Stat(abs); os.IsNotExist(err) {
		return "", fmt.Errorf("the path [%s] doesn't exist", abs)
	}

	mode := finfo.Mode()
	if dir && mode.IsRegular() {
		return "", fmt.Errorf("the path [%s] is not a directory", abs)
	}

	if !dir && mode.IsDir() {
		return "", fmt.Errorf("the path [%s] is not a file", abs)
	}

	return abs, nil
}

// ValidateDirPath checks that the provided path exists and is a dir
func ValidateDirPath(path string) (string, error) {
	return validatePathExists(path, true)
}

// ValidateFilePath checks that the provided path exists and is not a dir
func ValidateFilePath(path string) (string, error) {
	return validatePathExists(path, false)
}
