package task

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/queue"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
)

// Deleter 处理文档下线：清掉 pgvector 切片。
// documents 记录保留（status=offline），由网关负责标记，符合审计要求。
type Deleter struct {
	kb  *store.PG
	log *slog.Logger
}

func NewDeleter(kb *store.PG, log *slog.Logger) *Deleter {
	return &Deleter{kb: kb, log: log}
}

func (d *Deleter) ProcessTask(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseIngestPayload(t)
	if err != nil {
		return fmt.Errorf("%w: %s", asynq.SkipRetry, err)
	}

	n, err := d.kb.DeleteByDoc(ctx, p.TenantID, p.DocID)
	if err != nil {
		return err
	}

	d.log.Info("document chunks removed",
		"tenantId", p.TenantID, "docId", p.DocID, "deleted", n)
	return nil
}
