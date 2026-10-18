package migrations

import (
	"encoding/json"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type config000004 struct {
	ID          string    `gorm:"size:64;primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name" validate:"required,max=128"`
	Key         string    `gorm:"column:config_key;size:128;not null;uniqueIndex" json:"key" validate:"required,max=128"`
	Value       string    `gorm:"type:text;not null" json:"value" validate:"max=65535"`
	ValueType   string    `gorm:"size:16;not null" json:"valueType" validate:"required,oneof=string number boolean json"`
	Group       string    `gorm:"column:config_group;size:64;not null" json:"group" validate:"max=64"`
	Description string    `gorm:"size:1000" json:"description" validate:"max=1000"`
	BuiltIn     bool      `json:"builtIn"`
	Sort        int       `json:"sort" validate:"gte=0,lte=1000000"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (config000004) TableName() string { return "sys_configs" }

type configGroup000004 struct {
	ID          string    `gorm:"size:64;primaryKey" json:"id"`
	Code        string    `gorm:"size:64;not null;uniqueIndex" json:"code"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Description string    `gorm:"size:1000" json:"description"`
	Sort        int       `json:"sort"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (configGroup000004) TableName() string { return "sys_config_groups" }

var configGroups000004 = []configGroup000004{
	{ID: "config_group_basic", Code: "basic", Name: "基础配置", Sort: 1},
	{ID: "config_group_security", Code: "security", Name: "安全配置", Sort: 2},
	{ID: "config_group_upload", Code: "upload", Name: "上传配置", Sort: 3},
	{ID: "config_group_notification", Code: "notification", Name: "通知配置", Sort: 4},
}

type dicttype000004 struct {
	ID          string    `gorm:"size:64;primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name" validate:"required,max=128"`
	Code        string    `gorm:"size:128;not null;uniqueIndex" json:"code" validate:"required,max=128"`
	Description string    `gorm:"size:1000" json:"description" validate:"max=1000"`
	Status      string    `gorm:"size:16;not null" json:"status" validate:"required,oneof=enabled disabled"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (dicttype000004) TableName() string { return "sys_dict_types" }

type dictdata000004 struct {
	ID        string    `gorm:"size:64;primaryKey" json:"id"`
	TypeCode  string    `gorm:"size:128;not null;uniqueIndex:idx_dict_value" json:"typeCode" validate:"required,max=128"`
	Label     string    `gorm:"size:128;not null" json:"label" validate:"required,max=128"`
	Value     string    `gorm:"size:128;not null;uniqueIndex:idx_dict_value" json:"value" validate:"required,max=128"`
	Sort      int       `json:"sort" validate:"gte=0,lte=1000000"`
	Status    string    `gorm:"size:16;not null" json:"status" validate:"required,oneof=enabled disabled"`
	Remark    string    `gorm:"size:1000" json:"remark" validate:"max=1000"`
	CreatedAt time.Time `json:"createTime"`
	UpdatedAt time.Time `json:"updateTime"`
}

func (dictdata000004) TableName() string { return "sys_dict_data" }

type file000004 struct {
	ID           string    `gorm:"size:64;primaryKey" json:"id"`
	Name         string    `gorm:"size:255" json:"name"`
	OriginalName string    `gorm:"size:255" json:"originalName"`
	Path         string    `gorm:"size:255" json:"path"`
	Size         int64     `json:"size"`
	MimeType     string    `gorm:"size:128" json:"mimeType"`
	Ext          string    `gorm:"size:32" json:"ext"`
	Storage      string    `gorm:"size:16" json:"storage"`
	Uploader     string    `gorm:"size:64" json:"uploader"`
	Remark       string    `gorm:"size:1000" json:"remark"`
	Data         []byte    `json:"-"`
	CreatedAt    time.Time `json:"createTime"`
}

func (file000004) TableName() string { return "sys_files" }

type log000004 struct {
	ID          string    `gorm:"size:64;primaryKey" json:"id"`
	Kind        string    `gorm:"size:16;index" json:"-"`
	Username    string    `gorm:"size:64;index" json:"username"`
	Module      string    `gorm:"size:128" json:"module"`
	Action      string    `gorm:"size:16" json:"action"`
	Description string    `gorm:"size:255" json:"description"`
	Method      string    `gorm:"size:10" json:"method"`
	URL         string    `gorm:"size:255" json:"url"`
	IP          string    `gorm:"size:64" json:"ip"`
	Browser     string    `gorm:"size:64" json:"browser"`
	OS          string    `gorm:"size:64" json:"os"`
	Status      string    `gorm:"size:16" json:"status"`
	ErrorMsg    string    `gorm:"size:255" json:"errorMsg"`
	Message     string    `gorm:"size:255" json:"message"`
	Duration    int64     `json:"duration"`
	CreatedAt   time.Time `gorm:"index" json:"createTime"`
}

func (log000004) TableName() string { return "sys_logs" }

type profile000004 struct {
	Phone  string `gorm:"size:32;not null;default:''"`
	Gender string `gorm:"size:16;not null;default:''"`
	Bio    string `gorm:"size:200;not null;default:''"`
}

func (profile000004) TableName() string { return "auth_users" }

type route000004 struct {
	Method         string `gorm:"size:10;primaryKey"`
	Route          string `gorm:"size:255;primaryKey"`
	PermissionCode string `gorm:"size:128;not null;index"`
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (route000004) TableName() string { return "route_permissions" }

type permission000004 struct {
	Code       string `gorm:"size:128;primaryKey"`
	Definition string `gorm:"type:text;not null"`
}

func (permission000004) TableName() string { return "auth_permissions" }

type rolePermission000004 struct {
	RoleID         int64  `gorm:"primaryKey"`
	PermissionCode string `gorm:"size:128;primaryKey;index"`
}

func (rolePermission000004) TableName() string { return "auth_role_permissions" }

var managementRoutes000004 = []route000004{
	{Method: "GET", Route: "/api/users", PermissionCode: "system.user.view", Enabled: true},
	{Method: "GET", Route: "/api/users/:id", PermissionCode: "system.user.view", Enabled: true},
	{Method: "POST", Route: "/api/users", PermissionCode: "system.user.create", Enabled: true},
	{Method: "PUT", Route: "/api/users/:id", PermissionCode: "system.user.edit", Enabled: true},
	{Method: "DELETE", Route: "/api/users/:id", PermissionCode: "system.user.delete", Enabled: true},
	{Method: "GET", Route: "/api/roles", PermissionCode: "system.role.view", Enabled: true},
	{Method: "GET", Route: "/api/roles/options", PermissionCode: "system.user.view", Enabled: true},
	{Method: "GET", Route: "/api/roles/:id", PermissionCode: "system.role.view", Enabled: true},
	{Method: "POST", Route: "/api/roles", PermissionCode: "system.role.create", Enabled: true},
	{Method: "PUT", Route: "/api/roles/:id", PermissionCode: "system.role.edit", Enabled: true},
	{Method: "DELETE", Route: "/api/roles/:id", PermissionCode: "system.role.delete", Enabled: true},
	{Method: "GET", Route: "/api/permissions", PermissionCode: "system.permission.view", Enabled: true},
	{Method: "GET", Route: "/api/permissions/tree", PermissionCode: "system.role.view", Enabled: true},
	{Method: "GET", Route: "/api/permissions/:id", PermissionCode: "system.permission.view", Enabled: true},
	{Method: "POST", Route: "/api/permissions", PermissionCode: "system.permission.create", Enabled: true},
	{Method: "PUT", Route: "/api/permissions/:id", PermissionCode: "system.permission.edit", Enabled: true},
	{Method: "DELETE", Route: "/api/permissions/:id", PermissionCode: "system.permission.delete", Enabled: true},
	{Method: "GET", Route: "/api/config/list", PermissionCode: "system.config.view", Enabled: true},
	{Method: "GET", Route: "/api/config/key/:key", PermissionCode: "system.config.view", Enabled: true},
	{Method: "POST", Route: "/api/config", PermissionCode: "system.config.create", Enabled: true},
	{Method: "PUT", Route: "/api/config/:id", PermissionCode: "system.config.edit", Enabled: true},
	{Method: "DELETE", Route: "/api/config/:id", PermissionCode: "system.config.delete", Enabled: true},
	{Method: "GET", Route: "/api/dict/types", PermissionCode: "system.dict.view", Enabled: true},
	{Method: "GET", Route: "/api/dict/type/list", PermissionCode: "system.dict.view", Enabled: true},
	{Method: "GET", Route: "/api/dict/data/all", PermissionCode: "system.dict.view", Enabled: true},
	{Method: "GET", Route: "/api/dict/data/list", PermissionCode: "system.dict.view", Enabled: true},
	{Method: "GET", Route: "/api/dict/data/:typeCode", PermissionCode: "system.dict.view", Enabled: true},
	{Method: "POST", Route: "/api/dict/type", PermissionCode: "system.dict.create", Enabled: true},
	{Method: "PUT", Route: "/api/dict/type/:id", PermissionCode: "system.dict.edit", Enabled: true},
	{Method: "DELETE", Route: "/api/dict/type/:id", PermissionCode: "system.dict.delete", Enabled: true},
	{Method: "POST", Route: "/api/dict/data", PermissionCode: "system.dict.create", Enabled: true},
	{Method: "PUT", Route: "/api/dict/data/:id", PermissionCode: "system.dict.edit", Enabled: true},
	{Method: "DELETE", Route: "/api/dict/data/:id", PermissionCode: "system.dict.delete", Enabled: true},
	{Method: "GET", Route: "/api/file/list", PermissionCode: "system.file.view", Enabled: true},
	{Method: "GET", Route: "/api/file/:id/download", PermissionCode: "system.file.view", Enabled: true},
	{Method: "POST", Route: "/api/file/upload", PermissionCode: "system.file.upload", Enabled: true},
	{Method: "DELETE", Route: "/api/file/:id", PermissionCode: "system.file.delete", Enabled: true},
	{Method: "GET", Route: "/api/log/operation/list", PermissionCode: "system.log.view", Enabled: true},
	{Method: "GET", Route: "/api/log/login/list", PermissionCode: "system.log.view", Enabled: true},
	{Method: "DELETE", Route: "/api/log/operation/clear", PermissionCode: "system.log.clear", Enabled: true},
	{Method: "DELETE", Route: "/api/log/login/clear", PermissionCode: "system.log.clear", Enabled: true},
}

func migration000004Management() *gormigrate.Migration {
	return &gormigrate.Migration{ID: "000004_management", Migrate: func(tx *gorm.DB) error {
		for _, column := range []string{"Phone", "Gender", "Bio"} {
			if err := tx.Migrator().AddColumn(&profile000004{}, column); err != nil {
				return err
			}
		}
		if err := tx.AutoMigrate(&configGroup000004{}, &config000004{}, &dicttype000004{}, &dictdata000004{}, &file000004{}, &log000004{}); err != nil {
			return err
		}
		groups := append([]configGroup000004(nil), configGroups000004...)
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&groups).Error; err != nil {
			return err
		}
		for _, route := range managementRoutes000004 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&route).Error; err != nil {
				return err
			}
		}
		definition, err := json.Marshal(map[string]any{"id": "system.file.upload", "code": "system.file.upload", "name": map[string]string{"zh-CN": "上传文件", "en-US": "Upload file", "ja-JP": "ファイルをアップロード", "ko-KR": "파일 업로드"}, "description": "上传文件", "resource": "system.file", "action": "upload", "type": "button", "parentId": "58", "status": "active", "visible": true})
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&permission000004{Code: "system.file.upload", Definition: string(definition)}).Error; err != nil {
			return err
		}
		var admin struct{ ID int64 }
		if err := tx.Table("auth_roles").Where("code = ?", "admin").Take(&admin).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rolePermission000004{RoleID: admin.ID, PermissionCode: "system.file.upload"}).Error
	}, Rollback: func(tx *gorm.DB) error {
		for _, route := range managementRoutes000004 {
			if err := tx.Where("method = ? AND route = ?", route.Method, route.Route).Delete(&route000004{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("permission_code = ?", "system.file.upload").Delete(&rolePermission000004{}).Error; err != nil {
			return err
		}
		if err := tx.Where("code = ?", "system.file.upload").Delete(&permission000004{}).Error; err != nil {
			return err
		}
		if err := tx.Migrator().DropTable(&log000004{}, &file000004{}, &dictdata000004{}, &dicttype000004{}, &config000004{}, &configGroup000004{}); err != nil {
			return err
		}
		for _, column := range []string{"Bio", "Gender", "Phone"} {
			if err := tx.Migrator().DropColumn(&profile000004{}, column); err != nil {
				return err
			}
		}
		return nil
	}}
}
