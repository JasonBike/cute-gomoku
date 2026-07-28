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
	flag.Parse()

	server := &http.Server{
		Addr:              *addr,
		Handler:           game.NewServer(webapp.Files()),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("棋遇服务已启动：http://localhost%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
