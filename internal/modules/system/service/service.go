package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"antdv-next-admin-go/internal/modules/system/dto"
	"antdv-next-admin-go/internal/modules/system/model"
	"antdv-next-admin-go/internal/modules/system/repository"
)

type Service struct {
	repo     *repository.Repository
	validate *validator.Validate
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo, validate: validator.New()}
}
func identifier() string { var id [16]byte; rand.Read(id[:]); return hex.EncodeToString(id[:]) }
func (s *Service) List(ctx context.Context, kind string, q dto.Query, all bool) (dto.Page, error) {
	return s.repo.List(ctx, kind, q, all)
}
func (s *Service) ConfigByKey(ctx context.Context, key string) (model.Config, error) {
	return s.repo.ConfigByKey(ctx, key)
}
func (s *Service) Save(ctx context.Context, id string, input any) (any, error) {
	if err := s.validate.Struct(input); err != nil {
		return nil, repository.ErrInvalid
	}
	freshID := id
	if freshID == "" {
		freshID = identifier()
	}
	switch row := input.(type) {
	case *model.Config:
		if strings.TrimSpace(row.Key) == "" || strings.TrimSpace(row.Name) == "" {
			return nil, repository.ErrInvalid
		}
		switch row.ValueType {
		case "number":
			v, err := strconv.ParseFloat(row.Value, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, repository.ErrInvalid
			}
		case "boolean":
			if row.Value != "true" && row.Value != "false" {
				return nil, repository.ErrInvalid
			}
		case "json":
			if !json.Valid([]byte(row.Value)) {
				return nil, repository.ErrInvalid
			}
		}
		row.ID = freshID
		row.CreatedAt = time.Time{}
		row.UpdatedAt = time.Time{}
	case *model.DictType:
		row.ID = freshID
		row.CreatedAt = time.Time{}
		row.UpdatedAt = time.Time{}
	case *model.DictData:
		row.ID = freshID
		row.CreatedAt = time.Time{}
		row.UpdatedAt = time.Time{}
	default:
		return nil, repository.ErrInvalid
	}
	if err := s.repo.Save(ctx, id, input); err != nil {
		return nil, err
	}
	return input, nil
}
func (s *Service) Delete(ctx context.Context, kind, id string) error {
	return s.repo.Delete(ctx, kind, id)
}
func (s *Service) ClearLogs(ctx context.Context, kind string) error {
	return s.repo.ClearLogs(ctx, kind)
}
func (s *Service) Upload(ctx context.Context, name, uploader string, data []byte) (model.File, error) {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || strings.TrimSpace(name) == "" || len(name) > 255 || len(data) == 0 || len(data) > 10*1024*1024 {
		return model.File{}, repository.ErrInvalid
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if len(ext) > 32 {
		return model.File{}, repository.ErrInvalid
	}
	id := identifier()
	row := model.File{ID: id, Name: id, OriginalName: name, Path: "/api/file/" + id + "/download", Size: int64(len(data)), MimeType: http.DetectContentType(data), Ext: ext, Storage: "database", Uploader: uploader, Data: data}
	err := s.repo.CreateFile(ctx, &row)
	return row, err
}
func (s *Service) File(ctx context.Context, id string) (model.File, error) {
	return s.repo.File(ctx, id)
}
func (s *Service) Audit(ctx context.Context, row model.Log) error {
	row.ID = identifier()
	return s.repo.Audit(ctx, &row)
}
