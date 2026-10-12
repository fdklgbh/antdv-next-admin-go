package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"antdv-next-admin-go/internal/config"
	basecache "antdv-next-admin-go/internal/infrastructure/cache"
	"antdv-next-admin-go/internal/infrastructure/database"
	authcache "antdv-next-admin-go/internal/modules/auth/cache"
	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/modules/auth/repository"
	"antdv-next-admin-go/internal/modules/auth/service"
	"antdv-next-admin-go/migrations"
)

const authTestConfigPath = "../../../configs/config.test.yaml"

var (
	authTestConfig  config.Config
	authTestDB      *gorm.DB
	closeAuthDB     func() error
	fixtureSequence atomic.Uint64
)

func TestMain(m *testing.M) {
	testConfigPath := os.Getenv("CONFIG")
	if testConfigPath == "" {
		testConfigPath = authTestConfigPath
	}
	cfg, err := config.Load(testConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 Auth 测试配置 %q 失败: %v\n", testConfigPath, err)
		os.Exit(1)
	}
	if cfg.App.Env != "test" || cfg.Server.Mode != "test" {
		fmt.Fprintln(os.Stderr, "Auth 集成测试要求 app.env 和 server.mode 均为 test")
		os.Exit(1)
	}
	if cfg.Database.Driver != "mysql" {
		fmt.Fprintln(os.Stderr, "Auth 集成测试要求 database.driver=mysql")
		os.Exit(1)
	}
	mysqlConfig, err := mysql.ParseDSN(cfg.Database.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析 Auth 测试数据库 DSN 失败: %v\n", err)
		os.Exit(1)
	}
	if !strings.HasSuffix(strings.ToLower(mysqlConfig.DBName), "_test") {
		fmt.Fprintln(os.Stderr, "Auth 集成测试要求 MySQL 数据库名以 _test 结尾")
		os.Exit(1)
	}
	authTestConfig = cfg

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
		fmt.Fprintf(os.Stderr, "初始化 Auth 测试数据库失败: %v\n", err)
		os.Exit(1)
	}
	authTestDB = db
	closeAuthDB = closeDB

	exitCode := m.Run()
	if err := closeAuthDB(); err != nil {
		fmt.Fprintf(os.Stderr, "关闭 Auth 测试数据库失败: %v\n", err)
		exitCode = 1
	}
	os.Exit(exitCode)
}

type authFixture struct {
	db        *gorm.DB
	service   *service.Service
	actor     model.User
	superRole model.Role
}

func newAuthFixture(t *testing.T, queryLog *selectQueryCounter) *authFixture {
	t.Helper()
	db := authTestDB
	if queryLog != nil {
		db = db.Session(&gorm.Session{Logger: queryLog})
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开始测试事务: %v", tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
			t.Errorf("回滚测试事务: %v", err)
		}
	})

	var superRole model.Role
	if err := tx.Where("code = ?", "super_admin").First(&superRole).Error; err != nil {
		t.Fatalf("查询超级管理员角色: %v", err)
	}
	actor := model.User{
		Username:     uniqueTestValue("actor"),
		PasswordHash: "test-only-hash",
		DisplayName:  "Test Actor",
		Enabled:      true,
	}
	if err := tx.Create(&actor).Error; err != nil {
		t.Fatalf("创建测试操作者: %v", err)
	}
	if err := tx.Create(&model.UserRole{UserID: actor.ID, RoleID: superRole.ID}).Error; err != nil {
		t.Fatalf("授予测试操作者角色: %v", err)
	}

	store := basecache.NewMemoryStore()
	permissions := authcache.NewStoreCache(
		store,
		func(token string) string { return "auth:test:session:" + token },
		func(userID int64) string { return fmt.Sprintf("auth:test:permissions:%d", userID) },
	)
	repo := repository.New(tx)
	return &authFixture{
		db:        tx,
		service:   service.New(repo, nil, permissions, authTestConfig.Auth),
		actor:     actor,
		superRole: superRole,
	}
}

