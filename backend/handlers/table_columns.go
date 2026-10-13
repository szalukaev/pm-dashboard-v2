package handlers

// Пределы сохранённой настройки колонок: хватает любой таблице дашборда.
const (
	maxTableNameLen = 50
	maxColumnKeyLen = 50
	maxTableColumns = 50
)

// validTableColumns проверяет настройки колонок, присланные клиентом:
// {"<таблица>": {"visible": [ключи], "order": [ключи]}}. Сами ключи здесь не
// проверяются: таблицы живут в клиенте, и неизвестные ключи он пропускает.
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
