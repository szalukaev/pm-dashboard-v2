package handlers

import (
	"net/http"
	"strconv"
)

// PageSizes are the page sizes a user may choose for a table.
var PageSizes = []int{10, 25, 50, 100}

// maxPageSize bounds a page whatever the request asks for.
const maxPageSize = 100

// pageOf reads limit and offset of a list request. Without a limit the list
// is not paged (limit 0): that is what the callers that need the whole list
// rely on.
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

// pageClause is the LIMIT/OFFSET tail of a query; empty when the list is not
// paged. Both numbers come from pageOf, so they are safe to put into SQL.
func pageClause(limit, offset int) string {
	if limit == 0 {
		return ""
	}
	return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

// validPageSize reports whether n is one of the sizes offered to the user.
func validPageSize(n int) bool {
	for _, size := range PageSizes {
		if n == size {
			return true
		}
	}
	return false
}
