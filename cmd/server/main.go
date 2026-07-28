package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"cute-gomoku/internal/game"
	webapp "cute-gomoku/internal/web"
)

func main() {
	addr := flag.String("addr", ":8090", "HTTP listen address")
	dataPath := flag.String("data", "./data/state.json", "user and session JSON state path")
	flag.Parse()

	handler, err := game.NewServerWithDataFile(webapp.Files(), *dataPath)
	if err != nil {
		log.Fatalf("加载用户数据失败：%v", err)
	}
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("棋遇服务已启动：http://localhost%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
