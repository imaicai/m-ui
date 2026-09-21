package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
)

var version = "1.4.7"

//go:embed web/* web/assets/*
var webFiles embed.FS

func main() {
	initMode := flag.Bool("init", false, "initialize m-ui data")
	initUser := flag.String("user", "admin", "initial admin username")
	initPass := flag.String("password", "", "initial admin password")
	showVersion := flag.Bool("version", false, "print version")
	printPanelPath := flag.Bool("print-panel-path", false, "print configured panel base path")
	printAdminUser := flag.Bool("print-admin-user", false, "print configured admin username")
	setAdminUser := flag.String("set-admin-user", "", "set admin username")
	setAdminPassword := flag.String("set-admin-password", "", "set admin password")
	setPanelPath := flag.String("set-panel-path", "", "set panel base path")
	resetPanelAccess := flag.Bool("reset-panel-access", false, "reset panel username, password and base path")
	flag.Parse()

	if *showVersion {
		fmt.Printf("m-ui %s by MiniJet\n", version)
		return
	}

	dataPath := getenv("MUI_DATA", "/etc/m-ui/state.json")
	store, err := NewStore(dataPath)
	if err != nil {
		log.Fatalf("m-ui: %v", err)
	}

	if *initMode {
		if err := store.EnsureInitialized(*initUser, *initPass); err != nil {
			log.Fatalf("初始化失败: %v", err)
		}
		fmt.Println("m-ui 初始化完成")
		return
	}

	if *printPanelPath {
		p, err := normalizePanelPath(store.Snapshot().Settings.PanelPath)
		if err != nil || p == "" {
			p = "/"
		}
		fmt.Println(p)
		return
	}

	if *printAdminUser {
		fmt.Println(store.Snapshot().Settings.AdminUser)
		return
	}

	if *resetPanelAccess {
		st := store.Snapshot()
		st.Settings.AdminUser = "admin"
		password := randomToken(20)
		if err := setPassword(&st, password); err != nil {
			log.Fatalf("重置面板失败: %v", err)
		}
		st.Settings.PanelPath = randomPanelPath()
		if err := store.Replace(st); err != nil {
			log.Fatalf("重置面板失败: %v", err)
		}
		fmt.Printf("管理员账号: %s\n管理员密码: %s\n根路径: %s\n", st.Settings.AdminUser, password, st.Settings.PanelPath)
		return
	}

	if *setAdminUser != "" || *setAdminPassword != "" || *setPanelPath != "" {
		st := store.Snapshot()
		if *setAdminUser != "" {
			st.Settings.AdminUser = cleanText(*setAdminUser, st.Settings.AdminUser, 60)
		}
		if *setAdminPassword != "" {
			if err := setPassword(&st, *setAdminPassword); err != nil {
				log.Fatalf("修改密码失败: %v", err)
			}
		}
		if *setPanelPath != "" {
			p, err := normalizePanelPath(*setPanelPath)
			if err != nil {
				log.Fatalf("修改根路径失败: %v", err)
			}
			st.Settings.PanelPath = p
		}
		if err := store.Replace(st); err != nil {
			log.Fatalf("保存面板设置失败: %v", err)
		}
		fmt.Println("m-ui 面板设置已更新")
		return
	}

	if store.Snapshot().Settings.PasswordHash == "" {
		user := getenv("MUI_INIT_USER", "admin")
		pass := os.Getenv("MUI_INIT_PASS")
		if pass == "" {
			log.Fatal("m-ui 尚未初始化，请先执行: m-ui --init --user admin --password '你的密码'")
		}
		if err := store.EnsureInitialized(user, pass); err != nil {
			log.Fatalf("自动初始化失败: %v", err)
		}
	}

	srv := NewServer(store, ServerConfig{
		Addr:       getenv("MUI_ADDR", "0.0.0.0:16800"),
		XrayBin:    getenv("MUI_XRAY_BIN", "/usr/local/bin/xray"),
		XrayConfig: getenv("MUI_XRAY_CONFIG", "/usr/local/etc/xray/config.json"),
	})
	if _, err := os.Stat(srv.cfg.XrayBin); err == nil {
		if err := srv.xray.Apply(store.Snapshot()); err != nil {
			log.Printf("m-ui: 启动时同步 Xray 配置失败，面板仍继续启动: %v", err)
		} else {
			log.Printf("m-ui: 已使用当前数据同步并校验 Xray 配置")
		}
	}
	log.Printf("m-ui %s by MiniJet listening on %s", version, srv.cfg.Addr)
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
