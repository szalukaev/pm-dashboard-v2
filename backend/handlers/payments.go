package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"pm-dashboard/invoice"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type PaymentsHandler struct {
	DB *sql.DB
}

// ─── Organization ───

func (h *PaymentsHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.DB.Query(`SELECT id, name, address, inn, contact_name, contact_phone
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
	err := h.DB.QueryRow(`INSERT INTO organizations (user_id, name, address, inn, contact_name, contact_phone)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, body.Name, body.Address, body.INN, body.ContactName, body.ContactPhone).Scan(&id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": id, "success": true})
}

func (h *PaymentsHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	orgID, _ := strconv.Atoi(mux.Vars(r)["id"])
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
	h.DB.Exec("UPDATE organizations SET "+utils.JoinStrings(sets, ",")+" WHERE id=$"+utils.Itoa(idx)+" AND user_id=$"+utils.Itoa(idx+1), args...)
	utils.Success(w)
}

func (h *PaymentsHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	orgID, _ := strconv.Atoi(mux.Vars(r)["id"])
	// Unlink contracts instead of deleting
	h.DB.Exec("UPDATE contracts SET organization_id = NULL WHERE organization_id = $1 AND user_id = $2", orgID, userID)
	h.DB.Exec("DELETE FROM organizations WHERE id = $1 AND user_id = $2", orgID, userID)
	utils.Success(w)
}

// ─── Contract ───

func (h *PaymentsHandler) ListContracts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	filterType := r.URL.Query().Get("type")      // "service", "onetime"
	showClosed := r.URL.Query().Get("show_closed") // "true"
	search := r.URL.Query().Get("search")

	where := []string{"c.user_id = $1"}
	args := []interface{}{userID}
	idx := 2

	if filterType != "" {
		where = append(where, "c.contract_type = $"+utils.Itoa(idx))
		args = append(args, filterType)
		idx++
	}
	if search != "" {
		where = append(where, "(LOWER(c.name) LIKE $"+utils.Itoa(idx)+" OR LOWER(c.company_name) LIKE $"+utils.Itoa(idx)+")")
		args = append(args, "%"+search+"%")
		idx++
	}

	query := `SELECT c.id, c.organization_id, c.contract_type, c.name, c.company_name, c.company_address,
		c.amount, c.vat_rate, c.contact_name, c.contact_phone, c.start_date, c.end_date,
		COALESCE(o.name, '') as org_name,
		COALESCE((SELECT SUM(i.amount) FROM invoices i WHERE i.contract_id = c.id), 0) as invoiced_amount,
		COALESCE((SELECT SUM(p.amount) FROM contract_payments p JOIN invoices i ON p.invoice_id = i.id WHERE i.contract_id = c.id), 0) as total_paid
		FROM contracts c LEFT JOIN organizations o ON c.organization_id = o.id
		WHERE ` + utils.JoinStrings(where, " AND ") + ` ORDER BY c.created_at DESC`

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type Contract struct {
		ID             int      `json:"id"`
		OrganizationID *int     `json:"organization_id"`
		ContractType   string   `json:"contract_type"`
		Name           string   `json:"name"`
		CompanyName    *string  `json:"company_name"`
		CompanyAddress *string  `json:"company_address"`
		Amount         float64  `json:"amount"`
		VatRate        string   `json:"vat_rate"`
		ContactName    *string  `json:"contact_name"`
		ContactPhone   *string  `json:"contact_phone"`
		StartDate      *string  `json:"start_date"`
		EndDate        *string  `json:"end_date"`
		OrgName        string   `json:"org_name"`
		InvoicedAmount float64  `json:"invoiced_amount"`
		TotalPaid      float64  `json:"total_paid"`
		DebtAmount     float64  `json:"debt_amount"`
		IsClosed       bool     `json:"is_closed"`
		IsOverdue      bool     `json:"is_overdue"`
	}

	var contracts []Contract
	today := time.Now().Format("2006-01-02")
	for rows.Next() {
		var c Contract
		if rows.Scan(&c.ID, &c.OrganizationID, &c.ContractType, &c.Name, &c.CompanyName, &c.CompanyAddress,
			&c.Amount, &c.VatRate, &c.ContactName, &c.ContactPhone, &c.StartDate, &c.EndDate,
			&c.OrgName, &c.InvoicedAmount, &c.TotalPaid) != nil {
			continue
		}
		vatMul := 1.0
		if c.VatRate == "5" {
			vatMul = 1.05
		}
		totalObligation := c.Amount * vatMul
		c.DebtAmount = totalObligation - c.TotalPaid
		if c.DebtAmount < 0 {
			c.DebtAmount = 0
		}

		// Is closed?
		if c.ContractType == "onetime" {
			c.IsClosed = c.TotalPaid >= totalObligation && c.TotalPaid > 0
		} else {
			c.IsClosed = c.EndDate != nil && *c.EndDate <= today && c.DebtAmount <= 0
		}

		// Is overdue?
		c.IsOverdue = c.EndDate != nil && *c.EndDate < today && c.DebtAmount > 0

		// Filter closed
		if showClosed != "true" && c.IsClosed {
			continue
		}
		contracts = append(contracts, c)
	}
	if contracts == nil {
		contracts = []Contract{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"contracts": contracts})
}

