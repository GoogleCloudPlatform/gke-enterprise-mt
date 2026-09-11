package filteredinformerv2

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsObjectMatchingValue(t *testing.T) {
	testCases := []struct {
		desc               string
		providerConfigName string
		object             any
		allowMissing       bool
		expectedToMatch    bool
	}{
		{
			desc:               "Object with matching label should return true",
			providerConfigName: "p123456-abc",
			object:             &metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			allowMissing:       false,
			expectedToMatch:    true,
		},
		{
			desc:               "Object with different label should return false",
			providerConfigName: "p123456-abc",
			object:             &metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-def"}},
			allowMissing:       false,
			expectedToMatch:    false,
		},
		{
			desc:               "Object with no label and allowMissing false should return false",
			providerConfigName: "p123456-abc",
			object:             &metav1.ObjectMeta{Name: "obj3"},
			allowMissing:       false,
			expectedToMatch:    false,
		},
		{
			desc:               "Object with no label and allowMissing true should return true",
			providerConfigName: "p123456-abc",
			object:             &metav1.ObjectMeta{Name: "obj3"},
			allowMissing:       true,
			expectedToMatch:    true,
		},
		{
			desc:               "Invalid object should return false",
			providerConfigName: "p123456-abc",
			object:             "invalid-object",
			allowMissing:       true,
			expectedToMatch:    false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			result := isObjectMatchingValue(tc.object, providerConfigLabel, tc.providerConfigName, tc.allowMissing)
			if result != tc.expectedToMatch {
				t.Errorf("isObjectMatchingValue(%v, %q, %q, %t) = %v, want %v", tc.object, providerConfigLabel, tc.providerConfigName, tc.allowMissing, result, tc.expectedToMatch)
			}
		})
	}
}

func TestGetFilteredListByValue(t *testing.T) {
	testCases := []struct {
		desc               string
		providerConfigName string
		allowMissing       bool
		objects            []any
		expectedCount      int
	}{
		{
			desc:               "All objects match",
			providerConfigName: "p123456-abc",
			allowMissing:       false,
			objects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}, Name: "obj1"},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}, Name: "obj2"},
			},
			expectedCount: 2,
		},
		{
			desc:               "Objects without label included when allowMissing is true",
			providerConfigName: "p123456-abc",
			allowMissing:       true,
			objects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}, Name: "obj1"},
				&metav1.ObjectMeta{Name: "obj-unlabeled"},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "other"}, Name: "obj3"},
			},
			expectedCount: 2,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			result := getFilteredListByValue(tc.objects, providerConfigLabel, tc.providerConfigName, tc.allowMissing)
			if len(result) != tc.expectedCount {
				t.Errorf("getFilteredListByValue returned %d objects, want %d", len(result), tc.expectedCount)
			}
		})
	}
}

func TestMatchValue(t *testing.T) {
	tests := []struct {
		val          string
		ok           bool
		expectedVal  string
		allowMissing bool
		want         bool
	}{
		{val: "abc", ok: true, expectedVal: "abc", allowMissing: false, want: true},
		{val: "abc", ok: true, expectedVal: "def", allowMissing: false, want: false},
		{val: "", ok: false, expectedVal: "abc", allowMissing: false, want: false},
		{val: "", ok: false, expectedVal: "abc", allowMissing: true, want: true},
		{val: "abc", ok: true, expectedVal: "def", allowMissing: true, want: false},
	}

	for _, tt := range tests {
		got := MatchValue(tt.val, tt.ok, tt.expectedVal, tt.allowMissing)
		if got != tt.want {
			t.Errorf("MatchValue(%q, %t, %q, %t) = %t, want %t", tt.val, tt.ok, tt.expectedVal, tt.allowMissing, got, tt.want)
		}
	}
}
