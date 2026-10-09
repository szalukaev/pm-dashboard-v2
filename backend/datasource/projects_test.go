package datasource

import (
	"reflect"
	"testing"
)

func TestWithDescendants(t *testing.T) {
	// 1 → 2 → 4, 1 → 3; 5 is unrelated; 6 ↔ 7 is a cycle
	children := map[int][]int{1: {2, 3}, 2: {4}, 6: {7}, 7: {6}}

	tests := []struct {
		name string
		ids  []int
		want []int
	}{
		{"root with nested children", []int{1}, []int{1, 2, 3, 4}},
		{"leaf", []int{4}, []int{4}},
		{"parent and its child given together", []int{2, 1}, []int{2, 1, 4, 3}},
		{"unrelated project", []int{5}, []int{5}},
		{"cycle does not loop", []int{6}, []int{6, 7}},
		{"nothing selected", nil, []int{}},
	}
	for _, tt := range tests {
		if got := withDescendants(tt.ids, children); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
