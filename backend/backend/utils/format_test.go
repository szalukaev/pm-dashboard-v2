package utils

import (
	"testing"
	"time"
)

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0,00 ₽"},
		{150000, "150 000,00 ₽"},
		{1234.56, "1 234,56 ₽"},
		{999, "999,00 ₽"},
		{1000000, "1 000 000,00 ₽"},
	}
	for _, tt := range tests {
		result := FormatMoney(tt.input)
		if result != tt.expected {
			t.Errorf("FormatMoney(%f) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		input    time.Time
		expected string
	}{
		{time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), "15.01.2025"},
		{time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), "31.12.2025"},
	}
	for _, tt := range tests {
		result := FormatDate(tt.input)
		if result != tt.expected {
			t.Errorf("FormatDate(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0,00"},
		{1234.56, "1 234,56"},
		{1000000, "1 000 000,00"},
	}
	for _, tt := range tests {
		result := FormatNumber(tt.input)
		if result != tt.expected {
			t.Errorf("FormatNumber(%f) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
