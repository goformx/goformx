package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderDeliveryID = "X-GoFormX-Delivery-ID"
	HeaderTimestamp  = "X-GoFormX-Timestamp"
	HeaderSignature  = "X-GoFormX-Signature"
	signatureVersion = "v1="
)

func Sign(secret, deliveryID, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(deliveryID))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return signatureVersion + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret, deliveryID, timestamp, signature string, body []byte, now time.Time, tolerance time.Duration) error {
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid webhook timestamp")
	}
	signedAt := time.Unix(unix, 0)
	if signedAt.Before(now.Add(-tolerance)) || signedAt.After(now.Add(tolerance)) {
		return errors.New("webhook timestamp is outside the replay window")
	}
	if !strings.HasPrefix(signature, signatureVersion) {
		return errors.New("invalid webhook signature")
	}
	expected := Sign(secret, deliveryID, timestamp, body)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) != 1 {
		return errors.New("invalid webhook signature")
	}
	return nil
}
