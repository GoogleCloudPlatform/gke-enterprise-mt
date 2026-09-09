package filteredinformerv2

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIsObjectInProviderConfig(t *testing.T) {
	testCases := []struct {
		desc            string
		object          any
		expectedToMatch bool
	}{
		{
			desc:            "Object in provider config should return true",
			object:          &metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			expectedToMatch: true,
		},
		{
			desc:            "Object in different provider config should return false",
			object:          &metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-def"}},
			expectedToMatch: false,
		},
		{
			desc:            "Object with no provider config should return false",
			object:          &metav1.ObjectMeta{Name: "obj3"},
			expectedToMatch: false,
		},
		{
			desc:            "Invalid object should return false",
			object:          "invalid-object",
			expectedToMatch: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			providerConfigName := "p123456-abc"
			result := isObjectInProviderConfig(tc.object, providerConfigName)
			if result != tc.expectedToMatch {
				t.Errorf("isObjectInProviderConfig(%v, %q) = %v, want %v", tc.object, providerConfigName, result, tc.expectedToMatch)
			}
		})
	}
}

func TestProviderConfigFilteredList(t *testing.T) {
	testCases := []struct {
		desc            string
		objects         []any
		expectedObjects []any
	}{
		{
			desc: "All objects in the provider config",
			objects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			},
			expectedObjects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			},
		},
		{
			desc: "Some objects in the provider config",
			objects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-def"}},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			},
			expectedObjects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			},
		},
		{
			desc: "No objects in the provider config",
			objects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-def"}},
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p654321-def"}},
			},
			expectedObjects: []any{},
		},
		{
			desc: "Invalid objects in the list",
			objects: []any{
				"invalid-object",
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
				12345, // Non-object type
			},
			expectedObjects: []any{
				&metav1.ObjectMeta{Labels: map[string]string{providerConfigLabel: "p123456-abc"}},
			},
		},
		{
			desc:            "Empty object list",
			objects:         []any{},
			expectedObjects: []any{},
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			providerConfigName := "p123456-abc"
			result := providerConfigFilteredList(tc.objects, providerConfigName)

			if len(result) != len(tc.expectedObjects) {
				t.Errorf("providerConfigFilteredList(%v, %q) returned %d objects, want %d", tc.objects, providerConfigName, len(result), len(tc.expectedObjects))
			}

			for i, obj := range result {
				expectedObj := tc.expectedObjects[i]

				objMeta, err1 := metaAccessor(obj)
				expectedMeta, err2 := metaAccessor(expectedObj)

				if err1 != nil || err2 != nil {
					t.Errorf("Error accessing object metadata: %v, %v", err1, err2)
					continue
				}

				if objMeta.GetName() != expectedMeta.GetName() || objMeta.GetNamespace() != expectedMeta.GetNamespace() {
					t.Errorf("providerConfigFilteredList(%v, %q) returned object %v, want %v", tc.objects, providerConfigName, objMeta, expectedMeta)
				}
			}
		})
	}
}

// Helper function to access metadata
func metaAccessor(obj any) (metav1.Object, error) {
	if accessor, ok := obj.(metav1.Object); ok {
		return accessor, nil
	}
	if runtimeObj, ok := obj.(runtime.Object); ok {
		return meta.Accessor(runtimeObj)
	}
	return nil, fmt.Errorf("object does not have ObjectMeta")
}
