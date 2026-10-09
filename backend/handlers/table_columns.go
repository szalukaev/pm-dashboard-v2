package handlers

// Limits of a saved column setup: enough for any table of the dashboard.
const (
	maxTableNameLen = 50
	maxColumnKeyLen = 50
	maxTableColumns = 50
)

// validTableColumns checks the column setups sent by the client:
// {"<table>": {"visible": [keys], "order": [keys]}}. The keys themselves are
// not checked here — the tables live in the client, which ignores the keys
// it does not know.
func validTableColumns(v interface{}) bool {
	tables, ok := v.(map[string]interface{})
	if !ok {
		return false
	}
	for table, setup := range tables {
		lists, ok := setup.(map[string]interface{})
		if !ok || table == "" || len(table) > maxTableNameLen || len(lists) != 2 {
			return false
		}
		if !validColumnKeys(lists["visible"]) || !validColumnKeys(lists["order"]) {
			return false
		}
	}
	return true
}

func validColumnKeys(v interface{}) bool {
	keys, ok := v.([]interface{})
	if !ok || len(keys) > maxTableColumns {
		return false
	}
	for _, k := range keys {
		key, isString := k.(string)
		if !isString || key == "" || len(key) > maxColumnKeyLen {
			return false
		}
	}
	return true
}
