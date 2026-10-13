package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"pm-dashboard/mail"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// MailHandler — настройки отправки почты у администратора.
type MailHandler struct {
	Mail *mail.Store
}

// smtpView — настройки, как их видит интерфейс: без пароля, только признак,
// что он задан.
type smtpView struct {
	mail.Config
	HasPassword bool `json:"has_password"`
}

// GetSMTP возвращает настройки отправки почты.
func (h *MailHandler) GetSMTP(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.Mail.Load()
	if err != nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	view := smtpView{Config: cfg, HasPassword: cfg.PasswordEnc != ""}
	view.PasswordEnc = ""
	utils.JSON(w, http.StatusOK, view)
}

// UpdateSMTP сохраняет настройки. Пароль меняется, только если прислан:
// поле password отсутствует — прежний остаётся, пустая строка — убирается.
func (h *MailHandler) UpdateSMTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		mail.Config
		Password *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	cfg := body.Config
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.From = strings.TrimSpace(cfg.From)
	cfg.FromName = strings.TrimSpace(cfg.FromName)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.PasswordEnc = ""
	// Выключенную отправку можно сохранить и недозаполненной
	if code := cfg.Validate(); code != "" && (cfg.Enabled || cfg.Host != "") {
		utils.Error(w, http.StatusBadRequest, code)
		return
	}
	if err := h.Mail.Save(cfg, body.Password); err != nil {
		slog.Error("Could not save the mail settings", "error", err)
		utils.Error(w, http.StatusInternalServerError, "CONFIG_SAVE_FAILED")
		return
	}
	slog.Info("Mail settings changed", "user", middleware.GetUserID(r), "host", cfg.Host, "enabled", cfg.Enabled)
	utils.Success(w)
}

// TestSMTP отправляет пробное письмо на указанный адрес по сохранённым
// настройкам и возвращает ответ сервера, если отправка не удалась.
func (h *MailHandler) TestSMTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	to := strings.TrimSpace(body.To)
	if !mail.ValidAddress(to) {
		utils.Error(w, http.StatusBadRequest, "INVALID_EMAIL")
		return
	}
	cfg, err := h.Mail.Load()
	if err != nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	// Проверить можно и выключенную отправку — для того она и проверка
	cfg.Enabled = true
	if code := cfg.Validate(); code != "" {
		utils.Error(w, http.StatusBadRequest, "SMTP_NOT_CONFIGURED")
		return
	}
	html := `<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px">` +
		`<p>Это пробное письмо PM Dashboard: отправка почты настроена верно.</p>` +
		`<p>This is a test message from PM Dashboard: mail sending is set up correctly.</p></div>`
	if err := mail.Send(cfg, to, "PM Dashboard: проверка почты", html); err != nil {
		slog.Warn("Test mail failed", "user", middleware.GetUserID(r), "error", err)
		utils.JSON(w, http.StatusBadGateway, map[string]interface{}{"error": "MAIL_SEND_FAILED", "detail": err.Error()})
		return
	}
	utils.Success(w)
}
