package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// 与 knowreply-worker 的 internal/queue 保持一致（两端是独立模块，只能按契约重复定义）。
const (
	TypeDocIngest = "doc:ingest"
	TypeDocDelete = "doc:delete"
)

type DocPayload struct {
	TenantID uint64 `json:"tenantId"`
	DocID    uint64 `json:"docId"`
}

// Client 文档处理任务的投递端；网关只投递，不消费。
type Client struct {
	cli       *asynq.Client
	queueName string
}

func NewClient(addr, password string, db int, queueName string) *Client {
	return &Client{
		cli: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
		queueName: queueName,
	}
}

func (c *Client) Close() error { return c.cli.Close() }

// EnqueueIngest 投递入库任务。TaskID 用 docId 保证同文档并发重复投递被去重。
func (c *Client) EnqueueIngest(p DocPayload) (string, error) {
	info, err := c.enqueue(TypeDocIngest, p,
		asynq.TaskID(fmt.Sprintf("doc:%d", p.DocID)),
		asynq.MaxRetry(5),
		asynq.Timeout(5*time.Minute),
	)
	if err != nil {
		return "", fmt.Errorf("enqueue ingest: %w", err)
	}
	return info.ID, nil
}

// EnqueueDelete 投递下线任务，由 worker 清理 pgvector 中该文档的切片。
func (c *Client) EnqueueDelete(p DocPayload) (string, error) {
	info, err := c.enqueue(TypeDocDelete, p,
		asynq.TaskID(fmt.Sprintf("del:%d:%d", p.DocID, time.Now().Unix())),
		asynq.MaxRetry(3),
		asynq.Timeout(time.Minute),
	)
	if err != nil {
		return "", fmt.Errorf("enqueue delete: %w", err)
	}
	return info.ID, nil
}

func (c *Client) enqueue(taskType string, p DocPayload, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	allOpts := append([]asynq.Option{asynq.Queue(c.queueName)}, opts...)
	return c.cli.Enqueue(asynq.NewTask(taskType, body), allOpts...)
}
