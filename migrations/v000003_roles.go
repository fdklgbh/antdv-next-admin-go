package migrations

import (
	"encoding/json"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"time"
)

type role000003 struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Code        string `gorm:"size:64;not null;uniqueIndex"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"size:255;not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (role000003) TableName() string { return "auth_roles" }

type permission000003 struct {
	Code       string `gorm:"size:128;primaryKey"`
	Definition string `gorm:"type:text;not null"`
}

func (permission000003) TableName() string { return "auth_permissions" }

type userRole000003 struct {
	UserID int64 `gorm:"primaryKey"`
	RoleID int64 `gorm:"primaryKey;index"`
}

func (userRole000003) TableName() string { return "auth_user_roles" }

type rolePermission000003 struct {
	RoleID         int64  `gorm:"primaryKey"`
	PermissionCode string `gorm:"size:128;primaryKey;index"`
}

func (rolePermission000003) TableName() string { return "auth_role_permissions" }

type bootstrapState000003 struct {
	ID        int64 `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt time.Time
}

func (bootstrapState000003) TableName() string { return "auth_bootstrap" }

func migration000003Roles() *gormigrate.Migration {
	return &gormigrate.Migration{ID: "000003_roles", Migrate: func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&role000003{}, &permission000003{}, &userRole000003{}, &rolePermission000003{}, &bootstrapState000003{}); err != nil {
			return err
		}
		var definitions []json.RawMessage
		if err := json.Unmarshal([]byte(permissionSeed000003), &definitions); err != nil {
			return err
		}
		codes := []string{}
		for _, definition := range definitions {
			var item struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(definition, &item); err != nil {
				return err
			}
			if err := tx.Create(&permission000003{Code: item.Code, Definition: string(definition)}).Error; err != nil {
				return err
			}
			codes = append(codes, item.Code)
		}
		roles := []role000003{
			{Code: "super_admin", Name: "超级管理员", Description: "首次初始化创建，拥有全部权限"},
			{Code: "admin", Name: "管理员", Description: "拥有初始化时的全部前端权限"},
			{Code: "manager", Name: "管理人员", Description: "用户管理和角色查看"},
			{Code: "user", Name: "普通用户", Description: "仪表盘和用户查看"},
			{Code: "guest", Name: "访客", Description: "仅查看仪表盘"},
		}
		for _, role := range roles {
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
			for _, code := range codes {
				allowed := role.Code == "admin" ||
					role.Code == "manager" && (code == "dashboard.view" || code == "organization.menu" || code == "system.user.view" || code == "system.user.create" || code == "system.user.edit" || code == "system.user.delete" || code == "system.role.view") ||
					role.Code == "user" && (code == "dashboard.view" || code == "organization.menu" || code == "system.user.view") ||
					role.Code == "guest" && code == "dashboard.view"
				if allowed {
					if err := tx.Create(&rolePermission000003{RoleID: role.ID, PermissionCode: code}).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	}, Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable(&bootstrapState000003{}, &rolePermission000003{}, &userRole000003{}, &permission000003{}, &role000003{})
	}}
}

