package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/config"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/queue"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
)

// 开发期投递工具：网关的上传接口还没做，先用它把文档推进队列验证流水线。
func main() {
	var (
		tenantID = flag.Uint64("tenant", 1, "租户 ID")
		docID    = flag.Uint64("doc", 0, "已存在的文档 ID（与 -file 二选一）")
		file     = flag.String("file", "", "本地文件路径；指定后会新建 documents 记录再入队")
		title    = flag.String("title", "", "文档标题，默认取文件名")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if *docID == 0 && *file == "" {
		fmt.Fprintln(os.Stderr, "必须指定 -doc 或 -file")
		os.Exit(1)
	}

	if *file != "" {
		db, err := sql.Open("mysql", cfg.MySQL.DSN)
		if err != nil {
			fmt.Fprintln(os.Stderr, "open mysql:", err)
			os.Exit(1)
		}
		defer db.Close()

		abs, err := filepath.Abs(*file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "resolve path:", err)
			os.Exit(1)
		}
		if *title == "" {
			*title = filepath.Base(abs)
		}

		id, err := store.NewMySQL(db).CreateDocument(ctx, *tenantID, *title, "file://"+abs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create document:", err)
			os.Exit(1)
		}
		*docID = id
		fmt.Printf("created document id=%d title=%q source=file://%s\n", id, *title, abs)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.QueueDB,
	})
	defer client.Close()

	taskID, err := queue.EnqueueIngest(client, queue.IngestPayload{TenantID: *tenantID, DocID: *docID}, cfg.QueueName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "enqueue:", err)
		os.Exit(1)
	}

	fmt.Printf("enqueued task=%s queue=%s tenantId=%d docId=%d\n", taskID, cfg.QueueName, *tenantID, *docID)
}
