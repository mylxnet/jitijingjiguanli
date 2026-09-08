// Package onboarding 提供引导页「下载模板 + 上传导入」功能（14-onboarding-import）。
//
// 上传的 Excel 含三张表：流转企业 / 投资公司 / 再投资。每行代表一个往来单位，
// 后端按行调用 receivable.CreateParty（复用其「建单位 + 联动建同名二级科目」），
// 并写入投资/再投资的本金期初到同名科目，返回逐行成功/失败汇总。
package onboarding

import (
	"bytes"
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/receivable"
)

// type 常量 === receivable.typeToL1 的键。
const (
	typeFlow     = "flow"
	typeInvest   = "invest"
	typeReinvest = "reinvest"
)

// Handler 引导页导入。
type Handler struct {
	db  *sql.DB
	rep *receivable.Repo
	cl  *changelog.Repo
}

// NewHandler 创建导入 handler。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		db:  db,
		rep: receivable.NewRepo(db),
		cl:  changelog.NewRepo(db),
	}
}

// Register 注册路由（authed 下调用）。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/onboarding/template", h.DownloadTemplate)
	r.POST("/api/onboarding/import", h.Import)
}

// ============ sheet 与列定义（生成模板与解析共用） ============

type sheetCol struct {
	name string
}

// sheet 标题 → 单位类型。
var sheetToType = map[string]string{
	"流转企业": typeFlow,
	"投资公司": typeInvest,
	"再投资": typeReinvest,
}

// typeToSheet 类型 → sheet 标题。
var typeToSheet = map[string]string{
	typeFlow:     "流转企业",
	typeInvest:   "投资公司",
	typeReinvest: "再投资",
}

// sheetHeaders 每张表的表头（与模板一致，解析时按此定位列）。
var sheetHeaders = map[string][]string{
	"流转企业": {"单位名称", "联系方式", "流转面积(亩)", "每亩年流转费(元)", "每亩管理费(元)"},
	"投资公司": {"单位名称", "联系方式", "投资本金(元)", "预期年回报率(%)"},
	"再投资": {"单位名称", "联系方式", "投资本金(元)", "预期年回报率(%)"},
}

// ============ 下载模板 ============

// DownloadTemplate 生成一份空模板 xlsx 供下载。
// GET /api/onboarding/template
func (h *Handler) DownloadTemplate(c *gin.Context) {
	buf, err := buildTemplate()
	if err != nil {
		platform.Fail(c, 500, "TEMPLATE_BUILD_FAILED", "模板生成失败")
		return
	}
	c.Header("Content-Disposition", `attachment; filename=基础数据导入模板.xlsx`)
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

func buildTemplate() (*bytes.Buffer, error) {
	f := excelize.NewFile()
	defer f.Close()

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F5C48"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	widths := map[string]map[int]float64{
		"流转企业": {1: 32, 2: 16, 3: 14, 4: 20, 5: 20},
		"投资公司": {1: 32, 2: 16, 3: 18, 4: 20},
		"再投资":  {1: 32, 2: 16, 3: 18, 4: 20},
	}

	// 只建表头 + 列宽，不预填数据行（避免用户直接上传时误把示例当数据导入；示例见「填写说明」页）。
	for _, sheet := range []string{"流转企业", "投资公司", "再投资"} {
		_, _ = f.NewSheet(sheet)
		hdrs := sheetHeaders[sheet]
		for i, hh := range hdrs {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, cell, hh)
		}
		colEnd, _ := excelize.CoordinatesToCellName(len(hdrs), 1)
		_ = f.SetCellStyle(sheet, "A1", colEnd, headerStyle)
		_ = f.SetRowHeight(sheet, 1, 22)
		for col, w := range widths[sheet] {
			name, _ := excelize.ColumnNumberToName(col)
			_ = f.SetColWidth(sheet, name, name, w)
		}
	}

	guide := "填写说明"
	_, _ = f.NewSheet(guide)
	guideRows := [][]string{
		{"规则", "说明"},
		{"三个 Sheet", "分别导入流转企业 / 投资公司 / 再投资，相互独立。"},
		{"自动建账", "每个单位保存后自动在对应科目下创建同名二级科目。"},
		{"联系方式", "选填，将同步写入该单位的联系方式字段。"},
		{"同类型重名", "同一 Sheet（同一来源类型）内单位名称不可重复；不同类型允许同名。"},
		{"流转企业列", "必填单位名称、流转面积(亩)；每亩流转费/管理费选填，填了则总费自动=面积×每亩。"},
		{"投资/再投资列", "必填单位名称、投资本金(元)（对外投出方向）;期初=投资本金；回报率选填，预期回报=本金×回报率。"},
		{"三类可空", "没有哪一类就把对应 Sheet 留空，跳过该类不建账。"},
		{"填写示例", "流转企业：一 蓝天农业合作社 120(亩) 800(每亩费) 50(每亩管理费)；投资公司：一 X科技有限公司 500000(元) 6(%)；再投资：一 集体再投资基金 300000(元) 5(%)。上传前请删除示例。"},
		{"使用前删除示例行", "标题行+示例行；示例仅作参考，回传前请清空。"},
	}
	for r, row := range guideRows {
		for col, v := range row {
			cell, _ := excelize.CoordinatesToCellName(col+1, r+1)
			_ = f.SetCellValue(guide, cell, v)
		}
	}
	_ = f.SetCellStyle(guide, "A1", "B1", headerStyle)
	_ = f.SetColWidth(guide, "A", "A", 20)
	_ = f.SetColWidth(guide, "B", "B", 85)
	_ = f.SetSheetVisible(guide, false)

	_ = f.DeleteSheet("Sheet1")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return &buf, nil
}