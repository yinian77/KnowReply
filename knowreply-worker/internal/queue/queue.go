package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const (
	// TypeDocIngest 文档入库任务：解析 → 切分 → embedding → 写 pgvector。
	TypeDocIngest = "doc:ingest"
)

// IngestPayload 任务载荷。TaskID 用 docId 保证同文档重复投递幂等。
type IngestPayload struct {
	TenantID uint64 `json:"tenantId"`
	DocID    uint64 `json:"docId"`
}

func NewIngestTask(p IngestPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal ingest payload: %w", err)
	}
	return asynq.NewTask(TypeDocIngest, body), nil
}

func ParseIngestPayload(t *asynq.Task) (IngestPayload, error) {
	var p IngestPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return p, fmt.Errorf("unmarshal ingest payload: %w", err)
	}
	if p.DocID == 0 || p.TenantID == 0 {
		return p, fmt.Errorf("invalid payload: tenantId=%d docId=%d", p.TenantID, p.DocID)
	}
	return p, nil
}

// EnqueueIngest 投递入库任务；TaskID=docId 时同文档重复入队会被去重。
func EnqueueIngest(client *asynq.Client, p IngestPayload, queueName string) (string, error) {
	task, err := NewIngestTask(p)
	if err != nil {
		return "", err
	}
	info, err := client.Enqueue(task,
		asynq.Queue(queueName),
		asynq.TaskID(fmt.Sprintf("doc:%d", p.DocID)),
		asynq.MaxRetry(5),
		asynq.Timeout(5*time.Minute),
	)
	if err != nil {
		return "", fmt.Errorf("enqueue ingest task: %w", err)
	}
	return info.ID, nil
}
