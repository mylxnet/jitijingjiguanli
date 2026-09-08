package onboarding

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/receivable"
)

// 单位类型 → 需联动建同名二级科目的容器 L1（与 receivable 一致，双端共同约束）。
var containerL1 = map[string][]string{
	typeFlow:     {"土地流转费收入", "流转管理费"},
	typeInvest:   {"长期投资"},
	typeReinvest: {"再投资"},
}

// TypeSummary 单类型汇总。
type TypeSummary struct {
	Type    string `json:"type"`
	Created int    `json:"created"`
	Failed  int    `json:"failed"`
}

// ImportError 单行错误。
type ImportError struct {
	Type    string `json:"type"`
	Row     int    `json:"row"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// ImportResult 导入结果。
type ImportResult struct {
	Summary map[string]*TypeSummary `json:"summary"`
	Errors  []ImportError           `json:"errors"`
	Created int                     `json:"created"`
	Failed  int                     `json:"failed"`
}

// Import 解析上传的 Excel 并按行建账。
// POST /api/onboarding/import  (multipart: file)
func (h *Handler) Import(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		platform.Fail(c, http.StatusBadRequest, "NO_FILE", "请选择要上传的 Excel 文件")
		return
	}

	src, err := fh.Open()
	if err != nil {
		platform.Fail(c, http.StatusInternalServerError, "OPEN_FILE_FAILED", "读取上传文件失败")
		return
	}
	defer src.Close()

	fb, err := excelize.OpenReader(src)
	if err != nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_EXCEL", "文件不是有效的 Excel (.xlsx)，请使用模板下载后填写")
		return
	}
	defer fb.Close()

	result := &ImportResult{Summary: map[string]*TypeSummary{}}
	initSummary := func(t string) *TypeSummary {
		s, ex := result.Summary[t]
		if !ex {
			s = &TypeSummary{Type: t}
			result.Summary[t] = s
		}
		return s
	}

	// 同 sheet 内已看到的名称（防止一次文件内重复，跨类型允许同名）。
	seen := map[string]map[string]bool{}

	for sheet, ptype := range sheetToType {
		hdrs, ok := sheetHeaders[sheet]
		if !ok {
			continue
		}
		if _, ok := seen[ptype]; !ok {
			seen[ptype] = map[string]bool{}
		}
		rows, err := fb.GetRows(sheet)
		if err != nil {
			continue // 该 sheet 不存在 → 视为留空
		}
		for i := 1; i < len(rows); i++ {
			cells := rows[i]
			name := strings.TrimSpace(cell(cells, 0))
			if name == "" {
				continue // 空行跳过
			}
			sheetRow := i + 1 // 表头占第 1 行，数据从第 2 行起
			fail := func(msg string) {
				result.Errors = append(result.Errors, ImportError{
					Type: ptype, Row: sheetRow, Name: name, Message: msg,
				})
				initSummary(ptype).Failed++
				result.Failed++
			}

			if seen[ptype][name] {
				fail("同表内单位名称重复")
				continue
			}
			if exact, dupes := h.rep.FindDuplicate(orgID, name, ptype); exact {
				fail("该类型下已存在同名单位")
				continue
			} else if len(dupes) > 0 {
				fail("与已有单位极为相近：请修改名称后再试")
				continue
			}

			p := &receivable.Party{
				OrgID: orgID, Name: name, Type: ptype,
				ContactPhone: strings.TrimSpace(cell(cells, 1)),
			}
			if err := fillFields(hdrs, ptype, cells, p); err != nil {
				fail(err.Error())
				continue
			}

			created, err := h.rep.CreateParty(p)
			if err != nil {
				fail("保存失败：" + friendlyError(err))
				continue
			}

			// 投资/再投资：把本金同步到同名科目的期初（对外投出存量）。
			if (ptype == typeInvest || ptype == typeReinvest) && created.InvestAmountCents > 0 {
				for _, l1 := range containerL1[ptype] {
					h.applyOpening(orgID, l1, created.Name, created.InvestAmountCents)
				}
			}

			seen[ptype][name] = true
			h.cl.LogCreate(orgID, "party", created.ID)
			initSummary(ptype).Created++
			result.Created++
		}
	}

	platform.OK(c, result)
}

// fillFields 按表头定位列并填业务字段（金额按元 → 分）。
func fillFields(hdrs []string, ptype string, cells []string, p *receivable.Party) error {
	switch ptype {
	case typeFlow:
		area, err := parseFloat(cell(cells, hdrIndex(hdrs, "流转面积(亩)")))
		if err != nil {
			return errors.New("流转面积需为有效数字")
		}
		if area < 0 {
			return errors.New("流转面积不能为负数")
		}
		p.AreaMu = area
		p.LandMu = area

		landFee, _ := parseFloat(cell(cells, hdrIndex(hdrs, "每亩年流转费(元)")))
		mgmtFee, _ := parseFloat(cell(cells, hdrIndex(hdrs, "每亩管理费(元)")))
		p.LandFeePerMuCents = yuanToCents(landFee)
		p.MgmtFeePerMuCents = yuanToCents(mgmtFee)
		// 总费 = 面积 × 每亩（元）→ 分（未填每亩则总额为 0）
		p.ExpectedLandFeeCents = int64(math.Round(area * landFee * 100))
		p.ExpectedMgmtFeeCents = int64(math.Round(area * mgmtFee * 100))
	case typeInvest, typeReinvest:
		amount, err := parseFloat(cell(cells, hdrIndex(hdrs, "投资本金(元)")))
		if err != nil {
			return errors.New("投资本金需为有效数字")
		}
		if amount < 0 {
			return errors.New("投资本金不能为负数")
		}
		p.InvestAmountCents = yuanToCents(amount)
		rate, _ := parseFloat(cell(cells, hdrIndex(hdrs, "预期年回报率(%)")))
		if rate < 0 {
			return errors.New("预期年回报率不能为负数")
		}
		p.ReturnRateBps = int(math.Round(rate * 100)) // % → bps（6% → 600）
		// 预期回报 = 本金(分) × 回报率(%) / 100
		p.ExpectedReturnCents = int64(math.Round(float64(p.InvestAmountCents) * rate / 100))
	default:
		return nil
	}
	return nil
}

// applyOpening 把期初写入组织内「l1Name 下名为 l2Name 的二级科目」。
func (h *Handler) applyOpening(orgID int64, l1Name, l2Name string, opening int64) {
	var l1id int64
	if err := h.db.QueryRow(
		`SELECT id FROM category WHERE org_id=? AND name=? AND level=1 AND status='active'`,
		orgID, l1Name,
	).Scan(&l1id); err != nil {
		return
	}
	var l2id int64
	if err := h.db.QueryRow(
		`SELECT id FROM category WHERE org_id=? AND name=? AND parent_id=?`,
		orgID, l2Name, l1id,
	).Scan(&l2id); err != nil {
		return
	}
	_, _ = h.db.Exec(`UPDATE category SET opening_balance_cents=? WHERE id=?`, opening, l2id)
}

// -------- helper --------

// hdrIndex 返回表头列下标，找不到返回 -1。
func hdrIndex(hdrs []string, target string) int {
	if len(hdrs) == 0 {
		return -1
	}
	for i, hh := range hdrs {
		if strings.TrimSpace(hh) == target {
			return i
		}
	}
	return -1
}

func cell(cells []string, i int) string {
	if i < 0 || i >= len(cells) {
		return ""
	}
	return cells[i]
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil // 空视为 0
	}
	return strconv.ParseFloat(s, 64)
}

func yuanToCents(y float64) int64 {
	return int64(math.Round(y * 100))
}

func friendlyError(err error) string {
	// 未知 DB 错误给出通用文案。
	return "创建单位失败"
}

// unauthorized 统一未授权响应。
func (h *Handler) unauthorized(c *gin.Context) {
	platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或会话已失效")
}