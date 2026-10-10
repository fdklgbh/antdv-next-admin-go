package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"antdv-next-admin-go/internal/modules/system/dto"
	"antdv-next-admin-go/internal/modules/system/model"
)

var ErrInvalid = errors.New("invalid input")
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("resource exists or is referenced")
var ErrProtected = errors.New("built-in resource")

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }
func wrap(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("system repository: %w", err)
	}
	return nil
}
func (r *Repository) List(ctx context.Context, kind string, q dto.Query, all bool) (dto.Page, error) {
	db := r.db.WithContext(ctx)
	var rows any
	switch kind {
	case "config":
		rows = &[]model.Config{}
		db = db.Model(&model.Config{})
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Key != "" {
			db = db.Where("config_key LIKE ?", "%"+q.Key+"%")
		}
		if q.Group != "" {
			db = db.Where("config_group = ?", q.Group)
		}
		db = db.Order("sort, id")
	case "dict_type":
		rows = &[]model.DictType{}
		db = db.Model(&model.DictType{})
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Code != "" {
			db = db.Where("code LIKE ?", "%"+q.Code+"%")
		}
		if q.Status != "" {
			db = db.Where("status = ?", q.Status)
		}
		db = db.Order("created_at, id")
	case "dict_data":
		rows = &[]model.DictData{}
		db = db.Model(&model.DictData{})
		if q.TypeCode != "" {
			db = db.Where("type_code = ?", q.TypeCode)
		}
		if q.Label != "" {
			db = db.Where("label LIKE ?", "%"+q.Label+"%")
		}
		if q.Value != "" {
			db = db.Where("value LIKE ?", "%"+q.Value+"%")
		}
		if q.Status != "" {
			db = db.Where("status = ?", q.Status)
			if all && q.Status == "enabled" {
				db = db.Where("type_code IN (?)", r.db.Model(&model.DictType{}).Select("code").Where("status = ?", "enabled"))
			}
		}
		db = db.Order("sort, id")
	case "file":
		rows = &[]model.File{}
		db = db.Model(&model.File{}).Omit("data")
		if q.Name != "" {
			db = db.Where("original_name LIKE ?", "%"+q.Name+"%")
		}
		if q.Ext != "" {
			db = db.Where("ext = ?", q.Ext)
		}
		if q.Storage != "" {
			db = db.Where("storage = ?", q.Storage)
		}
		db = db.Order("created_at DESC, id")
	case "login", "operation":
		rows = &[]model.Log{}
		db = db.Model(&model.Log{}).Where("kind = ?", kind)
		for column, value := range map[string]string{"username": q.Username, "module": q.Module, "action": q.Action, "status": q.Status, "ip": q.IP} {
			if value != "" {
				db = db.Where(column+" = ?", value)
			}
		}
		for _, bound := range []struct{ value, op string }{{q.StartTime, ">="}, {q.EndTime, "<="}} {
			if bound.value != "" {
				t, err := parseTime(bound.value)
				if err != nil {
					return dto.Page{}, ErrInvalid
				}
				db = db.Where("created_at "+bound.op+" ?", t)
			}
		}
		db = db.Order("created_at DESC, id")
	default:
		return dto.Page{}, ErrInvalid
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return dto.Page{}, wrap(err)
	}
	if !all {
		db = db.Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize)
	}
	if err := db.Find(rows).Error; err != nil {
		return dto.Page{}, wrap(err)
	}
	return dto.Page{List: rows, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}
func parseTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, ErrInvalid
}
func (r *Repository) ConfigByKey(ctx context.Context, key string) (model.Config, error) {
	var row model.Config
	err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&row).Error
	return row, wrap(err)
}
func (r *Repository) Save(ctx context.Context, id string, input any) error {
	return wrap(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 字典类型与数据共用固定写锁，防止校验与删除/改码之间出现竞态。
		var lock struct{ ID int64 }
		if err := tx.Table("permission_write_locks").Clauses(clause.Locking{Strength: "UPDATE"}).First(&lock, "id = ?", 1).Error; err != nil {
			return err
		}
		switch row := input.(type) {
		case *model.Config:
			if id == "" {
				row.BuiltIn = false
				return tx.Create(row).Error
			}
			var old model.Config
			if err := tx.First(&old, "id = ?", id).Error; err != nil {
				return err
			}
			if old.Key != row.Key {
				return ErrInvalid
			}
			row.ID = id
			row.BuiltIn = old.BuiltIn
			row.CreatedAt = old.CreatedAt
			return tx.Save(row).Error
		case *model.DictType:
			if id == "" {
				return tx.Create(row).Error
			}
			var old model.DictType
			if err := tx.First(&old, "id = ?", id).Error; err != nil {
				return err
			}
			if old.Code != row.Code {
				return ErrInvalid
			}
			row.ID = id
			row.CreatedAt = old.CreatedAt
			return tx.Save(row).Error
		case *model.DictData:
			var parent model.DictType
			if err := tx.Where("code = ?", row.TypeCode).First(&parent).Error; err != nil {
				return err
			}
			if id == "" {
				return tx.Create(row).Error
			}
			var old model.DictData
			if err := tx.First(&old, "id = ?", id).Error; err != nil {
				return err
			}
			if old.TypeCode != row.TypeCode {
				return ErrInvalid
			}
			row.ID = id
			row.CreatedAt = old.CreatedAt
			return tx.Save(row).Error
		default:
			return ErrInvalid
		}
	}))
}
func (r *Repository) Delete(ctx context.Context, kind, id string) error {
	return wrap(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lock struct{ ID int64 }
		if err := tx.Table("permission_write_locks").Clauses(clause.Locking{Strength: "UPDATE"}).First(&lock, "id = ?", 1).Error; err != nil {
			return err
		}
		switch kind {
		case "config":
			var row model.Config
			if err := tx.First(&row, "id = ?", id).Error; err != nil {
				return err
			}
			if row.BuiltIn {
				return ErrProtected
			}
			return tx.Delete(&row).Error
		case "dict_type":
			var row model.DictType
			if err := tx.First(&row, "id = ?", id).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.DictData{}).Where("type_code = ?", row.Code).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrConflict
			}
			return tx.Delete(&row).Error
		case "dict_data":
			var row model.DictData
			if err := tx.First(&row, "id = ?", id).Error; err != nil {
				return err
			}
			return tx.Delete(&row).Error
		case "file":
			result := tx.Where("id = ?", id).Delete(&model.File{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrNotFound
			}
			return nil
		default:
			return ErrInvalid
		}
	}))
}
func (r *Repository) File(ctx context.Context, id string) (model.File, error) {
	var row model.File
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	return row, wrap(err)
}
func (r *Repository) CreateFile(ctx context.Context, row *model.File) error {
	return wrap(r.db.WithContext(ctx).Create(row).Error)
}
func (r *Repository) Audit(ctx context.Context, row *model.Log) error {
	return wrap(r.db.WithContext(ctx).Create(row).Error)
}
func (r *Repository) ClearLogs(ctx context.Context, kind string) error {
	return wrap(r.db.WithContext(ctx).Where("kind = ?", kind).Delete(&model.Log{}).Error)
}