func TestSavePermissionPreservesParentAndBuildsTree(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()
	root, err := fixture.service.SavePermission(ctx, "", testMenu(uniqueTestValue("root"), nil))
	if err != nil {
		t.Fatalf("创建根菜单: %v", err)
	}
	parentID := root.ID
	child, err := fixture.service.SavePermission(ctx, "", testMenu(uniqueTestValue("child"), &parentID))
	if err != nil {
		t.Fatalf("创建子菜单: %v", err)
	}

	updatedInput := child
	updatedInput.Name = json.RawMessage(`"Updated child"`)
	updatedInput.ParentID = nil
	updated, err := fixture.service.SavePermission(ctx, child.ID, updatedInput)
	if err != nil {
		t.Fatalf("更新子菜单: %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != root.ID {
		t.Fatalf("更新后父级丢失: got %v, want %q", updated.ParentID, root.ID)
	}

	tree, err := fixture.service.Permissions(ctx, dto.PageQuery{}, true)
	if err != nil {
		t.Fatalf("读取菜单树: %v", err)
	}
	rootNode, ok := findPermission(tree, root.ID)
	if !ok || len(rootNode.Children) != 1 || rootNode.Children[0].ID != child.ID {
		t.Fatalf("菜单树未保留父子关系: root=%+v", rootNode)
	}
}

func TestSavePermissionRejectsParentCycle(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()
	root, err := fixture.service.SavePermission(ctx, "", testMenu(uniqueTestValue("root"), nil))
	if err != nil {
		t.Fatalf("创建根菜单: %v", err)
	}
	parentID := root.ID
	child, err := fixture.service.SavePermission(ctx, "", testMenu(uniqueTestValue("child"), &parentID))
	if err != nil {
		t.Fatalf("创建子菜单: %v", err)
	}

	childID := child.ID
	root.ParentID = &childID
	if _, err := fixture.service.SavePermission(ctx, root.ID, root); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("期望循环父级返回 ErrInvalid，得到 %v", err)
	}
}

func TestSaveRoleRejectsSuperAdminModification(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	_, err := fixture.service.SaveRole(context.Background(), fixture.actor.ID, fixture.superRole.ID, dto.RoleInput{
		Name:        "Changed super admin",
		Code:        fixture.superRole.Code,
		Description: "test",
		Permissions: []dto.PermissionInfo{},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望保护超级管理员角色，得到 %v", err)
	}
}

// createRoleWithLevel 在测试事务内创建一个指定优先级的角色，并返回其模型与 ID。
func createRoleWithLevel(t *testing.T, fixture *authFixture, level int) model.Role {
	t.Helper()
	role := model.Role{Code: uniqueTestValue("role"), Name: "level role", Description: "test", Level: level}
	if err := fixture.db.Create(&role).Error; err != nil {
		t.Fatalf("创建测试角色失败: %v", err)
	}
	return role
}

// actorWithRole 创建一个持有指定角色的用户，作为后续操作的 actor。
func actorWithRole(t *testing.T, fixture *authFixture, role model.Role) model.User {
	t.Helper()
	user := model.User{
		Username:     uniqueTestValue("actor"),
		PasswordHash: "test-only-hash",
		DisplayName:  "Level Actor",
		Enabled:      true,
	}
	if err := fixture.db.Create(&user).Error; err != nil {
		t.Fatalf("创建测试操作者失败: %v", err)
	}
	if err := fixture.db.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
		t.Fatalf("绑定测试操作者角色失败: %v", err)
	}
	return user
}

