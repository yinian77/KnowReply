package handler

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yinian77/KnowReply/knowreply-gateway/internal/queue"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/store"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
	maxTitleLen      = 200
)

// allowedExt 与 worker 的解析器保持一致，其他格式在入口就拒掉，避免进队列后才失败。
var allowedExt = map[string]bool{
	".md":       true,
	".markdown": true,
	".txt":      true,
	".docx":     true,
	".pdf":      true,
}

// ListDocuments 知识文档列表，按租户隔离。
func (h *Handler) ListDocuments(c *gin.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	limit := defaultPageLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = min(n, maxPageLimit)
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	docs, err := h.deps.Store.ListDocuments(ctx, claims.TenantID, c.Query("status"), limit, offset)
	if err != nil {
		h.deps.Log.Error("list documents failed", "err", err, "tenantId", claims.TenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list documents failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": docs, "limit": limit, "offset": offset})
}

// UploadDocument 接收 multipart 上传：落盘 → 建 documents 记录 → 投递入库任务。
// 返回 202，真正的解析与向量化在 worker 异步完成。
func (h *Handler) UploadDocument(c *gin.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	maxBytes := h.deps.Storage.MaxUploadBytes
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes+1<<20)

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart field \"file\" is required"})
		return
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowedExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "unsupported file type",
			"allowed": []string{".md", ".markdown", ".txt", ".docx", ".pdf"},
		})
		return
	}
	if fh.Size > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "file too large", "maxBytes": maxBytes, "size": fh.Size,
		})
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename))
	}
	if len([]rune(title)) > maxTitleLen {
		title = string([]rune(title)[:maxTitleLen])
	}

	absPath, err := h.saveUpload(fh, claims.TenantID, ext)
	if err != nil {
		h.deps.Log.Error("save upload failed", "err", err, "tenantId", claims.TenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save file failed"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	docID, err := h.deps.Store.CreateDocument(ctx, claims.TenantID, title, "file://"+absPath, "v1")
	if err != nil {
		_ = os.Remove(absPath)
		h.deps.Log.Error("create document failed", "err", err, "tenantId", claims.TenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create document failed"})
		return
	}

	taskID, err := h.deps.Queue.EnqueueIngest(queue.DocPayload{TenantID: claims.TenantID, DocID: docID})
	if err != nil {
		// 文件与记录已落，标记失败让运营可见并可通过重建重试
		_ = h.deps.Store.SetDocumentStatus(ctx, claims.TenantID, docID, "failed")
		h.deps.Log.Error("enqueue ingest failed", "err", err, "docId", docID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "enqueue task failed", "docId": docID})
		return
	}

	h.deps.Log.Info("document uploaded",
		"tenantId", claims.TenantID, "docId", docID, "taskId", taskID,
		"ext", ext, "size", fh.Size, "title", title)

	c.JSON(http.StatusAccepted, gin.H{
		"docId":  docID,
		"taskId": taskID,
		"title":  title,
		"status": "pending",
	})
}

// DeleteDocument 下线文档：状态置 offline 并投递清理任务，worker 负责删切片。
// 元数据保留用于审计，符合 README「过期下线而非删除」。
func (h *Handler) DeleteDocument(c *gin.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	docID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || docID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	doc, err := h.deps.Store.GetDocument(ctx, claims.TenantID, docID)
	if errors.Is(err, store.ErrDocumentNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	if err != nil {
		h.deps.Log.Error("get document failed", "err", err, "docId", docID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get document failed"})
		return
	}

	if err := h.deps.Store.SetDocumentStatus(ctx, claims.TenantID, docID, "offline"); err != nil {
		h.deps.Log.Error("mark offline failed", "err", err, "docId", docID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mark offline failed"})
		return
	}

	taskID, err := h.deps.Queue.EnqueueDelete(queue.DocPayload{TenantID: claims.TenantID, DocID: docID})
	if err != nil {
		h.deps.Log.Error("enqueue delete failed", "err", err, "docId", docID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "enqueue cleanup failed", "docId": docID})
		return
	}

	h.deps.Log.Info("document offlined", "tenantId", claims.TenantID, "docId", docID, "taskId", taskID)
	c.JSON(http.StatusOK, gin.H{"docId": docID, "title": doc.Title, "status": "offline", "taskId": taskID})
}

// saveUpload 存到 {root}/{tenantId}/{yyyy}/{mm}/{random}{ext}，返回绝对路径。
// 用随机文件名而不是 docId，避免「先建记录后落盘」的顺序依赖。
func (h *Handler) saveUpload(fh *multipart.FileHeader, tenantID uint64, ext string) (string, error) {
	dir := filepath.Join(h.deps.Storage.RootDir, strconv.FormatUint(tenantID, 10), time.Now().Format("2006/01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	abs := filepath.Join(dir, newID("")+ext)

	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(abs)
		return "", err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(abs)
		return "", err
	}
	return abs, nil
}
