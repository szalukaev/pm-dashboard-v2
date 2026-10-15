package push

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/hkdf"
)

// Браузер должен суметь расшифровать то, что отправляет сервер (RFC 8291,
// aes128gcm). Тест играет роль браузера: свои ключи подписки, приём запроса
// и расшифровка.
func TestPayloadDecryptsLikeBrowser(t *testing.T) {
	curve := ecdh.P256()
	uaPriv, _ := curve.GenerateKey(rand.Reader)
	authSecret := make([]byte, 16)
	rand.Read(authSecret)
	b64 := base64.RawURLEncoding

	var body []byte
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	vapidPriv, vapidPub, _ := webpush.GenerateVAPIDKeys()
	sub := &webpush.Subscription{Endpoint: srv.URL}
	sub.Keys.P256dh = b64.EncodeToString(uaPriv.PublicKey().Bytes())
	sub.Keys.Auth = b64.EncodeToString(authSecret)
	payload := []byte(`{"title":"Тест","body":"проверка","url":"/notifications"}`)
	resp, err := webpush.SendNotification(payload, sub, &webpush.Options{
		Subscriber: fallbackSubject, VAPIDPublicKey: vapidPub, VAPIDPrivateKey: vapidPriv, TTL: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if headers.Get("Content-Encoding") != "aes128gcm" {
		t.Fatalf("Content-Encoding = %q", headers.Get("Content-Encoding"))
	}

	// Заголовок записи: salt(16) | rs(4) | idlen(1) | keyid(=открытый ключ сервера)
	salt := body[:16]
	idlen := int(body[20])
	asPub, err := curve.NewPublicKey(body[21 : 21+idlen])
	if err != nil {
		t.Fatal(err)
	}
	cipherText := body[21+idlen:]
	shared, _ := uaPriv.ECDH(asPub)

	keyInfo := append(append([]byte("WebPush: info\x00"), uaPriv.PublicKey().Bytes()...), asPub.Bytes()...)
	ikm := make([]byte, 32)
	io.ReadFull(hkdf.New(sha256.New, shared, authSecret, keyInfo), ikm)
	cek := make([]byte, 16)
	io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: aes128gcm\x00")), cek)
	nonce := make([]byte, 12)
	io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: nonce\x00")), nonce)

	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		t.Fatalf("браузер не расшифрует сообщение: %v (rs=%d, len=%d)", err, binary.BigEndian.Uint32(body[16:20]), len(body))
	}
	// Конец данных — разделитель 0x02, дальше нули дополнения
	end := bytes.LastIndexByte(bytes.TrimRight(plain, "\x00"), 0x02)
	if end < 0 || !bytes.Equal(plain[:end], payload) {
		t.Fatalf("расшифровано не то: %q", plain)
	}
}
