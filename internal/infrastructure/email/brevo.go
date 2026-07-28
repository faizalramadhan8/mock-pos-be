package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Brevo (Sendinblue) transactional email client — sederhana, cukup untuk
// OTP + order confirmation. Docs: https://developers.brevo.com/reference/sendtransacemail
//
// Kalau APIKey kosong = dev mode: log ke stdout, tidak beneran kirim email.
// Return nil error di kedua mode supaya caller tidak crash.

type Client struct {
	APIKey      string
	SenderEmail string
	SenderName  string
	http        *http.Client
}

func NewClient(apiKey, senderEmail, senderName string) *Client {
	if senderName == "" {
		senderName = "TBK Santi"
	}
	if senderEmail == "" {
		senderEmail = "noreply@tbksanti.id"
	}
	return &Client{
		APIKey:      apiKey,
		SenderEmail: senderEmail,
		SenderName:  senderName,
		http:        &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) IsConfigured() bool { return c.APIKey != "" }

type sendReq struct {
	Sender      sender      `json:"sender"`
	To          []recipient `json:"to"`
	Subject     string      `json:"subject"`
	HTMLContent string      `json:"htmlContent"`
	TextContent string      `json:"textContent,omitempty"`
}

type sender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
type recipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Send — 1 email 1 recipient. Blocking call max 20s (per Timeout).
// Caller SHOULD run di goroutine kalau tidak boleh block HTTP request.
func (c *Client) Send(toEmail, toName, subject, htmlContent, textContent string) error {
	if !c.IsConfigured() {
		// Dev/stub — log ke stdout supaya developer bisa lihat isi email tanpa
		// setup Brevo. Untuk OTP, angka masuk log — developer copy manual.
		fmt.Printf("[email:stub] to=%s subject=%q\n--- BODY ---\n%s\n--- END ---\n", toEmail, subject, textContent)
		return nil
	}

	body, _ := json.Marshal(sendReq{
		Sender:      sender{Name: c.SenderName, Email: c.SenderEmail},
		To:          []recipient{{Email: toEmail, Name: toName}},
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: textContent,
	})

	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("brevo error (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}
