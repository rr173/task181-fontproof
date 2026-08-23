// Command fontproof 是数字字体回退覆盖证明工作台的入口。
//
// 支持：
//
//	--addr :8080     启动长驻 HTTP 服务
//	--db path        指定 SQLite 数据库路径（默认 :memory:）
//	--smoke-test     执行离线端到端验证后以 0 退出（不启动服务）
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"task181-fontproof/internal/demo"
	"task181-fontproof/internal/httpapi"
	"task181-fontproof/internal/service"
	"task181-fontproof/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "", "sqlite database path (default: in-memory)")
	smoke := flag.Bool("smoke-test", false, "run offline end-to-end smoke test and exit")
	flag.Parse()

	if *smoke {
		if err := demo.RunSmoke(*dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "smoke test failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("fontproof smoke test OK")
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	svc := service.New(st)
	if _, err := svc.Recover(); err != nil {
		log.Printf("recover: %v", err)
	}
	srv := &http.Server{
		Addr:    *addr,
		Handler: httpapi.New(svc).Handler(),
	}
	log.Printf("fontproof listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
