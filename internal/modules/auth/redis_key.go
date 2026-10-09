package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

func sessionKey(prefix, token string) string {
	digest := sha256.Sum256([]byte(token))
	return cacheKey(prefix, "auth:refresh:"+hex.EncodeToString(digest[:]))
}
func permissionRoutesKey(prefix string, userID int64) string {
	return cacheKey(prefix, "auth:permission_routes:"+strconv.FormatInt(userID, 10))
}
func cacheKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + ":" + key
}
