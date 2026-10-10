package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	basecache "antdv-next-admin-go/internal/infrastructure/cache"
	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
)

const accessLifetime = 15 * time.Minute

type accessClaims struct {
	Username  string `json:"username"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func (s *Service) issue(session model.RefreshSession) (dto.LoginResult, error) {
	now := time.Now()
	claims := accessClaims{Username: session.Principal.Username, SessionID: session.SessionID,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: s.authConfig.JWTIssuer, Audience: jwt.ClaimStrings{s.authConfig.JWTAudience},
			Subject: strconv.FormatInt(session.Principal.UserID, 10), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(accessLifetime))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.authConfig.JWTSecret))
	if err != nil {
		return dto.LoginResult{}, fmt.Errorf("sign access JWT: %w", err)
	}
	return dto.LoginResult{Token: token, ExpiresIn: int(accessLifetime.Seconds()), SessionID: session.SessionID, Remember: session.Remember}, nil
}
func (s *Service) Authenticate(_ context.Context, raw string) (model.Principal, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (any, error) { return []byte(s.authConfig.JWTSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.authConfig.JWTIssuer), jwt.WithAudience(s.authConfig.JWTAudience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.Username == "" || claims.SessionID == "" || claims.IssuedAt == nil {
		return model.Principal{}, model.ErrUnauthenticated
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return model.Principal{}, model.ErrUnauthenticated
	}
	return model.Principal{UserID: id, Username: claims.Username}, nil
}
func (s *Service) Login(ctx context.Context, input dto.LoginRequest, previous string) (dto.LoginResult, string, time.Time, error) {
	user, err := s.repo.FindUser(ctx, input.Username)
	if err != nil {
		return dto.LoginResult{}, "", time.Time{}, err
	}
	if !user.Enabled || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return dto.LoginResult{}, "", time.Time{}, model.ErrUnauthenticated
	}
	ttl := 24 * time.Hour
	if input.Remember {
		if input.RememberDays != 7 && input.RememberDays != 15 && input.RememberDays != 30 {
			return dto.LoginResult{}, "", time.Time{}, fmt.Errorf("invalid remember duration")
		}
		ttl = time.Duration(input.RememberDays) * 24 * time.Hour
	}
	refresh, err := randomToken()
	if err != nil {
		return dto.LoginResult{}, "", time.Time{}, err
	}
	sid, err := randomToken()
	if err != nil {
		return dto.LoginResult{}, "", time.Time{}, err
	}
	session := model.RefreshSession{Principal: model.Principal{UserID: user.ID, Username: user.Username}, SessionID: sid, ExpiresAt: time.Now().Add(ttl), Remember: input.Remember}
	result, err := s.issue(session)
	if err != nil {
		return dto.LoginResult{}, "", time.Time{}, err
	}
	if previous != "" {
		if err := s.sessions.DeleteRefresh(ctx, previous); err != nil {
			return dto.LoginResult{}, "", time.Time{}, fmt.Errorf("revoke previous session: %w", err)
		}
	}
	if err := s.sessions.SetRefresh(ctx, refresh, session, ttl); err != nil {
		return dto.LoginResult{}, "", time.Time{}, fmt.Errorf("save refresh session: %w", err)
	}
	return result, refresh, session.ExpiresAt, nil
}
func (s *Service) Refresh(ctx context.Context, refresh string) (dto.LoginResult, error) {
	if refresh == "" {
		return dto.LoginResult{}, model.ErrUnauthenticated
	}
	session, err := s.sessions.GetRefresh(ctx, refresh)
	if errors.Is(err, basecache.ErrMiss) {
		return dto.LoginResult{}, model.ErrUnauthenticated
	}
	if err != nil {
		return dto.LoginResult{}, fmt.Errorf("read refresh session: %w", err)
	}
	if !time.Now().Before(session.ExpiresAt) {
		return dto.LoginResult{}, model.ErrUnauthenticated
	}
	user, err := s.repo.UserByID(ctx, session.Principal.UserID)
	if err != nil {
		return dto.LoginResult{}, err
	}
	if !user.Enabled {
		return dto.LoginResult{}, model.ErrUnauthenticated
	}
	// 刷新只读取会话；并发刷新不会轮换凭证，也不会延长绝对有效期。
	session.Principal.Username = user.Username
	return s.issue(session)
}
func (s *Service) Logout(ctx context.Context, refresh string) error {
	if refresh == "" {
		return nil
	}
	if err := s.sessions.DeleteRefresh(ctx, refresh); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	return nil
}
func (s *Service) UserInfo(ctx context.Context, id int64) (dto.UserInfo, error) {
	user, err := s.repo.UserByID(ctx, id)
	if err != nil {
		return dto.UserInfo{}, err
	}
	if !user.Enabled {
		return dto.UserInfo{}, model.ErrUnauthenticated
	}
	codes, err := s.repo.PermissionCodes(ctx, id)
	if err != nil {
		return dto.UserInfo{}, err
	}
	roles, permissions, err := s.userAuthorization(ctx, user, codes)
	if err != nil {
		return dto.UserInfo{}, err
	}
	return dto.UserInfo{ID: strconv.FormatInt(user.ID, 10), Username: user.Username, Email: user.Email, Phone: user.Phone, Gender: user.Gender, Bio: user.Bio, DisplayName: user.DisplayName, Status: "active", CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, Roles: roles, Permissions: permissions}, nil
}
