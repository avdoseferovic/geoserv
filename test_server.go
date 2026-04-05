package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avdoseferovic/geoserv/internal/admin"
	"github.com/avdoseferovic/geoserv/internal/config"
)

type dummyWorld struct{}

func (d dummyWorld) OnlinePlayerCount() int { return 42 }

func main() {
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Port: "8089",
		},
	}
	srv := admin.New(cfg, dummyWorld{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Start(ctx)
	
	time.Sleep(1 * time.Second)
	
	resp, err := http.Get("http://127.0.0.1:8089/")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
