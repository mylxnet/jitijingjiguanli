package contract

import (
	"database/sql"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/receivable"
)

// dateLayout 合同期至时间格式（与表内 TEXT 约定一致）。
const dateLayout = "2006-01-02"

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
	r.GET("/api/contracts/expiring", h.ListExpiring) // 注意：须在 /:id 之前注册
	r.GET("/api/contracts/:id", h.GetContract)
	r.GET("/api/contracts/:id/text", h.GetContractText) // 老式 .doc 正文文本提取
	r.PUT("/api/contracts/:id/expiry", h.UpdateContractExpiry)
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

// ListExpiring 到期合同清单：返回「已到期」或「30 天内即将到期」的合同，关联单位信息与剩余天数。
// GET /api/contracts/expiring
func (h *Handler) ListExpiring(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	rows, err := h.repo.ListExpiring(orgID)
	if err != nil {
		h.internal(c, "查询到期合同失败")
		return
	}

	now := platform.Now()
	loc := now.Location()
	today, _ := time.ParseInLocation(dateLayout, now.Format(dateLayout), loc)

	out := []ExpiringItem{}
	for _, it := range rows {
		exp, err := time.ParseInLocation(dateLayout, it.ExpiresAt, loc)
		if err != nil {
			continue
		}
		days := int64(exp.Sub(today).Hours() / 24)
		if days > 30 {
			continue // 只留「已到期或 30 天内即将到期」
		}
		it.HasExpired = days < 0
		it.DaysUntil = days
		out = append(out, it)
	}
	platform.SuccessResponse(c, out)
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

	// 校验「合同期至时间」：可选，非空须为 YYYY-MM-DD
	var expires *string
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		if _, err := time.Parse(dateLayout, *req.ExpiresAt); err != nil {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "合同期至时间格式不合法，应为 YYYY-MM-DD",
			})
			return
		}
		expires = req.ExpiresAt
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
		ExpiresAt:     expires,
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

// GetContractText 读取合同二进制并提取封装在老式 .doc（OLE2）里的正文文本，供前端文本预览。
// GET /api/contracts/:id/text  →  {text:string, supported:bool}
func (h *Handler) GetContractText(c *gin.Context) {
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
	text, extracted := extractDocText(cnt.FileBytes)
	platform.SuccessResponse(c, gin.H{"text": text, "supported": extracted})
}

// extractDocText 从老式 .doc（OLE2）二进制中尽量提取正文文本。
// 原理：Word 正文常以 UTF-16LE 连续码元存储在文件内，将其按 2 字节扫描，
// 只收集「可打印 ASCII + CJK + 常用全角标点」的连续片段，过滤掉结构残留噪声。
// .doc 二进制仅在提取到足够长度的连续文本时视为 supported。
func extractDocText(raw []byte) (text string, ok bool) {
	if len(raw) == 0 {
		return "", false
	}
	// OLE2/CFB 魔数 d0cf11e0a1b11ae1，非 .doc 二进制不进文本提取
	if len(raw) < 8 || !(raw[0] == 0xd0 && raw[1] == 0xcf && raw[2] == 0x11 && raw[3] == 0xe0) {
		return "", false
	}
	n := len(raw) - 1
	var sb strings.Builder
	seg := make([]byte, 0, 512)
	flush := func() {
		if len(seg) >= 8 { // 至少 4 个连续可读码元（8 字节）才作为正文片段
			s := decodeUTF16LE(seg)
			for _, r := range strings.Fields(s) {
				sb.WriteString(r)
				sb.WriteByte('\n')
			}
		}
		seg = seg[:0]
	}
	for i := 0; i+1 < n; i += 2 {
		v := uint16(raw[i]) | uint16(raw[i+1])<<8
		if isDocPrintable(v) {
			seg = append(seg, raw[i], raw[i+1])
		} else {
			flush()
		}
	}
	flush()
	if sb.Len() < 8 {
		return "", false
	}
	return sb.String(), true
}

// isDocPrintable 判断 UTF-16LE 码元是否为可作正文的字符（ASCII/CJK/全角标点/空格）。
func isDocPrintable(v uint16) bool {
	switch {
	case v == 0: // 空分隔
		return false
	case v >= 0x20 && v < 0x7E: // 可打印 ASCII（含空格）
		return true
	case v >= 0x4E00 && v <= 0x9FFF: // CJK 统一表意文字
		return true
	case v >= 0x3000 && v <= 0x303F: // CJK 标点
		return true
	case v >= 0xFF00 && v <= 0xFFEF: // 全角标点
		return true
	}
	switch v {
	case 0x2018, 0x2019, 0x201C, 0x201D, 0x2026, 0x300A, 0x300B:
		return true
	}
	return false
}

// decodeUTF16LE 将 UTF-16LE 字节解码为字符串（容错替换非法码元）。
func decodeUTF16LE(b []byte) string {
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		runes = append(runes, rune(uint16(b[i]) | uint16(b[i+1])<<8))
	}
	return string(runes)
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

// UpdateContractExpiry 修改合同「合同期至时间」，可清除（置空）。记 changelog。
// PUT /api/contracts/:id/expiry  body {expiresAt?: string|null}
func (h *Handler) UpdateContractExpiry(c *gin.Context) {
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
	var req UpdateContractExpiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}
	// 非空须为 YYYY-MM-DD；空串/nil = 清除到期
	var expires *string
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		if _, err := time.Parse(dateLayout, *req.ExpiresAt); err != nil {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "合同期至时间格式不合法，应为 YYYY-MM-DD",
			})
			return
		}
		expires = req.ExpiresAt
	}

	// 读旧值并校验归属（复用 FindContractByID，忽略其 fileData）
	oldC, err := h.repo.FindContractByID(id)
	if err != nil {
		h.internal(c, "查询合同失败")
		return
	}
	if oldC == nil || oldC.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "CONTRACT_NOT_FOUND", Message: "合同不存在",
		})
		return
	}
	oldVal := ""
	if oldC.ExpiresAt != nil {
		oldVal = *oldC.ExpiresAt
	}
	newVal := ""
	if expires != nil {
		newVal = *expires
	}
	if oldVal != newVal {
		hit, err := h.repo.UpdateExpiry(id, orgID, expires)
		if err != nil {
			h.internal(c, "更新合同到期日失败")
			return
		}
		if !hit {
			platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
				Code: "CONTRACT_NOT_FOUND", Message: "合同不存在",
			})
			return
		}
		_ = h.clRepo.LogUpdateField(orgID, "contract", id, "expires_at", oldVal, newVal)
	}
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