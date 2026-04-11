package dto

type SubscribePushRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
	P256dh   string `json:"p256dh" validate:"required"`
	Auth     string `json:"auth" validate:"required"`
}

type UnsubscribePushRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
}

type VAPIDKeysResponse struct {
	PublicKey string `json:"public_key"`
}
