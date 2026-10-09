package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"pm-dashboard/invoice"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type PaymentsHandler struct {
	DB **sql.DB
}

// ─── Organization ───

func (h *PaymentsHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	rows, err := (*h.DB).Query(`SELECT id, name, address, inn, contact_name, contact_phone
		FROM organizations WHERE user_id = $1 ORDER BY name`, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type Org struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		Address     *string `json:"address"`
		INN         *string `json:"inn"`
		ContactName *string `json:"contact_name"`
		ContactPhone *string `json:"contact_phone"`
	}
	var orgs []Org
	for rows.Next() {
		var o Org
		if rows.Scan(&o.ID, &o.Name, &o.Address, &o.INN, &o.ContactName, &o.ContactPhone) == nil {
			orgs = append(orgs, o)
		}
	}
	if orgs == nil {
		orgs = []Org{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"organizations": orgs})
}

func (h *PaymentsHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	var body struct {
		Name         string  `json:"name"`
		Address      *string `json:"address"`
		INN          *string `json:"inn"`
		ContactName  *string `json:"contact_name"`
		ContactPhone *string `json:"contact_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var id int
	err := (*h.DB).QueryRow(`INSERT INTO organizations (user_id, name, address, inn, contact_name, contact_phone)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, body.Name, body.Address, body.INN, body.ContactName, body.ContactPhone).Scan(&id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": id, "success": true})
}

func (h *PaymentsHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	orgID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)

	sets, args, idx := buildUpdateSets(body, map[string]bool{
		"name": true, "address": true, "inn": true, "contact_name": true, "contact_phone": true,
	}, 1)
	if len(sets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS")
		return
	}
	args = append(args, orgID, userID)
	res, err := (*h.DB).Exec("UPDATE organizations SET "+utils.JoinStrings(sets, ",")+" WHERE id=$"+utils.Itoa(idx)+" AND user_id=$"+utils.Itoa(idx+1), args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
		return
	}
	utils.Success(w)
}

func (h *PaymentsHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	orgID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Unlink contracts instead of deleting
	if _, err := (*h.DB).Exec("UPDATE contracts SET organization_id = NULL WHERE organization_id = $1 AND user_id = $2", orgID, userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM organizations WHERE id = $1 AND user_id = $2", orgID, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
		return
	}
	utils.Success(w)
}

// ─── Contract ───

// paymentContract is a contract with what is computed for it.
type paymentContract struct {
	ID             int     `json:"id"`
	OrganizationID *int    `json:"organization_id"`
	ContractType   string  `json:"contract_type"`
	Name           string  `json:"name"`
	CompanyName    *string `json:"company_name"`
	CompanyAddress *string `json:"company_address"`
	Amount         float64 `json:"amount"`
	VatRate        string  `json:"vat_rate"`
	ContactName    *string `json:"contact_name"`
	ContactPhone   *string `json:"contact_phone"`
	StartDate      *string `json:"start_date"`
	EndDate        *string `json:"end_date"`
	OrgName        string  `json:"org_name"`
	// TotalAmount: the amount of the contract with VAT.
	TotalAmount float64 `json:"total_amount"`
	// InvoicedAmount: the sum of the invoices, with VAT.
	InvoicedAmount float64 `json:"invoiced_amount"`
	TotalPaid      float64 `json:"total_paid"`
	// DebtAmount: invoiced and not paid yet.
	DebtAmount  float64 `json:"debt_amount"`
	IsClosed    bool    `json:"is_closed"`
	IsOverdue   bool    `json:"is_overdue"`
	IsFullyPaid bool    `json:"is_fully_paid"`

	money contractMoney
}

// contractFilter is what the list of contracts is narrowed by.
type contractFilter struct {
	Type       string // "service" | "onetime" | "" for both
	Search     string // in the name of the contract or of the organization
	ShowClosed bool
}

func contractFilterOf(r *http.Request) contractFilter {
	q := r.URL.Query()
	return contractFilter{
		Type:       q.Get("type"),
		Search:     strings.TrimSpace(q.Get("search")),
		ShowClosed: q.Get("show_closed") == "true",
	}
}

