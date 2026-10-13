package datasource

import (
	"testing"
)

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Регистрируется фабрика-заглушка
	registry.Register("test", func(cfg SourceConfig) DataSource {
		return nil
	})

	// В списке должен быть "test"
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

	// Зарегистрированный источник создаётся
	_, err := registry.Create("test", SourceConfig{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Незарегистрированный источник даёт ошибку
	_, err = registry.Create("unknown", SourceConfig{})
	if err == nil {
		t.Error("Expected error for unregistered source")
	}
}
