package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

func sessionKey(app, token string) string {
	digest := sha256.Sum256([]byte(token))
	return app + ":auth:session:" + hex.EncodeToString(digest[:])
}
func permissionRoutesKey(app string, userID int64) string {
	return app + ":auth:permission_routes:" + strconv.FormatInt(userID, 10)
}
