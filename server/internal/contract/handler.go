package contract

import (
	"database/sql"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/receivable"
)

// Handler 处理合同/附件 HTTP 请求。
type Handler struct {
	repo    *Repo
	ptyRepo *receivable.Repo
	clRepo  *changelog.Repo
}

// NewHandler 创建 Handler。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		repo:    NewRepo(db),
		ptyRepo: receivable.NewRepo(db),
		clRepo:  changelog.NewRepo(db),
	}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/contracts", h.ListContracts)
	r.POST("/api/contracts", h.CreateContract)
	r.GET("/api/contracts/:id", h.GetContract)
	r.DELETE("/api/contracts/:id", h.DeleteContract)
}

func (h *Handler) unauthorized(c *gin.Context) {
	platform.ErrResponse(c, http.StatusUnauthorized, &platform.AppError{
		Code: "UNAUTHORIZED", Message: "未登录或登录已过期",
	})
}

func (h *Handler) internal(c *gin.Context, msg string) {
	platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
		Code: "INTERNAL_ERROR", Message: msg,
	})
}

// ListContracts 合同列表。
// GET /api/contracts?partyId=
func (h *Handler) ListContracts(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	var partyID *int64
	if s := c.Query("partyId"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			partyID = &id
		}
	}
	items, err := h.repo.ListContracts(orgID, partyID)
	if err != nil {
		h.internal(c, "查询合同列表失败")
		return
	}
	if items == nil {
		items = []Contract{}
	}
	platform.SuccessResponse(c, items)
}

// CreateContract 新建合同/附件。
// POST /api/contracts  body {partyId, fileName, fileSize, mimeType, contractTitle, fileData}
func (h *Handler) CreateContract(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	var req CreateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	// 校验单位归属本组织
	pty, err := h.ptyRepo.FindPartyByID(req.PartyID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if pty == nil || pty.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "PARTY_NOT_FOUND", Message: "往来单位不存在",
		})
		return
	}

	// 解码 base64 data URL
	fileData, ok := decodeDataURL(req.FileData)
	if !ok {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "文件内容格式不合法",
		})
		return
	}
	if len(fileData) == 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "文件内容为空",
		})
		return
	}

	title := req.ContractTitle
	if title == "" {
		title = req.FileName
	}
	nc := &Contract{
		OrgID:         orgID,
		PartyID:       req.PartyID,
		FileName:      req.FileName,
		FileSize:      req.FileSize,
		MimeType:      req.MimeType,
		ContractTitle: title,
	}
	created, err := h.repo.CreateContract(nc, fileData)
	if err != nil {
		h.internal(c, "新建合同失败")
		return
	}
	_ = h.clRepo.LogCreate(orgID, "contract", created.ID)
	platform.SuccessResponse(c, created)
}

// GetContract 合同详情（含 fileData，用于下载/预览）。
// GET /api/contracts/:id
func (h *Handler) GetContract(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "合同 ID 不合法",
		})
		return
	}
	cnt, err := h.repo.FindContractByID(id)
	if err != nil {
		h.internal(c, "查询合同失败")
		return
	}
	if cnt == nil || cnt.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "CONTRACT_NOT_FOUND", Message: "合同不存在",
		})
		return
	}
	if len(cnt.FileBytes) > 0 {
		cnt.FileData = encodeDataURL(cnt.MimeType, cnt.FileBytes)
	}
	platform.SuccessResponse(c, cnt)
}

// DeleteContract 删除合同。
// DELETE /api/contracts/:id
func (h *Handler) DeleteContract(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "合同 ID 不合法",
		})
		return
	}
	hit, err := h.repo.DeleteContract(id, orgID)
	if err != nil {
		h.internal(c, "删除合同失败")
		return
	}
	if !hit {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "CONTRACT_NOT_FOUND", Message: "合同不存在",
		})
		return
	}
	_ = h.clRepo.LogChangeVoid(orgID, "contract", id, "delete", "normal", "deleted")
	platform.SuccessResponse(c, gin.H{"ok": true})
}

// decodeDataURL 解析 base64 data URL（形如 data:<mime>;base64,<payload>），返回原始字节。
func decodeDataURL(dataURL string) ([]byte, bool) {
	if idx := strings.Index(dataURL, ","); idx >= 0 {
		dataURL = dataURL[idx+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataURL))
	if err != nil {
		return nil, false
	}
	return raw, true
}

// encodeDataURL 将原始字节编码为 data URL（data:<mime>;base64,<b64>）。
func encodeDataURL(mime string, b []byte) string {
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}