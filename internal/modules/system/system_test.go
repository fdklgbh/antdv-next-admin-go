package system_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"antdv-next-admin-go/internal/config"
	"antdv-next-admin-go/internal/infrastructure/database"
	"antdv-next-admin-go/internal/modules/system/dto"
	"antdv-next-admin-go/internal/modules/system/model"
	"antdv-next-admin-go/internal/modules/system/repository"
	"antdv-next-admin-go/internal/modules/system/service"
	"antdv-next-admin-go/migrations"
)

const systemTestConfigPath = "../../../configs/config.test.yaml"

var (
	systemTestDB  *gorm.DB
	closeSystemDB func() error
	testSequence  atomic.Uint64
)

func TestMain(m *testing.M) {
	testConfigPath := os.Getenv("CONFIG")
	if testConfigPath == "" {
		testConfigPath = systemTestConfigPath
	}
	cfg, err := config.Load(testConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 System 测试配置 %q 失败: %v\n", testConfigPath, err)
		os.Exit(1)
	}
	if cfg.App.Env != "test" || cfg.Server.Mode != "test" {
		fmt.Fprintln(os.Stderr, "System 集成测试要求 app.env 和 server.mode 均为 test")
		os.Exit(1)
	}
	if cfg.Database.Driver != "mysql" {
		fmt.Fprintln(os.Stderr, "System 集成测试要求 database.driver=mysql")
		os.Exit(1)
	}
	dsn, err := mysql.ParseDSN(cfg.Database.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析 System 测试数据库 DSN 失败: %v\n", err)
		os.Exit(1)
	}
	if !strings.HasSuffix(strings.ToLower(dsn.DBName), "_test") {
		fmt.Fprintln(os.Stderr, "System 集成测试要求 MySQL 数据库名以 _test 结尾")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.StartupTimeout)
	db, closeDB, err := database.Open(ctx, cfg.Database)
	if err == nil {
		err = migrations.Migrator(db.WithContext(ctx)).Migrate()
	}
	cancel()
	if err != nil {
		if closeDB != nil {
			_ = closeDB()
		}
		fmt.Fprintf(os.Stderr, "初始化 System 测试数据库失败: %v\n", err)
		os.Exit(1)
	}
	systemTestDB, closeSystemDB = db, closeDB

	exitCode := m.Run()
	if err := closeSystemDB(); err != nil {
		fmt.Fprintf(os.Stderr, "关闭 System 测试数据库失败: %v\n", err)
		exitCode = 1
	}
	os.Exit(exitCode)
}

type systemFixture struct {
	db      *gorm.DB
	service *service.Service
}

func newSystemFixture(t *testing.T) *systemFixture {
	t.Helper()
	tx := systemTestDB.Begin()
	if tx.Error != nil {
		t.Fatalf("开始测试事务: %v", tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
			t.Errorf("回滚测试事务: %v", err)
		}
	})
	return &systemFixture{db: tx, service: service.New(repository.New(tx))}
}

