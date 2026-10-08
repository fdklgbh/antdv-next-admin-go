package model

import "time"

type Config struct {
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

func (Config) TableName() string { return "sys_configs" }

type DictType struct {
	ID          string    `gorm:"size:64;primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name" validate:"required,max=128"`
	Code        string    `gorm:"size:128;not null;uniqueIndex" json:"code" validate:"required,max=128"`
	Description string    `gorm:"size:1000" json:"description" validate:"max=1000"`
	Status      string    `gorm:"size:16;not null" json:"status" validate:"required,oneof=enabled disabled"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (DictType) TableName() string { return "sys_dict_types" }

type DictData struct {
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

func (DictData) TableName() string { return "sys_dict_data" }

type File struct {
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

func (File) TableName() string { return "sys_files" }

type Log struct {
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

func (Log) TableName() string { return "sys_logs" }
