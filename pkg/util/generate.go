package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateRandomCode(length int) string {
	code := make([]byte, length)
	for i := 0; i < length; i++ {
		code[i] = charset[rng.Intn(len(charset))]
	}
	return string(code)
}

func GenerateXIDWithPrefix(prefix string) string {
	const xidLength = 10
	xid := GenerateRandomString("1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ", xidLength)
	return fmt.Sprintf("%s%s", prefix, xid)
}

func GenerateXID() string {
	const xidLength = 16
	xid := GenerateRandomString("1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ", xidLength)
	return xid
}

func GenerateSHA256(salt, word string) string {
	payload := fmt.Sprint(salt, word)
	h := sha256.New()
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func GenerateRandomString(chars string, strlen int) string {
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = chars[rng.Intn(len(chars))]
	}
	return string(result)
}

func GenerateOTP() string {
	return GenerateRandomString("1234567890", 6)
}

func GenerateGuestNumber() string {
	return GenerateRandomString("1234567890", 6)
}

func GenerateQRCode() string {
	return GenerateRandomString("1234567890ABCDEFGHIJKLMNOPQRSTUVXYZ", 20)
}

func UniqueStrings(strings []string) []string {
	uniqueMap := make(map[string]bool)
	var uniqueStrings []string

	for _, str := range strings {
		if !uniqueMap[str] {
			uniqueMap[str] = true
			uniqueStrings = append(uniqueStrings, str)
		}
	}

	return uniqueStrings
}

func MaskingString(stringData string) string {
	lastFour := stringData[len(stringData)-4:]
	masked := strings.Repeat("*", len(stringData)-4) + lastFour

	return masked
}

func HideName(name string) string {
	parts := strings.Split(name, " ")
	for i := range parts {
		if len(parts[i]) > 2 {
			parts[i] = parts[i][:1] + strings.Repeat("*", len(parts[i])-1)
		}
	}
	return strings.Join(parts, " ")
}

func GenerateMd5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func GenerateUUID() string {
	return uuid.New().String()
}

func HashHMACSHA512(salt string, word ...any) string {
	payload := fmt.Sprint(word...)
	h := hmac.New(sha512.New, []byte(salt))
	h.Write([]byte(payload))
	hashed := hex.EncodeToString(h.Sum(nil))
	return hashed
}

func GenerateSignature(apiKey, apiSecret, referenceNum, timestamp string) string {
	data := fmt.Sprintf("%s|%s|%s", timestamp, apiKey, referenceNum)
	return HashHMACSHA512(apiSecret, data)
}

func ValidateSignature(secret string, signature string, payload []string) bool {
	originSignature := strings.Join(payload, "|")
	hashedSignature := HashHMACSHA512(secret, originSignature)
	return strings.Compare(hashedSignature, signature) == 0
}