func (h *PaymentsHandler) CreateContract(w http.ResponseWriter, r *http.Request) {
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
	// Inherit from org if provided
	if body.OrganizationID != nil {
		var orgName, orgAddr, orgContact, orgPhone sql.NullString
		h.DB.QueryRow("SELECT name, address, contact_name, contact_phone FROM organizations WHERE id=$1", *body.OrganizationID).Scan(&orgName, &orgAddr, &orgContact, &orgPhone)
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
	err := h.DB.QueryRow(`INSERT INTO contracts (user_id, organization_id, contract_type, name, company_name,
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
	userID := middleware.GetUserID(r)
	contractID, _ := strconv.Atoi(mux.Vars(r)["id"])
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	sets, args, idx := buildUpdateSets(body, map[string]bool{
		"name": true, "contract_type": true, "organization_id": true, "company_name": true,
		"company_address": true, "amount": true, "vat_rate": true, "contact_name": true,
		"contact_phone": true, "start_date": true, "end_date": true,
	}, 1)
	if len(sets) == 0 { utils.Error(w, http.StatusBadRequest, "NO_FIELDS"); return }
	args = append(args, contractID, userID)
	h.DB.Exec("UPDATE contracts SET "+utils.JoinStrings(sets, ",")+" WHERE id=$"+utils.Itoa(idx)+" AND user_id=$"+utils.Itoa(idx+1), args...)
	utils.Success(w)
}

func (h *PaymentsHandler) DeleteContract(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	contractID, _ := strconv.Atoi(mux.Vars(r)["id"])
	h.DB.Exec("DELETE FROM contracts WHERE id=$1 AND user_id=$2", contractID, userID)
	utils.Success(w)
}

// ─── Invoice ───

func (h *PaymentsHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	contractID, _ := strconv.Atoi(mux.Vars(r)["id"])
	rows, err := h.DB.Query(`SELECT id, amount, vat_rate, issued_at, paid_amount, status
		FROM invoices WHERE contract_id=$1 ORDER BY issued_at DESC`, contractID)
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
	}
	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		if rows.Scan(&inv.ID, &inv.Amount, &inv.VatRate, &inv.IssuedAt, &inv.PaidAmount, &inv.Status) != nil {
			continue
		}
		vatMul := 1.0
		if inv.VatRate == "5" { vatMul = 1.05 }
		inv.Total = inv.Amount * vatMul
		inv.Remainder = inv.Total - inv.PaidAmount
		if inv.Remainder < 0 { inv.Remainder = 0 }
		invoices = append(invoices, inv)
	}
	if invoices == nil { invoices = []Invoice{} }
	utils.JSON(w, http.StatusOK, map[string]interface{}{"invoices": invoices})
}

func (h *PaymentsHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	contractID, _ := strconv.Atoi(mux.Vars(r)["id"])

	var contract struct{ Amount float64; VatRate string; Name string }
	err := h.DB.QueryRow("SELECT amount, vat_rate, name FROM contracts WHERE id=$1 AND user_id=$2", contractID, userID).Scan(&contract.Amount, &contract.VatRate, &contract.Name)
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
	var invoiceID int
	err = h.DB.QueryRow(`INSERT INTO invoices (contract_id, amount, vat_rate, issued_at, paid_amount, status)
		VALUES ($1,$2,$3,$4,0,'unpaid') RETURNING id`, contractID, totalAmount, contract.VatRate, today).Scan(&invoiceID)
	if err != nil { utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED"); return }

	for _, item := range body.Items {
		h.DB.Exec("INSERT INTO invoice_items (invoice_id, name, quantity, price) VALUES ($1,$2,$3,$4)",
			invoiceID, item.Name, item.Quantity, item.Price)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": invoiceID, "success": true})
}

func (h *PaymentsHandler) DeleteInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID, _ := strconv.Atoi(mux.Vars(r)["invoiceId"])
	h.DB.Exec("DELETE FROM invoices WHERE id=$1", invoiceID)
	utils.Success(w)
}

func (h *PaymentsHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID, _ := strconv.Atoi(mux.Vars(r)["invoiceId"])

	var inv struct{ Amount float64; VatRate string; PaidAmount float64 }
	err := h.DB.QueryRow("SELECT amount, vat_rate, paid_amount FROM invoices WHERE id=$1", invoiceID).Scan(&inv.Amount, &inv.VatRate, &inv.PaidAmount)
	if err != nil { utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND"); return }

	vatMul := 1.0
	if inv.VatRate == "5" { vatMul = 1.05 }
	totalObligation := utils.Round2(inv.Amount * vatMul)
	remainder := totalObligation - inv.PaidAmount

	var body struct{ Amount *float64 `json:"amount"`; PaidAt *string `json:"paid_at"` }
	json.NewDecoder(r.Body).Decode(&body)

	payAmount := remainder
	if body.Amount != nil { payAmount = *body.Amount }
	if payAmount > remainder { payAmount = remainder }
	payAmount = utils.Round2(payAmount)

	payDate := time.Now().Format("2006-01-02")
	if body.PaidAt != nil { payDate = *body.PaidAt }

	newPaid := utils.Round2(inv.PaidAmount + payAmount)
	if newPaid > totalObligation { newPaid = totalObligation }
	status := "partial"
	if newPaid >= totalObligation { status = "paid" }

	h.DB.Exec("UPDATE invoices SET paid_amount=$1, status=$2 WHERE id=$3", newPaid, status, invoiceID)
	h.DB.Exec("INSERT INTO contract_payments (invoice_id, amount, paid_at) VALUES ($1,$2,$3)", invoiceID, payAmount, payDate)

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "paid_amount": newPaid, "status": status})
}

func (h *PaymentsHandler) DownloadInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID, _ := strconv.Atoi(mux.Vars(r)["invoiceId"])

	// Get invoice + contract info
	var inv struct {
		ContractID int
		Amount     float64
		VatRate    string
		IssuedAt   string
	}
	err := h.DB.QueryRow("SELECT contract_id, amount, vat_rate, issued_at FROM invoices WHERE id=$1", invoiceID).Scan(
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
	h.DB.QueryRow(`SELECT COALESCE(company_name,''), COALESCE(company_address,''),
		COALESCE(contact_phone,''), COALESCE(contact_name,''), vat_rate, name
		FROM contracts WHERE id=$1`, inv.ContractID).Scan(
		&contract.CompanyName, &contract.CompanyAddress, &contract.ContactPhone,
		&contract.ContactName, &contract.VatRate, &contract.Name)

	// Get invoice items
	rows, err := h.DB.Query("SELECT name, quantity, price FROM invoice_items WHERE invoice_id=$1", invoiceID)
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
	}, items)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DOCX_GENERATION_FAILED")
		return
	}

	// Serve file
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadName))
	http.ServeFile(w, r, outPath)

	// Cleanup after 60 seconds
	go func() {
		time.Sleep(60 * time.Second)
		os.Remove(outPath)
	}()
}

// ─── Statistics ───

func (h *PaymentsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var contractCount int
	var totalAmount, totalInvoiced, totalPaid, totalDebt float64
	var closedCount int

	h.DB.QueryRow("SELECT COUNT(*) FROM contracts WHERE user_id=$1", userID).Scan(&contractCount)
	h.DB.QueryRow("SELECT COALESCE(SUM(amount),0) FROM contracts WHERE user_id=$1", userID).Scan(&totalAmount)

	// Total invoiced
	h.DB.QueryRow(`SELECT COALESCE(SUM(i.amount),0) FROM invoices i
		JOIN contracts c ON i.contract_id=c.id WHERE c.user_id=$1`, userID).Scan(&totalInvoiced)

	// Total paid
	h.DB.QueryRow(`SELECT COALESCE(SUM(p.amount),0) FROM contract_payments p
		JOIN invoices i ON p.invoice_id=i.id JOIN contracts c ON i.contract_id=c.id WHERE c.user_id=$1`, userID).Scan(&totalPaid)

	// Debt = total obligation - total paid (simplified)
	totalDebt = totalInvoiced*1.025 - totalPaid // approximate with VAT
	if totalDebt < 0 { totalDebt = 0 }

	// Closed count (simplified)
	h.DB.QueryRow(`SELECT COUNT(*) FROM contracts WHERE user_id=$1 AND (
		(contract_type='onetime' AND id IN (SELECT contract_id FROM invoices GROUP BY contract_id HAVING SUM(paid_amount) >= SUM(amount)))
		OR (contract_type='service' AND end_date <= CURRENT_DATE)
	)`, userID).Scan(&closedCount)

	stats := []map[string]interface{}{
		{"label": "contracts", "value": contractCount},
		{"label": "total_amount", "value": utils.Round2(totalAmount)},
		{"label": "invoiced", "value": utils.Round2(totalInvoiced)},
		{"label": "debt", "value": utils.Round2(totalDebt), "variant": "danger"},
		{"label": "paid", "value": utils.Round2(totalPaid)},
		{"label": "closed", "value": closedCount},
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"stats": stats})
}

// ─── CSV Export ───

func (h *PaymentsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	rows, err := h.DB.Query(`SELECT c.contract_type, c.name, COALESCE(c.company_name,''), COALESCE(c.company_address,''),
		c.amount, c.vat_rate, COALESCE(c.contact_name,''), COALESCE(c.contact_phone,''),
		COALESCE(c.start_date::text,''), COALESCE(c.end_date::text,'')
		FROM contracts c WHERE c.user_id=$1 ORDER BY c.name`, userID)
	if err != nil { utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED"); return }
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=contracts.csv")
	// BOM for Excel
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	writer.Write([]string{"Тип", "Название", "Организация", "Адрес", "Сумма", "НДС", "Контакт", "Телефон", "Дата начала", "Срок", "Статус"})

	today := time.Now().Format("2006-01-02")
	for rows.Next() {
		var ct, name, company, addr, vat, contact, phone, start, end string
		var amount float64
		rows.Scan(&ct, &name, &company, &addr, &amount, &vat, &contact, &phone, &start, &end)

		typeName := "Сопровождение"
		if ct == "onetime" { typeName = "Разовый" }
		vatLabel := "Без НДС"
		if vat == "5" { vatLabel = "НДС 5%" }

		status := "Активен"
		if end != "" && end <= today { status = "Просрочен" }

		writer.Write([]string{typeName, name, company, addr, fmt.Sprintf("%.2f", amount), vatLabel, contact, phone, start, end, status})
	}
	writer.Flush()
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
