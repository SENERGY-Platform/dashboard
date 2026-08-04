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
	"testing"
	"time"
)

func TestCacheValidator(t *testing.T) {
	// Last-Modified has second resolution: a validator inside the current second
	// would hide a write happening later in that same second.
	base := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name         string
		lastModified time.Time
		now          time.Time
		expected     time.Time
	}{
		{
			name:         "older writes are handed out truncated",
			lastModified: base.Add(-5 * time.Second).Add(400 * time.Millisecond),
			now:          base,
			expected:     base.Add(-5 * time.Second),
		},
		{
			name:         "a write in the current second is capped",
			lastModified: base.Add(100 * time.Millisecond),
			now:          base.Add(300 * time.Millisecond),
			expected:     base.Add(-time.Second),
		},
		{
			name:         "a write in the previous second is safe",
			lastModified: base.Add(-time.Second).Add(900 * time.Millisecond),
			now:          base.Add(100 * time.Millisecond),
			expected:     base.Add(-time.Second),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cacheValidator(c.lastModified, c.now); !got.Equal(c.expected) {
				t.Fatalf("expected %s, got %s", c.expected, got)
			}
		})
	}

	t.Run("a later write is never hidden by the validator", func(t *testing.T) {
		// the comparison the read endpoints use is UpdatedAt.Truncate(s).After(ifModifiedSince)
		read := base.Add(300 * time.Millisecond)
		validator := cacheValidator(base.Add(100*time.Millisecond), read)
		for _, delay := range []time.Duration{time.Millisecond, 100 * time.Millisecond, 699 * time.Millisecond, time.Second} {
			write := read.Add(delay)
			if !write.Truncate(time.Second).After(validator) {
				t.Fatalf("write at %s would be reported as not modified against validator %s", write, validator)
			}
		}
	})
}
