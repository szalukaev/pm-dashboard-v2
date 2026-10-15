package handlers

import (
	"net/http"
	"strconv"

	"pm-dashboard/utils"
	"pm-dashboard/workcal"
)

// CalendarHandler отдаёт производственный календарь для расчёта рабочих дней.
type CalendarHandler struct {
	Calendar *workcal.Service
}

// Days возвращает дни года: '1' — выходной или праздник, '0' — рабочий.
func (h *CalendarHandler) Days(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 2000 || year > 2100 {
		utils.Error(w, http.StatusBadRequest, "INVALID_YEAR")
		return
	}
	utils.JSON(w, http.StatusOK, h.Calendar.Get(year))
}
