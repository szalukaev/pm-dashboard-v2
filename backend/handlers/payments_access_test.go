package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pm-dashboard/access"
	"pm-dashboard/middleware"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Запрещённое действие с оплатами отклоняется до обращения к базе: база
// здесь открыта, но недоступна, и любой запрос к ней дал бы 500.
func TestPaymentActionsForbidden(t *testing.T) {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &PaymentsHandler{DB: &db}

	cases := []struct {
		action  string
		method  string
		handler http.HandlerFunc
	}{
		{access.PaymentOrgCreate, "POST", h.CreateOrganization},
		{access.PaymentOrgEdit, "PUT", h.UpdateOrganization},
		{access.PaymentOrgDelete, "DELETE", h.DeleteOrganization},
		{access.PaymentContractCreate, "POST", h.CreateContract},
		{access.PaymentContractEdit, "PUT", h.UpdateContract},
		{access.PaymentContractDelete, "DELETE", h.DeleteContract},
		{access.PaymentInvoiceCreate, "POST", h.CreateInvoice},
		{access.PaymentInvoiceDelete, "DELETE", h.DeleteInvoice},
		{access.PaymentInvoicePay, "POST", h.PayInvoice},
		{access.PaymentDownload, "GET", h.DownloadInvoice},
		{access.PaymentDownload, "GET", h.ExportCSV},
	}
	for _, c := range cases {
		// Разрешено всё, кроме проверяемого действия
		others := []string{}
		for _, a := range access.PaymentActions {
			if a != c.action {
				others = append(others, a)
			}
		}
		req := httptest.NewRequest(c.method, "/", strings.NewReader(`{"name":"x"}`))
		req = mux.SetURLVars(req, map[string]string{"id": "1", "invoiceId": "1"})
		req = middleware.WithScope(req, &access.Scope{UserID: 5, PaymentActions: &others})
		rec := httptest.NewRecorder()
		c.handler(rec, req)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "PAYMENT_ACTION_FORBIDDEN") {
			t.Errorf("%s без права %q: ответ %d %s", c.method, c.action, rec.Code, rec.Body.String())
		}
	}
}

func TestPaymentsOwnerFilter(t *testing.T) {
	scoped := func(s *access.Scope) *http.Request {
		return middleware.WithScope(httptest.NewRequest("GET", "/", nil), s)
	}
	// «Только свои» — фильтр по автору, иначе NULL: видны все записи
	if got := paymentsOwner(scoped(&access.Scope{UserID: 5, PaymentsOwnOnly: true})); got != 5 {
		t.Errorf("только свои: параметр %v, ожидался 5", got)
	}
	if got := paymentsOwner(scoped(&access.Scope{UserID: 5})); got != nil {
		t.Errorf("общие оплаты: параметр %v, ожидался NULL", got)
	}
	if got := paymentsOwner(scoped(&access.Scope{UserID: 1, Admin: true, PaymentsOwnOnly: true})); got != nil {
		t.Errorf("администратор: параметр %v, ожидался NULL", got)
	}
	if got := ownedBy("c.user_id", 2); got != "($2::int IS NULL OR c.user_id = $2)" {
		t.Errorf("условие = %s", got)
	}
}
