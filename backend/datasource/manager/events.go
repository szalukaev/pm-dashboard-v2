package manager

import (
	"sort"
	"time"
)

// Events sent to open pages over WebSocket.
const (
	// EventSyncStatus: a sync started or finished, see SyncStatus.
	EventSyncStatus = "sync-status"
	// EventIssuesChanged: issues were added, updated or removed, see IssueChanges.
	EventIssuesChanged = "issues-changed"
	// EventCollectionComplete: a sync finished successfully.
	EventCollectionComplete = "collection-complete"
)

type SyncStatus struct {
	Status string    `json:"status"` // running | success | error
	At     time.Time `json:"at"`
}

// IssueChanges lists issue ids only: every page re-reads what it shows with
// the rights of its own user.
type IssueChanges struct {
	Added   []int `json:"added"`
	Updated []int `json:"updated"`
	Removed []int `json:"removed"`
}

func (c IssueChanges) Empty() bool {
	return len(c.Added) == 0 && len(c.Updated) == 0 && len(c.Removed) == 0
}

// DiffIssues compares two snapshots (issue id → fingerprint).
func DiffIssues(before, after map[int]string) IssueChanges {
	changes := IssueChanges{Added: []int{}, Updated: []int{}, Removed: []int{}}
	for id, hash := range after {
		old, existed := before[id]
		switch {
		case !existed:
			changes.Added = append(changes.Added, id)
		case old != hash:
			changes.Updated = append(changes.Updated, id)
		}
	}
	for id := range before {
		if _, still := after[id]; !still {
			changes.Removed = append(changes.Removed, id)
		}
	}
	sort.Ints(changes.Added)
	sort.Ints(changes.Updated)
	sort.Ints(changes.Removed)
	return changes
}
