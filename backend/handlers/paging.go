package handlers

import (
	"net/http"
	"strconv"
)

// PageSizes — размеры страницы, которые пользователь может выбрать для таблицы.
var PageSizes = []int{10, 25, 50, 100}

// maxPageSize ограничивает страницу, что бы ни просил запрос.
const maxPageSize = 100

// pageOf читает limit и offset запроса списка. Без limit список не делится
// на страницы (limit 0): на это рассчитывают те, кому нужен список целиком.
func pageOf(r *http.Request) (limit, offset int) {
	q := r.URL.Query()
	limit, _ = strconv.Atoi(q.Get("limit"))
	offset, _ = strconv.Atoi(q.Get("offset"))
	if limit < 0 {
		limit = 0
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 || limit == 0 {
		offset = 0
	}
	return limit, offset
}

// pageClause — окончание запроса с LIMIT/OFFSET; пусто, если список не
// делится на страницы. Оба числа приходят из pageOf, поэтому их безопасно
// подставлять в SQL.
func pageClause(limit, offset int) string {
	if limit == 0 {
		return ""
	}
	return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

// validPageSize сообщает, входит ли n в размеры, предлагаемые пользователю.
func validPageSize(n int) bool {
	for _, size := range PageSizes {
		if n == size {
			return true
		}
	}
	return false
}
