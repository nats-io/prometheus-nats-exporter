// Copyright 2017-2019 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseNATSUptime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "seconds only",
			input:    "30s",
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "minutes and seconds",
			input:    "45m20s",
			expected: 45*time.Minute + 20*time.Second,
			wantErr:  false,
		},
		{
			name:     "hours, minutes and seconds",
			input:    "3h15m30s",
			expected: 3*time.Hour + 15*time.Minute + 30*time.Second,
			wantErr:  false,
		},
		{
			name:     "days, hours, minutes and seconds",
			input:    "7d13h49m13s",
			expected: 7*24*time.Hour + 13*time.Hour + 49*time.Minute + 13*time.Second,
			wantErr:  false,
		},
		{
			name:     "years, days, hours, minutes and seconds",
			input:    "1y2d3h4m5s",
			expected: 365*24*time.Hour + 2*24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second,
			wantErr:  false,
		},
		{
			name:     "only days",
			input:    "5d",
			expected: 5 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "only hours",
			input:    "12h",
			expected: 12 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "no units",
			input:    "123",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseNATSUptime(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "expected error for input: %s", tt.input)
			} else {
				assert.NoError(t, err, "unexpected error for input: %s", tt.input)
				assert.Equal(t, tt.expected, result, "duration mismatch for input: %s", tt.input)
			}
		})
	}
}
