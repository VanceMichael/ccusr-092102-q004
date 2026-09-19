package main

import (
	"flag"
	"log"
	"net/http"

	"example.com/teacher-curriculum-evidence/internal/api"
	"example.com/teacher-curriculum-evidence/internal/evidence"
)

func main() {
	addr := flag.String("addr", ":8080", "监听地址")
	data := flag.String("data", "data/evidence.json", "证据快照文件路径（空字符串表示不持久化）")
	demo := flag.Bool("demo", true, "存储为空时写入十国演示数据")
	flag.Parse()

	store, err := evidence.NewStore(*data)
	if err != nil {
		log.Fatalf("打开证据存储失败: %v", err)
	}
	svc := evidence.NewService(store)
	if *demo {
		if err := svc.SeedDemo(); err != nil {
			log.Fatalf("写入演示数据失败: %v", err)
		}
	}

	srv := api.NewServer(svc)
	log.Printf("教师课程改革证据服务监听 %s（数据文件 %q）", *addr, *data)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
