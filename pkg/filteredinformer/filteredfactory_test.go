package filteredinformer

import (
	"sync"
	"testing"

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

	if factory == nil {
		t.Fatal("Expected factory to be non-nil")
	}
	if factory.filterKey != filterKey {
		t.Errorf("Expected filterKey %q, got %q", filterKey, factory.filterKey)
	}
	if factory.filterValue != filterValue {
		t.Errorf("Expected filterValue %q, got %q", filterValue, factory.filterValue)
	}
	if factory.allowMissing != allowMissing {
		t.Errorf("Expected allowMissing %t, got %t", allowMissing, factory.allowMissing)
	}
	if factory.SharedInformerFactory != parentFactory {
		t.Errorf("Expected SharedInformerFactory %p, got %p", parentFactory, factory.SharedInformerFactory)
	}
}

func TestFilteredSharedInformerFactory_Core_Nodes(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	filterKey := "test-key"
	filterValue := "test-value"
	allowMissing := false
	factory := NewFilteredSharedInformerFactory(parentFactory, filterKey, filterValue, allowMissing)

	nodeInformer := factory.Core().V1().Nodes()
	if nodeInformer == nil {
		t.Fatal("Expected nodeInformer to be non-nil")
	}

	_, ok := nodeInformer.(*FilteredNodeInformer)
	if !ok {
		t.Errorf("Expected nodeInformer to be of type *FilteredNodeInformer, got %T", nodeInformer)
	}

	sharedInformer := nodeInformer.Informer()
	filteredInf, ok := sharedInformer.(*ProviderConfigFilteredInformer)
	if !ok {
		t.Errorf("Expected sharedInformer to be of type *ProviderConfigFilteredInformer, got %T", sharedInformer)
	} else {
		if filteredInf.filterKey != filterKey {
			t.Errorf("Expected filterKey %q, got %q", filterKey, filteredInf.filterKey)
		}
		if filteredInf.filterValue != filterValue {
			t.Errorf("Expected filterValue %q, got %q", filterValue, filteredInf.filterValue)
		}
		if filteredInf.allowMissing != allowMissing {
			t.Errorf("Expected allowMissing %t, got %t", allowMissing, filteredInf.allowMissing)
		}
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
	if leaseInformer == nil {
		t.Fatal("Expected leaseInformer to be non-nil")
	}

	_, ok := leaseInformer.(*FilteredLeaseInformer)
	if !ok {
		t.Errorf("Expected leaseInformer to be of type *FilteredLeaseInformer, got %T", leaseInformer)
	}

	sharedInformer := leaseInformer.Informer()
	filteredInf, ok := sharedInformer.(*ProviderConfigFilteredInformer)
	if !ok {
		t.Errorf("Expected sharedInformer to be of type *ProviderConfigFilteredInformer, got %T", sharedInformer)
	} else {
		if filteredInf.filterKey != filterKey {
			t.Errorf("Expected filterKey %q, got %q", filterKey, filteredInf.filterKey)
		}
		if filteredInf.filterValue != filterValue {
			t.Errorf("Expected filterValue %q, got %q", filterValue, filteredInf.filterValue)
		}
		if filteredInf.allowMissing != allowMissing {
			t.Errorf("Expected allowMissing %t, got %t", allowMissing, filteredInf.allowMissing)
		}
	}
}

func TestFilteredSharedInformerFactory_Cleanup(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	factory := NewFilteredSharedInformerFactory(parentFactory, "k", "v", false)

	_ = factory.Core().V1().Nodes().Informer()
	_ = factory.Coordination().V1().Leases().Informer()

	factory.Cleanup()

	if factory.informers != nil {
		t.Errorf("Expected factory.informers to be nil after Cleanup, got %v", factory.informers)
	}

	// Ensure calling it again is safe
	factory.Cleanup()
}

func TestFilteredSharedInformerFactory_Concurrency(t *testing.T) {
	client := fake.NewSimpleClientset()
	parentFactory := informers.NewSharedInformerFactory(client, 0)
	factory := NewFilteredSharedInformerFactory(parentFactory, "k", "v", false)

	var wg sync.WaitGroup
	startCh := make(chan struct{})

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
