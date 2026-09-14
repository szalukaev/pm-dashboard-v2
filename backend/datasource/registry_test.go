package datasource

import (
	"testing"
)

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Register a mock factory
	registry.Register("test", func(cfg SourceConfig) DataSource {
		return nil
	})

	// List should contain "test"
	names := registry.List()
	found := false
	for _, n := range names {
		if n == "test" {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'test' in registry list")
	}

	// Create should work for registered source
	_, err := registry.Create("test", SourceConfig{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Create should fail for unregistered source
	_, err = registry.Create("unknown", SourceConfig{})
	if err == nil {
		t.Error("Expected error for unregistered source")
	}
}
