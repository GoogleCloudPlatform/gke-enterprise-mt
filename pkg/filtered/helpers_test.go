/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchValue(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		exists       bool
		filterValue  string
		allowMissing bool
		want         bool
	}{
		{
			name:         "Exact match",
			value:        "foo",
			exists:       true,
			filterValue:  "foo",
			allowMissing: false,
			want:         true,
		},
		{
			name:         "Mismatch",
			value:        "bar",
			exists:       true,
			filterValue:  "foo",
			allowMissing: false,
			want:         false,
		},
		{
			name:         "Missing but allowed",
			value:        "",
			exists:       false,
			filterValue:  "foo",
			allowMissing: true,
			want:         true,
		},
		{
			name:         "Missing and not allowed",
			value:        "",
			exists:       false,
			filterValue:  "foo",
			allowMissing: false,
			want:         false,
		},
		{
			name:         "Exact match with allowMissing true",
			value:        "foo",
			exists:       true,
			filterValue:  "foo",
			allowMissing: true,
			want:         true,
		},
		{
			name:         "Mismatch with allowMissing true",
			value:        "bar",
			exists:       true,
			filterValue:  "foo",
			allowMissing: true,
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchValue(tc.value, tc.exists, tc.filterValue, tc.allowMissing)
			assert.Equal(t, tc.want, got)
		})
	}
}
