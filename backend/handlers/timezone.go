package handlers

import (
	"strings"
	"time"
)

// maxTimezoneLen — длина колонок timezone и default_timezone в базе.
const maxTimezoneLen = 64

// validTimezone проверяет имя часового пояса, присланное клиентом. Пустая
// строка допустима и значит «не задан». «Local» отклоняется: это пояс
// сервера, а не имя из базы поясов.
func validTimezone(name string) bool {
	if name == "" {
		return true
	}
	if len(name) > maxTimezoneLen || name == "Local" || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// serverTimezone — имя пояса, по которому сервер ведёт расписания.
func serverTimezone() string {
	name := time.Local.String()
	if name == "Local" {
		// Имя неизвестно (TZ не задан): показываем смещение
		return time.Now().Format("-07:00")
	}
	return name
}
