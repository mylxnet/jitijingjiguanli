package export

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// 列坐标工具
func colName(i int) string { // 0-based → A, B, ...
	name, _ := excelize.ColumnNumberToName(i + 1)
	return name
}

func cellRef(col, row int) string { // 1-based row, 0-based col
	return fmt.Sprintf("%s%d", colName(col), row)
}

// writeXLSX 将一张工作表渲染为 xlsx（免加工：表头、列宽、金额右对齐、合计行加粗）。
func writeXLSX(w io.Writer, s *xlSheet) error {
	f := excelize.NewFile()
	sheet := s.Name
	if _, err := f.NewSheet(sheet); err != nil {
		return fmt.Errorf("创建工作表失败: %w", err)
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "left"},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#EDEDED"}, Pattern: 1},
		Border:    []excelize.Border{{Type: "bottom", Color: "#999999", Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	numStyle, _ := f.NewStyle(&excelize.Style{
		NumFmt:    4, // #,##0.00
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})
	boldNumStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		NumFmt:    4,
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})
	boldTopNumStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Border:    []excelize.Border{{Type: "top", Color: "#000000", Style: 2}},
		NumFmt:    4,
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})
	boldStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	boldTopStyle, _ := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Border: []excelize.Border{{Type: "top", Color: "#000000", Style: 2}},
	})

	// 单元格样式：行级（bold/top）× 列级（金额）组合
	cellStyle := func(i int, r xlRow) int {
		num := false
		for _, n := range s.NumCols {
			if n == i {
				num = true
				break
			}
		}
		switch {
		case r.Bold && r.Top && num:
			return boldTopNumStyle
		case r.Bold && num:
			return boldNumStyle
		case num:
			return numStyle
		case r.Bold && r.Top:
			return boldTopStyle
		case r.Bold:
			return boldStyle
		default:
			return 0 // 普通文本无样式
		}
	}

	lastCol := len(s.Header) - 1
	row := 1

	// 大标题行
	if s.Title != "" {
		if err := f.SetCellValue(sheet, cellRef(0, row), s.Title); err != nil {
			return err
		}
		if err := f.MergeCell(sheet, cellRef(0, row), cellRef(lastCol, row)); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellRef(0, row), cellRef(lastCol, row), titleStyle); err != nil {
			return err
		}
		row++
	}
	// 表头
	for i, h := range s.Header {
		if err := f.SetCellValue(sheet, cellRef(i, row), h); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sheet, cellRef(0, row), cellRef(lastCol, row), headerStyle); err != nil {
		return err
	}
	// 冻结标题+表头行（此后写数据行）
	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      row,
		TopLeftCell: cellRef(0, row+1),
		ActivePane:  "bottomRight",
	}); err != nil {
		return err
	}
	row++

	// 数据行（分组小计行加粗、合计行上边框、金额列右对齐）
	writeRow := func(r xlRow) error {
		for i, c := range r.Cells {
			if c != nil {
				if err := f.SetCellValue(sheet, cellRef(i, row), c); err != nil {
					return err
				}
			}
			if style := cellStyle(i, r); style != 0 {
				if err := f.SetCellStyle(sheet, cellRef(i, row), cellRef(i, row), style); err != nil {
					return err
				}
			}
		}
		row++
		return nil
	}
	for _, r := range s.Rows {
		if err := writeRow(r); err != nil {
			return err
		}
	}
	for _, r := range s.FootRows {
		if err := writeRow(r); err != nil {
			return err
		}
	}

	// 列宽
	for i, wd := range s.Widths {
		if err := f.SetColWidth(sheet, colName(i), colName(i), wd); err != nil {
			return err
		}
	}

	if _, err := f.WriteTo(w); err != nil {
		return fmt.Errorf("写出 xlsx 失败: %w", err)
	}
	return nil
}
