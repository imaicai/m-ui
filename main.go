package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
)

var version = "1.0.0"

//go:embed web/* web/assets/*
var webFiles embed.FS

func main() {
	initMode := flag.Bool("init", false, "initialize m-ui data")
	initUser := flag.String("user", "admin", "initial admin username")
	initPass := flag.String("password", "", "initial admin password")
	showVersion := flag.Bool("version", false, "print version")
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
		Addr:       getenv("MUI_ADDR", "0.0.0.0:54321"),
		XrayBin:    getenv("MUI_XRAY_BIN", "/usr/local/bin/xray"),
		XrayConfig: getenv("MUI_XRAY_CONFIG", "/usr/local/etc/xray/config.json"),
	})
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