// loadContracts returns the contracts of the user with their computed
// state. A nil filter gives every contract.
func (h *PaymentsHandler) loadContracts(userID int, filter *contractFilter) ([]paymentContract, error) {
	where := []string{"c.user_id = $1"}
	args := []interface{}{userID}
	if filter != nil && filter.Type != "" {
		args = append(args, filter.Type)
		where = append(where, "c.contract_type = $"+utils.Itoa(len(args)))
	}
	if filter != nil && filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		n := utils.Itoa(len(args))
		where = append(where, "(LOWER(c.name) LIKE $"+n+" OR LOWER(COALESCE(c.company_name, '')) LIKE $"+n+
			" OR LOWER(COALESCE(o.name, '')) LIKE $"+n+")")
	}

	rows, err := (*h.DB).Query(`SELECT c.id, c.organization_id, c.contract_type, c.name, c.company_name, c.company_address,
		c.amount, c.vat_rate, c.contact_name, c.contact_phone, c.start_date, c.end_date,
		COALESCE(o.name, '') AS org_name,
		COALESCE((SELECT SUM(i.amount * CASE WHEN i.vat_rate = '5' THEN 1.05 ELSE 1 END)
			FROM invoices i WHERE i.contract_id = c.id), 0) AS invoiced_amount,
		COALESCE((SELECT SUM(p.amount) FROM contract_payments p
			JOIN invoices i ON p.invoice_id = i.id WHERE i.contract_id = c.id), 0) AS total_paid
		FROM contracts c LEFT JOIN organizations o ON c.organization_id = o.id
		WHERE `+utils.JoinStrings(where, " AND ")+` ORDER BY c.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contracts := []paymentContract{}
	today := time.Now().Format("2006-01-02")
	for rows.Next() {
		var c paymentContract
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.ContractType, &c.Name, &c.CompanyName, &c.CompanyAddress,
			&c.Amount, &c.VatRate, &c.ContactName, &c.ContactPhone, &c.StartDate, &c.EndDate,
			&c.OrgName, &c.InvoicedAmount, &c.TotalPaid); err != nil {
			return nil, err
		}
		c.InvoicedAmount = utils.Round2(c.InvoicedAmount)
		c.money = contractMoney{Type: c.ContractType, Amount: c.Amount, VatRate: c.VatRate,
			Invoiced: c.InvoicedAmount, Paid: c.TotalPaid, EndDate: c.EndDate}
		c.TotalAmount = c.money.Total()
		c.DebtAmount = c.money.Debt()
		c.IsFullyPaid = c.money.FullyPaid()
		c.IsClosed = c.money.Closed(today)
		c.IsOverdue = c.money.Overdue(today)

		if filter != nil && !filter.ShowClosed && c.IsClosed {
			continue
		}
		contracts = append(contracts, c)
	}
	return contracts, rows.Err()
}

func (h *PaymentsHandler) ListContracts(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	filter := contractFilterOf(r)
	contracts, err := h.loadContracts(middleware.GetUserID(r), &filter)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"contracts": contracts})
}

func (h *PaymentsHandler) CreateContract(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	var body struct {
		ContractType   string   `json:"contract_type"`
		Name           string   `json:"name"`
		OrganizationID *int     `json:"organization_id"`
		CompanyName    *string  `json:"company_name"`
		CompanyAddress *string  `json:"company_address"`
		Amount         *float64 `json:"amount"`
		VatRate        *string  `json:"vat_rate"`
		ContactName    *string  `json:"contact_name"`
		ContactPhone   *string  `json:"contact_phone"`
		StartDate      *string  `json:"start_date"`
		EndDate        *string  `json:"end_date"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" || body.ContractType == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	// Inherit from org if provided (only the caller's own organization)
	if body.OrganizationID != nil {
		var orgName, orgAddr, orgContact, orgPhone sql.NullString
		err := (*h.DB).QueryRow("SELECT name, address, contact_name, contact_phone FROM organizations WHERE id=$1 AND user_id=$2", *body.OrganizationID, userID).Scan(&orgName, &orgAddr, &orgContact, &orgPhone)
		if err != nil {
			// Foreign or missing organization — refuse rather than cross-link.
			utils.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
			return
		}
		if body.CompanyName == nil && orgName.Valid { body.CompanyName = &orgName.String }
		if body.CompanyAddress == nil && orgAddr.Valid { body.CompanyAddress = &orgAddr.String }
		if body.ContactName == nil && orgContact.Valid { body.ContactName = &orgContact.String }
		if body.ContactPhone == nil && orgPhone.Valid { body.ContactPhone = &orgPhone.String }
	}
	vatRate := "none"
	if body.VatRate != nil { vatRate = *body.VatRate }
	amount := 0.0
	if body.Amount != nil { amount = *body.Amount }

	var id int
	err := (*h.DB).QueryRow(`INSERT INTO contracts (user_id, organization_id, contract_type, name, company_name,
		company_address, amount, vat_rate, contact_name, contact_phone, start_date, end_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		userID, body.OrganizationID, body.ContractType, body.Name, body.CompanyName,
		body.CompanyAddress, amount, vatRate, body.ContactName, body.ContactPhone,
		body.StartDate, body.EndDate).Scan(&id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": id, "success": true})
}

func (h *PaymentsHandler) UpdateContract(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	sets, args, idx := buildUpdateSets(body, map[string]bool{
		"name": true, "contract_type": true, "organization_id": true, "company_name": true,
		"company_address": true, "amount": true, "vat_rate": true, "contact_name": true,
		"contact_phone": true, "start_date": true, "end_date": true,
	}, 1)
	if len(sets) == 0 { utils.Error(w, http.StatusBadRequest, "NO_FIELDS"); return }
	args = append(args, contractID, userID)
	res, err := (*h.DB).Exec("UPDATE contracts SET "+utils.JoinStrings(sets, ",")+" WHERE id=$"+utils.Itoa(idx)+" AND user_id=$"+utils.Itoa(idx+1), args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND")
		return
	}
	utils.Success(w)
}

func (h *PaymentsHandler) DeleteContract(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM contracts WHERE id=$1 AND user_id=$2", contractID, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND")
		return
	}
	utils.Success(w)
}

// ─── Invoice ───

func (h *PaymentsHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Ownership: invoices are visible only via a contract belonging to the caller.
	limit, offset := pageOf(r)
	total := 0
	if err := (*h.DB).QueryRow(`SELECT COUNT(*) FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.contract_id=$1 AND c.user_id=$2`, contractID, userID).Scan(&total); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	rows, err := (*h.DB).Query(`SELECT i.id, i.amount, i.vat_rate, i.issued_at, i.paid_amount, i.status,
			(SELECT MAX(p.paid_at)::text FROM contract_payments p WHERE p.invoice_id = i.id)
		FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.contract_id=$1 AND c.user_id=$2 ORDER BY i.issued_at DESC, i.id DESC`+pageClause(limit, offset), contractID, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED"); return
	}
	defer rows.Close()

	type Invoice struct {
		ID        int     `json:"id"`
		Amount    float64 `json:"amount"`
		VatRate   string  `json:"vat_rate"`
		IssuedAt  string  `json:"issued_at"`
		PaidAmount float64 `json:"paid_amount"`
		Status    string  `json:"status"`
		Total     float64 `json:"total"`
		Remainder float64 `json:"remainder"`
		// Number: the date of issue as YYYYMM-DD
		Number string `json:"number"`
		// PaidAt: the date of the last payment, nil when nothing is paid
		PaidAt *string `json:"paid_at"`
	}
	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		if rows.Scan(&inv.ID, &inv.Amount, &inv.VatRate, &inv.IssuedAt, &inv.PaidAmount, &inv.Status, &inv.PaidAt) != nil {
			continue
		}
		inv.Total = utils.Round2(inv.Amount * vatMultiplier(inv.VatRate))
		inv.Remainder = utils.Round2(inv.Total - inv.PaidAmount)
		if inv.Remainder < 0 {
			inv.Remainder = 0
		}
		inv.Number = invoiceNumber(inv.IssuedAt)
		invoices = append(invoices, inv)
	}
	if invoices == nil { invoices = []Invoice{} }
	utils.JSON(w, http.StatusOK, map[string]interface{}{"invoices": invoices, "total": total})
}

func (h *PaymentsHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var contract struct{ Amount float64; VatRate string; Name string }
	err = (*h.DB).QueryRow("SELECT amount, vat_rate, name FROM contracts WHERE id=$1 AND user_id=$2", contractID, userID).Scan(&contract.Amount, &contract.VatRate, &contract.Name)
	if err != nil { utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND"); return }

	var body struct {
		Items []struct{ Name string; Quantity float64; Price float64 } `json:"items"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if len(body.Items) == 0 {
		body.Items = []struct{ Name string; Quantity float64; Price float64 }{{Name: contract.Name, Quantity: 1, Price: contract.Amount}}
	}

	totalAmount := 0.0
	for _, item := range body.Items {
		totalAmount += item.Quantity * item.Price
	}
	totalAmount = utils.Round2(totalAmount)

	today := time.Now().Format("2006-01-02")

	// Invoice header + items must be written atomically — otherwise a partial
	// failure leaves an invoice without its line items.
	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	var invoiceID int
	err = tx.QueryRow(`INSERT INTO invoices (contract_id, amount, vat_rate, issued_at, paid_amount, status)
		VALUES ($1,$2,$3,$4,0,'unpaid') RETURNING id`, contractID, totalAmount, contract.VatRate, today).Scan(&invoiceID)
	if err != nil {
		tx.Rollback()
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	for _, item := range body.Items {
		if _, err := tx.Exec("INSERT INTO invoice_items (invoice_id, name, quantity, price) VALUES ($1,$2,$3,$4)",
			invoiceID, item.Name, item.Quantity, item.Price); err != nil {
			tx.Rollback()
			utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": invoiceID, "success": true})
}

func (h *PaymentsHandler) DeleteInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Ownership: delete only an invoice of the caller's own contract.
	res, err := (*h.DB).Exec(`DELETE FROM invoices WHERE id=$1 AND contract_id IN
		(SELECT id FROM contracts WHERE user_id=$2)`, invoiceID, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED"); return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND"); return
	}
	utils.Success(w)
}

func (h *PaymentsHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	// Ownership: pay only an invoice of the caller's own contract.
	var inv struct{ Amount float64; VatRate string; PaidAmount float64 }
	err = (*h.DB).QueryRow(`SELECT i.amount, i.vat_rate, i.paid_amount
		FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.id=$1 AND c.user_id=$2`, invoiceID, userID).Scan(&inv.Amount, &inv.VatRate, &inv.PaidAmount)
	if err != nil { utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND"); return }

	totalObligation := utils.Round2(inv.Amount * vatMultiplier(inv.VatRate))
	remainder := totalObligation - inv.PaidAmount

	var body struct{ Amount *float64 `json:"amount"`; PaidAt *string `json:"paid_at"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// More than is left to pay is cut to the remainder; the answer says so,
	// and the page tells the user how much was taken.
	payAmount, clamped, ok := appliedPayment(body.Amount, remainder)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_AMOUNT")
		return
	}

	payDate := time.Now().Format("2006-01-02")
	if body.PaidAt != nil { payDate = *body.PaidAt }

	newPaid := utils.Round2(inv.PaidAmount + payAmount)
	if newPaid > totalObligation { newPaid = totalObligation }
	status := "partial"
	if newPaid >= totalObligation { status = "paid" }

	// Atomic update of invoice + payment row (avoids lost updates on
	// concurrent payments and inconsistent state if one statement fails).
	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "TX_FAILED")
		return
	}
	if _, err := tx.Exec("UPDATE invoices SET paid_amount=$1, status=$2 WHERE id=$3", newPaid, status, invoiceID); err != nil {
		tx.Rollback()
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if _, err := tx.Exec("INSERT INTO contract_payments (invoice_id, amount, paid_at) VALUES ($1,$2,$3)", invoiceID, payAmount, payDate); err != nil {
		tx.Rollback()
		utils.Error(w, http.StatusInternalServerError, "INSERT_FAILED")
		return
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "COMMIT_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "paid_amount": newPaid, "status": status,
		"applied": payAmount, "clamped": clamped,
	})
}

func (h *PaymentsHandler) DownloadInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	// Get invoice + contract info (only the caller's own invoice)
	var inv struct {
		ContractID int
		Amount     float64
		VatRate    string
		IssuedAt   string
	}
	err = (*h.DB).QueryRow(`SELECT i.contract_id, i.amount, i.vat_rate, i.issued_at
		FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.id=$1 AND c.user_id=$2`, invoiceID, userID).Scan(
		&inv.ContractID, &inv.Amount, &inv.VatRate, &inv.IssuedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND")
		return
	}

	var contract struct {
		CompanyName    string
		CompanyAddress string
		ContactPhone   string
		ContactName    string
		VatRate        string
		Name           string
	}
	(*h.DB).QueryRow(`SELECT COALESCE(company_name,''), COALESCE(company_address,''),
		COALESCE(contact_phone,''), COALESCE(contact_name,''), vat_rate, name
		FROM contracts WHERE id=$1 AND user_id=$2`, inv.ContractID, userID).Scan(
		&contract.CompanyName, &contract.CompanyAddress, &contract.ContactPhone,
		&contract.ContactName, &contract.VatRate, &contract.Name)

	// Get invoice items
	rows, err := (*h.DB).Query("SELECT name, quantity, price FROM invoice_items WHERE invoice_id=$1", invoiceID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var items []invoice.InvoiceItem
	for rows.Next() {
		var it invoice.InvoiceItem
		if rows.Scan(&it.Name, &it.Quantity, &it.Price) == nil {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		items = []invoice.InvoiceItem{{Name: contract.Name, Quantity: 1, Price: inv.Amount}}
	}

	// Generate .docx
	templatePath := "invoice/invoice_template.docx"
	outPath, downloadName, err := invoice.GenerateInvoice(templatePath, invoice.ContractInfo{
		CompanyName:    contract.CompanyName,
		CompanyAddress: contract.CompanyAddress,
		ContactPhone:   contract.ContactPhone,
		ContactName:    contract.ContactName,
		VatRate:        contract.VatRate,
		Name:           contract.Name,
		IssuedAt:       firstN(inv.IssuedAt, 10),
	}, items)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DOCX_GENERATION_FAILED")
		return
	}

	// Serve file
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	// The name is in Russian: filename* carries it encoded, filename is the
	// fallback for what cannot read that
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"invoice.docx\"; filename*=UTF-8''%s", url.PathEscape(downloadName)))
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
	http.ServeFile(w, r, outPath)

	// Cleanup after 60 seconds
	go func() {
		time.Sleep(60 * time.Second)
		os.Remove(outPath)
	}()
}

// ─── Statistics ───

// GetStats returns the figures of the cards above the list. They always
// cover every contract of the user, whatever the filters show.
func (h *PaymentsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	contracts, err := h.loadContracts(middleware.GetUserID(r), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	money := make([]contractMoney, len(contracts))
	for i, c := range contracts {
		money[i] = c.money
	}
	totals := sumContracts(money, time.Now().Format("2006-01-02"))

	stats := []map[string]interface{}{
		{"label": "contracts", "value": totals.Contracts},
		{"label": "total_amount", "value": totals.TotalAmount},
		{"label": "invoiced", "value": totals.Invoiced},
		{"label": "debt", "value": totals.Debt},
		{"label": "paid", "value": totals.Paid},
		{"label": "closed_amount", "value": totals.ClosedAmount},
	}
	// The debt card is red only when there is a debt
	if totals.Debt > 0 {
		stats[3]["variant"] = "danger"
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"stats": stats})
}

// ─── CSV Export ───

// ExportCSV writes the contracts the list shows — the same filters apply.
func (h *PaymentsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	filter := contractFilterOf(r)
	contracts, err := h.loadContracts(middleware.GetUserID(r), &filter)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	sort.SliceStable(contracts, func(i, j int) bool {
		return strings.ToLower(contracts[i].Name) < strings.ToLower(contracts[j].Name)
	})

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=contracts.csv")
	// BOM for Excel
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	for _, row := range contractsCSV(contracts, time.Now().Format("2006-01-02")) {
		writer.Write(row)
	}
	writer.Flush()
}

// contractsCSV builds the rows of the export, the header first. Amounts
// are with VAT and use a decimal comma, as spreadsheets expect it here.
func contractsCSV(contracts []paymentContract, today string) [][]string {
	text := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	money := func(v float64) string {
		return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1)
	}
	rows := [][]string{{"Тип", "Название", "Организация", "Адрес", "Сумма", "НДС", "Выставлено", "Долг",
		"Контакт", "Телефон", "Дата начала", "Срок", "Статус"}}
	for _, c := range contracts {
		typeName := "Сопровождение"
		if c.ContractType == "onetime" {
			typeName = "Разовый"
		}
		vatLabel := "Без НДС"
		if c.VatRate == "5" {
			vatLabel = "НДС 5%"
		}
		organization := c.OrgName
		if organization == "" {
			organization = text(c.CompanyName)
		}
		rows = append(rows, []string{typeName, c.Name, organization, text(c.CompanyAddress),
			money(c.TotalAmount), vatLabel, money(c.InvoicedAmount), money(c.DebtAmount),
			text(c.ContactName), text(c.ContactPhone),
			firstN(text(c.StartDate), 10), firstN(text(c.EndDate), 10), c.money.ExportStatus(today)})
	}
	return rows
}

// ─── Helpers ───

func buildUpdateSets(body map[string]interface{}, allowed map[string]bool, startIdx int) ([]string, []interface{}, int) {
	sets := []string{}
	args := []interface{}{}
	idx := startIdx
	for k, v := range body {
		if allowed[k] {
			sets = append(sets, k+"=$"+utils.Itoa(idx))
			args = append(args, v)
			idx++
		}
	}
	return sets, args, idx
}
