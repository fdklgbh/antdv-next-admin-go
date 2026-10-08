package main

import (
	"antdv-next-admin-go/internal/config"
	"antdv-next-admin-go/internal/infrastructure/database"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/modules/auth/repository"
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"log"
	"os"
	"strings"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	path := flag.String("config", "configs/config.dev.yaml", "配置文件")
	username := flag.String("username", "", "用户名")
	email := flag.String("email", "", "邮箱地址（可选）")
	flag.Parse()
	if err := validator.New().Var(*email, "omitempty,email,max=254"); err != nil {
		return fmt.Errorf("邮箱格式无效或超过 254 字符")
	}
	if strings.TrimSpace(*username) == "" || len(*username) > 64 {
		return fmt.Errorf("用户名必填且不能超过 64 字节")
	}
	// 密码通过标准输入提供，避免出现在进程参数中。
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("从标准输入读取密码失败")
	}
	password := scanner.Text()
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("密码必须为 8 至 72 字节")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("计算密码哈希: %w", err)
	}
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.StartupTimeout)
	defer cancel()
	db, closeDB, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer closeDB()
	user := model.User{Username: *username, Email: *email, DisplayName: *username, PasswordHash: string(hash), Enabled: true}
	if err := repository.New(db).CreateUser(ctx, &user); err != nil {
		return err
	}
	fmt.Printf("已创建用户 ID=%d；尚未分配权限。\n", user.ID)
	return nil
}