func TestSaveConfigRejectsInvalidTypedValues(t *testing.T) {
	svc := service.New(repository.New(nil))
	tests := []struct {
		name      string
		valueType string
		value     string
	}{
		{name: "invalid number", valueType: "number", value: "NaN"},
		{name: "infinite number", valueType: "number", value: "+Inf"},
		{name: "invalid boolean", valueType: "boolean", value: "yes"},
		{name: "invalid json", valueType: "json", value: `{"missing":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validConfig(test.valueType, test.value)
			if _, err := svc.Save(context.Background(), "", &input); !errors.Is(err, repository.ErrInvalid) {
				t.Fatalf("Save error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestSaveConfigPreservesBuiltInAndRejectsKeyChanges(t *testing.T) {
	fixture := newSystemFixture(t)
	config := validConfig("string", "original")
	config.ID = uniqueValue("config")
	config.BuiltIn = true
	if err := fixture.db.Create(&config).Error; err != nil {
		t.Fatalf("创建内置配置: %v", err)
	}

	updated := validConfig("string", "updated")
	updated.Key = config.Key
	updated.BuiltIn = false
	if _, err := fixture.service.Save(context.Background(), config.ID, &updated); err != nil {
		t.Fatalf("更新内置配置: %v", err)
	}
	var saved model.Config
	if err := fixture.db.First(&saved, "id = ?", config.ID).Error; err != nil {
		t.Fatalf("读取已更新配置: %v", err)
	}
	if !saved.BuiltIn || saved.Value != "updated" {
		t.Fatalf("内置标记或配置值错误: builtIn=%t value=%q", saved.BuiltIn, saved.Value)
	}

	changedKey := validConfig("string", "changed")
	changedKey.Key = uniqueValue("changed_key")
	if _, err := fixture.service.Save(context.Background(), config.ID, &changedKey); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("修改配置 key 的错误 = %v, want ErrInvalid", err)
	}
	if err := fixture.service.Delete(context.Background(), "config", config.ID); !errors.Is(err, repository.ErrProtected) {
		t.Fatalf("删除内置配置的错误 = %v, want ErrProtected", err)
	}
}

func TestDictionaryReferencesAndTypeIdentityAreProtected(t *testing.T) {
	fixture := newSystemFixture(t)
	ctx := context.Background()
	firstType := validDictType("first")
	if _, err := fixture.service.Save(ctx, "", &firstType); err != nil {
		t.Fatalf("创建字典类型: %v", err)
	}
	secondType := validDictType("second")
	if _, err := fixture.service.Save(ctx, "", &secondType); err != nil {
		t.Fatalf("创建第二个字典类型: %v", err)
	}
	data := validDictData(firstType.Code)
	if _, err := fixture.service.Save(ctx, "", &data); err != nil {
		t.Fatalf("创建字典数据: %v", err)
	}

	if err := fixture.service.Delete(ctx, "dict_type", firstType.ID); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("删除仍有关联数据的类型错误 = %v, want ErrConflict", err)
	}
	changed := data
	changed.TypeCode = secondType.Code
	if _, err := fixture.service.Save(ctx, data.ID, &changed); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("修改字典数据所属类型错误 = %v, want ErrInvalid", err)
	}
	var saved model.DictData
	if err := fixture.db.First(&saved, "id = ?", data.ID).Error; err != nil {
		t.Fatalf("读取字典数据: %v", err)
	}
	if saved.TypeCode != firstType.Code {
		t.Fatalf("字典数据所属类型被修改: got %q, want %q", saved.TypeCode, firstType.Code)
	}
}

func TestEnabledDictionaryListExcludesDisabledTypes(t *testing.T) {
	fixture := newSystemFixture(t)
	ctx := context.Background()
	enabledType := validDictType("enabled")
	disabledType := validDictType("disabled")
	disabledType.Status = "disabled"
	for _, row := range []*model.DictType{&enabledType, &disabledType} {
		if _, err := fixture.service.Save(ctx, "", row); err != nil {
			t.Fatalf("创建字典类型 %q: %v", row.Code, err)
		}
	}
	for _, row := range []model.DictData{validDictData(enabledType.Code), validDictData(disabledType.Code)} {
		if _, err := fixture.service.Save(ctx, "", &row); err != nil {
			t.Fatalf("创建字典数据: %v", err)
		}
	}

	page, err := fixture.service.List(ctx, "dict_data", dto.Query{Page: 1, PageSize: 100, Status: "enabled"}, true)
	if err != nil {
		t.Fatalf("查询启用字典: %v", err)
	}
	rows, ok := page.List.(*[]model.DictData)
	if !ok {
		t.Fatalf("列表类型错误: %T", page.List)
	}
	if len(*rows) != 1 || (*rows)[0].TypeCode != enabledType.Code {
		t.Fatalf("启用字典列表包含错误数据: %+v", *rows)
	}
}

func TestUploadNormalizesFilenameAndPersistsDownloadData(t *testing.T) {
	fixture := newSystemFixture(t)
	content := []byte("file content")
	file, err := fixture.service.Upload(context.Background(), `C:\\temp\\report.TXT`, "tester", content)
	if err != nil {
		t.Fatalf("上传文件: %v", err)
	}
	if file.OriginalName != "report.TXT" || file.Ext != "txt" || file.Size != int64(len(content)) || file.MimeType != http.DetectContentType(content) {
		t.Fatalf("文件元数据错误: %+v", file)
	}
	if file.Path != "/api/file/"+file.ID+"/download" {
		t.Fatalf("下载路径错误: %q", file.Path)
	}
	saved, err := fixture.service.File(context.Background(), file.ID)
	if err != nil {
		t.Fatalf("读取文件: %v", err)
	}
	if string(saved.Data) != string(content) {
		t.Fatalf("读取到的文件内容错误: %q", saved.Data)
	}
}

func TestUploadRejectsInvalidSizeAndName(t *testing.T) {
	fixture := newSystemFixture(t)
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "empty file", data: nil},
		{name: "empty filename", data: []byte("x")},
		{name: "oversized extension", data: []byte("x")},
		{name: "oversized content", data: make([]byte, 10*1024*1024+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := "file.txt"
			if test.name == "empty filename" {
				name = ""
			}
			if test.name == "oversized extension" {
				name = "file." + strings.Repeat("x", 33)
			}
			if _, err := fixture.service.Upload(context.Background(), name, "tester", test.data); !errors.Is(err, repository.ErrInvalid) {
				t.Fatalf("Upload error = %v, want ErrInvalid", err)
			}
		})
	}
}

func validConfig(valueType, value string) model.Config {
	return model.Config{Name: "Test config", Key: uniqueValue("config_key"), Value: value, ValueType: valueType, Group: "test", Sort: 0}
}

func validDictType(prefix string) model.DictType {
	code := uniqueValue(prefix)
	return model.DictType{ID: uniqueValue("dict_type"), Name: "Test dictionary", Code: code, Status: "enabled"}
}

func validDictData(typeCode string) model.DictData {
	return model.DictData{ID: uniqueValue("dict_data"), TypeCode: typeCode, Label: "Test item", Value: uniqueValue("value"), Status: "enabled"}
}

func uniqueValue(prefix string) string {
	return fmt.Sprintf("%s_%x_%x", prefix, time.Now().UnixNano(), testSequence.Add(1))
}