const permissionSeed000003 = `[{"id":"1","name":{"zh-CN":"仪表盘","en-US":"Dashboard","ja-JP":"ダッシュボード","ko-KR":"대시보드"},"code":"dashboard.view","description":"Dashboard menu","resource":"/dashboard","action":"view","type":"menu","path":"/dashboard","component":"dashboard/index","icon":"antdv-next:DashboardOutlined","sort":1,"status":"active","visible":true},{"id":"60","name":{"zh-CN":"权限管理","en-US":"Permission Management","ja-JP":"権限管理","ko-KR":"권한 관리"},"code":"organization.menu","description":"Permission management root menu","resource":"/organization","action":"*","type":"menu","path":"/organization","component":"Layout","icon":"antdv-next:TeamOutlined","sort":2,"status":"active","visible":true},{"id":"11","name":{"zh-CN":"用户管理","en-US":"User Management","ja-JP":"ユーザー管理","ko-KR":"사용자 관리"},"code":"system.user.view","description":"User management menu","resource":"/organization/user","action":"view","type":"menu","parentId":"60","path":"/organization/user","component":"system/user/index","icon":"antdv-next:UserOutlined","sort":1,"status":"active","visible":true},{"id":"12","name":{"zh-CN":"新增用户","en-US":"Create User","ja-JP":"ユーザーを作成","ko-KR":"사용자 생성"},"code":"system.user.create","description":"Can create users","resource":"system.user","action":"create","type":"button","parentId":"11","status":"active","visible":true},{"id":"13","name":{"zh-CN":"编辑用户","en-US":"Edit User","ja-JP":"ユーザーを編集","ko-KR":"사용자 편집"},"code":"system.user.edit","description":"Can edit users","resource":"system.user","action":"edit","type":"button","parentId":"11","status":"active","visible":true},{"id":"14","name":{"zh-CN":"删除用户","en-US":"Delete User","ja-JP":"ユーザーを削除","ko-KR":"사용자 삭제"},"code":"system.user.delete","description":"Can delete users","resource":"system.user","action":"delete","type":"button","parentId":"11","status":"active","visible":true},{"id":"20","name":{"zh-CN":"角色管理","en-US":"Role Management","ja-JP":"ロール管理","ko-KR":"역할 관리"},"code":"system.role.view","description":"Role management menu","resource":"/organization/role","action":"view","type":"menu","parentId":"60","path":"/organization/role","component":"system/role/index","icon":"antdv-next:TeamOutlined","sort":2,"status":"active","visible":true},{"id":"21","name":{"zh-CN":"新增角色","en-US":"Create Role","ja-JP":"ロールを作成","ko-KR":"역할 생성"},"code":"system.role.create","description":"Can create roles","resource":"system.role","action":"create","type":"button","parentId":"20","status":"active","visible":true},{"id":"22","name":{"zh-CN":"编辑角色","en-US":"Edit Role","ja-JP":"ロールを編集","ko-KR":"역할 편집"},"code":"system.role.edit","description":"Can edit roles","resource":"system.role","action":"edit","type":"button","parentId":"20","status":"active","visible":true},{"id":"23","name":{"zh-CN":"删除角色","en-US":"Delete Role","ja-JP":"ロールを削除","ko-KR":"역할 삭제"},"code":"system.role.delete","description":"Can delete roles","resource":"system.role","action":"delete","type":"button","parentId":"20","status":"active","visible":true},{"id":"30","name":{"zh-CN":"菜单管理","en-US":"Menu Management","ja-JP":"メニュー管理","ko-KR":"메뉴 관리"},"code":"system.permission.view","description":"Menu management menu","resource":"/organization/permission","action":"view","type":"menu","parentId":"60","path":"/organization/permission","component":"system/permission/index","icon":"antdv-next:SafetyOutlined","sort":3,"status":"active","visible":true},{"id":"31","name":{"zh-CN":"新增菜单","en-US":"Create Menu","ja-JP":"メニューを作成","ko-KR":"메뉴 생성"},"code":"system.permission.create","description":"Can create menu","resource":"system.permission","action":"create","type":"button","parentId":"30","status":"active","visible":true},{"id":"32","name":{"zh-CN":"编辑菜单","en-US":"Edit Menu","ja-JP":"メニューを編集","ko-KR":"메뉴 편집"},"code":"system.permission.edit","description":"Can edit menu","resource":"system.permission","action":"edit","type":"button","parentId":"30","status":"active","visible":true},{"id":"33","name":{"zh-CN":"删除菜单","en-US":"Delete Menu","ja-JP":"メニューを削除","ko-KR":"메뉴 삭제"},"code":"system.permission.delete","description":"Can delete menu","resource":"system.permission","action":"delete","type":"button","parentId":"30","status":"active","visible":true},{"id":"10","name":{"zh-CN":"系统管理","en-US":"System Management","ja-JP":"システム管理","ko-KR":"시스템 관리"},"code":"system.menu","description":"System management root menu","resource":"/system","action":"*","type":"menu","path":"/system","component":"Layout","icon":"antdv-next:SettingOutlined","sort":3,"status":"active","visible":true},{"id":"54","name":{"zh-CN":"系统配置","en-US":"System Config","ja-JP":"システム設定","ko-KR":"시스템 설정"},"code":"system.config.view","description":"System config menu","resource":"/system/config","action":"view","type":"menu","parentId":"10","path":"/system/config","component":"system/config/index","icon":"antdv-next:ControlOutlined","sort":1,"status":"active","visible":true},{"id":"55","name":{"zh-CN":"新增配置","en-US":"Create Config","ja-JP":"設定を作成","ko-KR":"설정 생성"},"code":"system.config.create","description":"Can create config","resource":"system.config","action":"create","type":"button","parentId":"54","status":"active","visible":true},{"id":"56","name":{"zh-CN":"编辑配置","en-US":"Edit Config","ja-JP":"設定を編集","ko-KR":"설정 편집"},"code":"system.config.edit","description":"Can edit config","resource":"system.config","action":"edit","type":"button","parentId":"54","status":"active","visible":true},{"id":"57","name":{"zh-CN":"删除配置","en-US":"Delete Config","ja-JP":"設定を削除","ko-KR":"설정 삭제"},"code":"system.config.delete","description":"Can delete config","resource":"system.config","action":"delete","type":"button","parentId":"54","status":"active","visible":true},{"id":"34","name":{"zh-CN":"字典管理","en-US":"Dictionary Management","ja-JP":"辞書管理","ko-KR":"사전 관리"},"code":"system.dict.view","description":"Dictionary management menu","resource":"/system/dict","action":"view","type":"menu","parentId":"10","path":"/system/dict","component":"system/dict/index","icon":"antdv-next:BookOutlined","sort":2,"status":"active","visible":true},{"id":"35","name":{"zh-CN":"新增字典","en-US":"Create Dictionary","ja-JP":"辞書を作成","ko-KR":"사전 생성"},"code":"system.dict.create","description":"Can create dictionary","resource":"system.dict","action":"create","type":"button","parentId":"34","status":"active","visible":true},{"id":"36","name":{"zh-CN":"编辑字典","en-US":"Edit Dictionary","ja-JP":"辞書を編集","ko-KR":"사전 편집"},"code":"system.dict.edit","description":"Can edit dictionary","resource":"system.dict","action":"edit","type":"button","parentId":"34","status":"active","visible":true},{"id":"37","name":{"zh-CN":"删除字典","en-US":"Delete Dictionary","ja-JP":"辞書を削除","ko-KR":"사전 삭제"},"code":"system.dict.delete","description":"Can delete dictionary","resource":"system.dict","action":"delete","type":"button","parentId":"34","status":"active","visible":true},{"id":"58","name":{"zh-CN":"文件管理","en-US":"File Management","ja-JP":"ファイル管理","ko-KR":"파일 관리"},"code":"system.file.view","description":"File management menu","resource":"/system/file","action":"view","type":"menu","parentId":"10","path":"/system/file","component":"system/file/index","icon":"antdv-next:FolderOutlined","sort":3,"status":"active","visible":true},{"id":"59","name":{"zh-CN":"删除文件","en-US":"Delete File","ja-JP":"ファイルを削除","ko-KR":"파일 삭제"},"code":"system.file.delete","description":"Can delete file","resource":"system.file","action":"delete","type":"button","parentId":"58","status":"active","visible":true},{"id":"38","name":{"zh-CN":"系统日志","en-US":"System Log","ja-JP":"システムログ","ko-KR":"시스템 로그"},"code":"system.log.view","description":"System log menu","resource":"/system/log","action":"view","type":"menu","parentId":"10","path":"/system/log","component":"system/log/index","icon":"antdv-next:FileTextOutlined","sort":4,"status":"active","visible":true},{"id":"39","name":{"zh-CN":"清空日志","en-US":"Clear Log","ja-JP":"ログをクリア","ko-KR":"로그 지우기"},"code":"system.log.clear","description":"Can clear logs","resource":"system.log","action":"delete","type":"button","parentId":"38","status":"active","visible":true},{"id":"40","name":{"zh-CN":"示例","en-US":"Examples","ja-JP":"サンプル","ko-KR":"예제"},"code":"examples.menu","description":"Examples root menu","resource":"/examples","action":"*","type":"menu","path":"/examples","component":"Layout","icon":"antdv-next:AppstoreOutlined","sort":4,"status":"inactive","visible":false},{"id":"41","name":{"zh-CN":"表格示例","en-US":"Table Example","ja-JP":"テーブルサンプル","ko-KR":"테이블 예제"},"code":"examples.table.view","description":"Table example menu","resource":"/examples/table","action":"view","type":"menu","parentId":"40","path":"/examples/table","component":"examples/table/index","icon":"antdv-next:TableOutlined","sort":1,"status":"inactive","visible":false},{"id":"42","name":{"zh-CN":"图标示例","en-US":"Icon Example","ja-JP":"アイコンサンプル","ko-KR":"아이콘 예제"},"code":"examples.icon.view","description":"Icon example menu","resource":"/examples/icon","action":"view","type":"menu","parentId":"40","path":"/examples/icon","component":"examples/icon/index","icon":"antdv-next:SmileOutlined","sort":2,"status":"inactive","visible":false},{"id":"50","name":{"zh-CN":"图标库","en-US":"Icon Library","ja-JP":"アイコンライブラリ","ko-KR":"아이콘 라이브러리"},"code":"examples.iconLibrary.view","description":"Icon library menu","resource":"/examples/icon-library","action":"view","type":"menu","parentId":"40","path":"/examples/icon-library","component":"examples/icon-library/index","icon":"antdv-next:AppstoreOutlined","sort":3,"status":"inactive","visible":false},{"id":"43","name":{"zh-CN":"表单示例","en-US":"Form Example","ja-JP":"フォームサンプル","ko-KR":"폼 예제"},"code":"examples.form.view","description":"Form example menu","resource":"/examples/form","action":"view","type":"menu","parentId":"40","path":"/examples/form","component":"examples/form/index","icon":"antdv-next:FormOutlined","sort":4,"status":"inactive","visible":false},{"id":"44","name":{"zh-CN":"弹窗示例","en-US":"Modal Example","ja-JP":"モーダルサンプル","ko-KR":"모달 예제"},"code":"examples.modal.view","description":"Modal example menu","resource":"/examples/modal","action":"view","type":"menu","parentId":"40","path":"/examples/modal","component":"examples/modal/index","icon":"antdv-next:ExpandOutlined","sort":5,"status":"inactive","visible":false},{"id":"45","name":{"zh-CN":"水印示例","en-US":"Watermark Example","ja-JP":"透かしサンプル","ko-KR":"워터마크 예제"},"code":"examples.watermark.view","description":"Watermark example menu","resource":"/examples/watermark","action":"view","type":"menu","parentId":"40","path":"/examples/watermark","component":"examples/watermark/index","icon":"antdv-next:HighlightOutlined","sort":6,"status":"inactive","visible":false},{"id":"46","name":{"zh-CN":"异常页面","en-US":"Exception Page","ja-JP":"例外ページ","ko-KR":"예외 페이지"},"code":"examples.exception.menu","description":"Exception root menu","resource":"/examples/exception","action":"view","type":"menu","parentId":"40","path":"/examples/exception","component":"RouteView","icon":"antdv-next:WarningOutlined","sort":7,"status":"inactive","visible":false},{"id":"47","name":{"zh-CN":"403","en-US":"403","ja-JP":"403","ko-KR":"403"},"code":"examples.exception.403.view","description":"403 exception page menu","resource":"/examples/exception/403","action":"view","type":"menu","parentId":"46","path":"/examples/exception/403","component":"examples/exception/403","icon":"antdv-next:StopOutlined","sort":1,"status":"inactive","visible":false},{"id":"48","name":{"zh-CN":"404","en-US":"404","ja-JP":"404","ko-KR":"404"},"code":"examples.exception.404.view","description":"404 exception page menu","resource":"/examples/exception/404","action":"view","type":"menu","parentId":"46","path":"/examples/exception/404","component":"examples/exception/404","icon":"antdv-next:FileUnknownOutlined","sort":2,"status":"inactive","visible":false},{"id":"49","name":{"zh-CN":"500","en-US":"500","ja-JP":"500","ko-KR":"500"},"code":"examples.exception.500.view","description":"500 exception page menu","resource":"/examples/exception/500","action":"view","type":"menu","parentId":"46","path":"/examples/exception/500","component":"examples/exception/500","icon":"antdv-next:BugOutlined","sort":3,"status":"inactive","visible":false}]`
