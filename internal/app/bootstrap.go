package app

import (
	"antdv-next-admin-go/internal/modules/auth"
	"context"
	"fmt"
	"os"
	"runtime"
)

func initializeSuperAdmin(ctx context.Context, module *auth.Module, username string) error {
	var terminal *os.File
	defer func() {
		if terminal != nil {
			_ = terminal.Close()
		}
	}()
	password, err := module.Service.Bootstrap(ctx, func() error {
		path := "/dev/tty"
		if runtime.GOOS == "windows" {
			path = "CONOUT$"
		}
		var err error
		terminal, err = os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("首次创建超级管理员需要交互终端，请在终端运行 task run: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("initialize super admin: %w", err)
	}
	if password != "" {
		if _, err := fmt.Fprintf(terminal, "\n超级管理员已创建：%s\n初始密码：%s\n此密码仅显示一次，请立即保存；数据库仅存储密码哈希。\n\n", username, password); err != nil {
			return fmt.Errorf("超级管理员已创建但终端显示失败，需要人工重置密码: %w", err)
		}
	}
	return nil
}
