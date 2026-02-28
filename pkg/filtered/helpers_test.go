/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const providerConfigLabelKey = "tenancy.gke.io/provider-config"

func TestIsObjectInProviderConfig(t *testing.T) {
	tests := []struct {
		name         string
		obj          interface{}
		filterKey    string
		filterValue  string
		allowMissing bool
		want         bool
	}{
		{
			name: "Matching Label",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						providerConfigLabelKey: "config1",
					},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         true,
		},
		{
			name: "Non-Matching Label",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						providerConfigLabelKey: "config2",
					},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         false,
		},
		{
			name: "Missing Label Allowed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: true,
			want:         true,
		},
		{
			name: "Missing Label Not Allowed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         false,
		},
		{
			name:         "Invalid Object",
			obj:          "not-an-object",
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isObjectMatchingValue(tc.obj, tc.filterKey, tc.filterValue, tc.allowMissing)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestProviderConfigFilteredList(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1",
			Labels: map[string]string{
				providerConfigLabelKey: "config1",
			},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod2",
			Labels: map[string]string{
				providerConfigLabelKey: "config2",
			},
		},
	}
	pod3 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod3",
			Labels: map[string]string{},
		},
	}

	tests := []struct {
		name         string
		items        []interface{}
		filterKey    string
		filterValue  string
		allowMissing bool
		want         []interface{}
	}{
		{
			name:         "Filter In Matching Only",
			items:        []interface{}{pod1, pod2, pod3},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         []interface{}{pod1},
		},
		{
			name:         "Filter In Matching and Missing",
			items:        []interface{}{pod1, pod2, pod3},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: true,
			want:         []interface{}{pod1, pod3},
		},
		{
			name:         "Filter In None",
			items:        []interface{}{pod2},
			filterKey:    providerConfigLabelKey,
			filterValue:  "config1",
			allowMissing: false,
			want:         nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getFilteredListByValue(tc.items, tc.filterKey, tc.filterValue, tc.allowMissing)
			assert.Equal(t, len(tc.want), len(got))
			for i, item := range got {
				assert.Equal(t, tc.want[i], item)
			}
		})
	}
}

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