func TestSaveRoleRejectsSameOrHigherLevel(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()

	adminRole := createRoleWithLevel(t, fixture, 90)
	admin := actorWithRole(t, fixture, adminRole)

	// 同优先级角色不可管理。
	sameLevel := createRoleWithLevel(t, fixture, 90)
	_, err := fixture.service.SaveRole(ctx, admin.ID, sameLevel.ID, dto.RoleInput{
		Name:        "rename same level",
		Code:        sameLevel.Code,
		Description: "test",
		Level:       sameLevel.Level,
		Permissions: []dto.PermissionInfo{},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望同优先级角色被保护，得到 %v", err)
	}

	// 更高优先级角色不可管理。
	higherLevel := createRoleWithLevel(t, fixture, 100)
	_, err = fixture.service.SaveRole(ctx, admin.ID, higherLevel.ID, dto.RoleInput{
		Name:        "rename higher level",
		Code:        higherLevel.Code,
		Description: "test",
		Level:       higherLevel.Level,
		Permissions: []dto.PermissionInfo{},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望更高优先级角色被保护，得到 %v", err)
	}
}

func TestSaveRoleAllowsLowerLevelManagement(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()

	adminRole := createRoleWithLevel(t, fixture, 90)
	admin := actorWithRole(t, fixture, adminRole)

	lowerRole := createRoleWithLevel(t, fixture, 70)
	out, err := fixture.service.SaveRole(ctx, admin.ID, lowerRole.ID, dto.RoleInput{
		Name:        "renamed lower level",
		Code:        lowerRole.Code,
		Description: "updated",
		Level:       60,
		Permissions: []dto.PermissionInfo{},
	})
	if err != nil {
		t.Fatalf("期望可以管理低优先级角色，得到 %v", err)
	}
	if out.Level != 60 {
		t.Fatalf("期望优先级更新为 60，得到 %d", out.Level)
	}
}

func TestSaveRoleRejectsCreatingBuiltinOrHigherLevelRole(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()

	adminRole := createRoleWithLevel(t, fixture, 90)
	admin := actorWithRole(t, fixture, adminRole)

	// 不允许占用内置角色码。
	_, err := fixture.service.SaveRole(ctx, admin.ID, 0, dto.RoleInput{
		Name:        "fake admin",
		Code:        "admin",
		Description: "test",
		Level:       10,
		Permissions: []dto.PermissionInfo{},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望内置角色码被保护，得到 %v", err)
	}

	// 不允许创建与操作者同级或更高的角色（管理员不能新建管理员）。
	_, err = fixture.service.SaveRole(ctx, admin.ID, 0, dto.RoleInput{
		Name:        "same level",
		Code:        uniqueTestValue("role"),
		Description: "test",
		Level:       90,
		Permissions: []dto.PermissionInfo{},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望同级新角色被保护，得到 %v", err)
	}

	// 允许创建低于自己的角色。
	if _, err := fixture.service.SaveRole(ctx, admin.ID, 0, dto.RoleInput{
		Name:        "lower level",
		Code:        uniqueTestValue("role"),
		Description: "test",
		Level:       10,
		Permissions: []dto.PermissionInfo{},
	}); err != nil {
		t.Fatalf("期望可以创建低优先级角色，得到 %v", err)
	}
}

func TestDeleteRoleEnforcesLevel(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()

	adminRole := createRoleWithLevel(t, fixture, 90)
	admin := actorWithRole(t, fixture, adminRole)

	// 同级不可删除。
	sameLevel := createRoleWithLevel(t, fixture, 90)
	if err := fixture.service.DeleteRole(ctx, admin.ID, sameLevel.ID); !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望同级角色不可删除，得到 %v", err)
	}

	// 低优先级可以删除。
	lowerLevel := createRoleWithLevel(t, fixture, 50)
	if err := fixture.service.DeleteRole(ctx, admin.ID, lowerLevel.ID); err != nil {
		t.Fatalf("期望可以删除低优先级角色，得到 %v", err)
	}
	var count int64
	if err := fixture.db.Model(&model.Role{}).Where("id = ?", lowerLevel.ID).Count(&count).Error; err != nil {
		t.Fatalf("统计被删除角色失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("期望低优先级角色已被删除，剩余 %d", count)
	}
}

func TestSaveUserRejectsAssigningSameOrHigherLevelRole(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()

	adminRole := createRoleWithLevel(t, fixture, 90)
	admin := actorWithRole(t, fixture, adminRole)
	managerRole := createRoleWithLevel(t, fixture, 70)

	user := model.User{
		Username:     uniqueTestValue("user"),
		PasswordHash: "test-only-hash",
		DisplayName:  "Level User",
		Enabled:      true,
	}
	if err := fixture.db.Create(&user).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	// 不能给用户分配与自己同级的角色。
	_, err := fixture.service.SaveUser(ctx, admin.ID, user.ID, dto.UserInput{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Status:      "active",
		Roles:       []dto.RoleInfo{{ID: strconv.FormatInt(adminRole.ID, 10), Code: adminRole.Code, Name: adminRole.Name, Level: adminRole.Level}},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望同级角色不可分配，得到 %v", err)
	}

	// 可以分配比自己低的角色。
	if _, err := fixture.service.SaveUser(ctx, admin.ID, user.ID, dto.UserInput{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Status:      "active",
		Roles:       []dto.RoleInfo{{ID: strconv.FormatInt(managerRole.ID, 10), Code: managerRole.Code, Name: managerRole.Name, Level: managerRole.Level}},
	}); err != nil {
		t.Fatalf("期望可以分配低优先级角色，得到 %v", err)
	}
}

func TestSaveUserCannotRemoveOwnRole(t *testing.T) {
	fixture := newAuthFixture(t, nil)
	ctx := context.Background()
	roleInfo, err := fixture.service.SaveRole(ctx, fixture.actor.ID, 0, dto.RoleInput{
		Name:        "Self managed role",
		Code:        uniqueTestValue("role"),
		Description: "test role",
		Permissions: []dto.PermissionInfo{},
	})
	if err != nil {
		t.Fatalf("创建测试角色: %v", err)
	}
	user := model.User{
		Username:     uniqueTestValue("user"),
		PasswordHash: "test-only-hash",
		DisplayName:  "Test User",
		Enabled:      true,
	}
	if err := fixture.db.Create(&user).Error; err != nil {
		t.Fatalf("创建测试用户: %v", err)
	}
	if err := fixture.db.Create(&model.UserRole{UserID: user.ID, RoleID: mustParseID(t, roleInfo.ID)}).Error; err != nil {
		t.Fatalf("分配测试角色: %v", err)
	}

	_, err = fixture.service.SaveUser(ctx, user.ID, user.ID, dto.UserInput{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Status:      "active",
		Roles:       []dto.RoleInfo{},
	})
	if !errors.Is(err, model.ErrProtected) {
		t.Fatalf("期望拒绝用户移除自身角色，得到 %v", err)
	}

	var assignments []model.UserRole
	if err := fixture.db.Where("user_id = ?", user.ID).Find(&assignments).Error; err != nil {
		t.Fatalf("读取用户角色: %v", err)
	}
	if len(assignments) != 1 || assignments[0].RoleID != mustParseID(t, roleInfo.ID) {
		t.Fatalf("拒绝操作后用户角色被修改: %+v", assignments)
	}
}

func TestListUsersQueryCountDoesNotGrowWithUserCount(t *testing.T) {
	queryLog := newSelectQueryCounter()
	fixture := newAuthFixture(t, queryLog)
	var basicRole model.Role
	if err := fixture.db.Where("code = ?", "user").First(&basicRole).Error; err != nil {
		t.Fatalf("查询普通用户角色: %v", err)
	}
	prefix := uniqueTestValue("list")
	insertUsersWithRole(t, fixture.db, prefix, 1, basicRole.ID)

	queryLog.Reset()
	firstPage, err := fixture.service.ListUsers(context.Background(), dto.PageQuery{Username: prefix, Current: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("查询单用户列表: %v", err)
	}
	firstCount := queryLog.Count()

	insertUsersWithRole(t, fixture.db, prefix+"_more", 19, basicRole.ID)
	queryLog.Reset()
	largePage, err := fixture.service.ListUsers(context.Background(), dto.PageQuery{Username: prefix, Current: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("查询多用户列表: %v", err)
	}
	largeCount := queryLog.Count()
	if firstPage.Total != 1 || largePage.Total != 20 {
		t.Fatalf("测试数据数量错误: first=%d large=%d", firstPage.Total, largePage.Total)
	}
	if firstCount != largeCount {
		t.Fatalf("用户数增加导致查询数增加: one user=%d, twenty users=%d", firstCount, largeCount)
	}
	if largeCount > 8 {
		t.Fatalf("用户列表查询过多: got %d SELECT statements", largeCount)
	}
}

func testMenu(code string, parentID *string) dto.PermissionInfo {
	return dto.PermissionInfo{
		ID:          "",
		Name:        json.RawMessage(`"Test menu"`),
		Code:        "test.menu." + code,
		Description: "integration test",
		Resource:    "/test",
		Action:      "view",
		Type:        "menu",
		ParentID:    parentID,
		Path:        "/test/" + code,
		Component:   "test/index",
		Sort:        1,
		Status:      "active",
	}
}

func insertUsersWithRole(t *testing.T, db *gorm.DB, prefix string, count int, roleID int64) {
	t.Helper()
	for i := 0; i < count; i++ {
		user := model.User{
			Username:     fmt.Sprintf("%s_%02d", prefix, i),
			PasswordHash: "test-only-hash",
			DisplayName:  "List Test User",
			Enabled:      true,
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("创建列表测试用户: %v", err)
		}
		if err := db.Create(&model.UserRole{UserID: user.ID, RoleID: roleID}).Error; err != nil {
			t.Fatalf("分配列表测试用户角色: %v", err)
		}
	}
}

func findPermission(permissions []dto.PermissionInfo, id string) (dto.PermissionInfo, bool) {
	for _, permission := range permissions {
		if permission.ID == id {
			return permission, true
		}
		if found, ok := findPermission(permission.Children, id); ok {
			return found, true
		}
	}
	return dto.PermissionInfo{}, false
}

func mustParseID(t *testing.T, id string) int64 {
	t.Helper()
	value, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		t.Fatalf("解析角色 ID %q: %v", id, err)
	}
	return value
}

func uniqueTestValue(prefix string) string {
	sequence := fixtureSequence.Add(1)
	return fmt.Sprintf("%s_%x_%x", prefix, time.Now().UnixNano(), sequence)
}

type selectQueryCounter struct {
	gormlogger.Interface
	mu      sync.Mutex
	selects int
}

func newSelectQueryCounter() *selectQueryCounter {
	base := gormlogger.New(log.New(io.Discard, "", 0), gormlogger.Config{LogLevel: gormlogger.Info})
	return &selectQueryCounter{Interface: base}
}

func (c *selectQueryCounter) Trace(_ context.Context, _ time.Time, query func() (string, int64), _ error) {
	sql, _ := query()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT ") {
		c.mu.Lock()
		c.selects++
		c.mu.Unlock()
	}
}

func (c *selectQueryCounter) Reset() {
	c.mu.Lock()
	c.selects = 0
	c.mu.Unlock()
}

func (c *selectQueryCounter) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.selects
}
