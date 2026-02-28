/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewFilteredSharedInformerFactory(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	filterKey := "test-key"
	filterValue := "test-value"
	allowMissing := true

	factory := NewFilteredSharedInformerFactory(parentFactory, filterKey, filterValue, allowMissing)

	assert.NotNil(t, factory)
	assert.Equal(t, filterKey, factory.filterKey)
	assert.Equal(t, filterValue, factory.filterValue)
	assert.Equal(t, allowMissing, factory.allowMissing)
	assert.Equal(t, parentFactory, factory.SharedInformerFactory)
}

func TestFilteredSharedInformerFactory_Core_Nodes(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	filterKey := "test-key"
	filterValue := "test-value"
	allowMissing := false
	factory := NewFilteredSharedInformerFactory(parentFactory, filterKey, filterValue, allowMissing)

	nodeInformer := factory.Core().V1().Nodes()
	assert.NotNil(t, nodeInformer)

	// Verify it's a filtered informer by checking the type or properties if possible.
	// Since FilteredNodeInformer is exported, we can check the type.
	_, ok := nodeInformer.(*FilteredNodeInformer)
	assert.True(t, ok, "Expected nodeInformer to be of type *FilteredNodeInformer")

	// Trigger Informer creation to check internal properties
	sharedInformer := nodeInformer.Informer()
	filteredInf, ok := sharedInformer.(*FilteredInformer)
	assert.True(t, ok, "Expected sharedInformer to be of type *FilteredInformer")
	if ok {
		assert.Equal(t, filterKey, filteredInf.filterKey)
		assert.Equal(t, filterValue, filteredInf.filterValue)
		assert.Equal(t, allowMissing, filteredInf.allowMissing)
	}
}

func TestFilteredSharedInformerFactory_Coordination_Leases(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	filterKey := "test-key"
	filterValue := "test-value"
	allowMissing := true
	factory := NewFilteredSharedInformerFactory(parentFactory, filterKey, filterValue, allowMissing)

	leaseInformer := factory.Coordination().V1().Leases()
	assert.NotNil(t, leaseInformer)

	// Verify it's a filtered informer
	_, ok := leaseInformer.(*FilteredLeaseInformer)
	assert.True(t, ok, "Expected leaseInformer to be of type *FilteredLeaseInformer")

	// Trigger Informer creation
	sharedInformer := leaseInformer.Informer()
	filteredInf, ok := sharedInformer.(*FilteredInformer)
	assert.True(t, ok, "Expected sharedInformer to be of type *FilteredInformer")
	if ok {
		assert.Equal(t, filterKey, filteredInf.filterKey)
		assert.Equal(t, filterValue, filteredInf.filterValue)
		assert.Equal(t, allowMissing, filteredInf.allowMissing)
	}
}

func TestFilteredSharedInformerFactory_Cleanup(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	factory := NewFilteredSharedInformerFactory(parentFactory, "k", "v", false)

	// Register some informers
	_ = factory.Core().V1().Nodes().Informer().(*FilteredInformer)
	_ = factory.Coordination().V1().Leases().Informer().(*FilteredInformer)

	factory.Cleanup()

	assert.Nil(t, factory.informers)

	// Ensure calling it again is safe
	factory.Cleanup()
}

func TestFilteredSharedInformerFactory_Concurrency(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	factory := NewFilteredSharedInformerFactory(parentFactory, "k", "v", false)

	// Test concurrent access to RegisterInformer and Cleanup
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// Writers (RegisterInformer via Informer())
	concurrency := 10
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			for j := 0; j < 100; j++ {
				_ = factory.Core().V1().Nodes().Informer()
			}
		}()
	}

	// Cleaner
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startCh
		for j := 0; j < 10; j++ {
			factory.Cleanup()
		}
	}()

	close(startCh)
	wg.Wait()
}
