package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

// writeCSV 将一张工作表渲染为 CSV（UTF-8 带 BOM，Excel 打开中文不乱码）。
func writeCSV(w io.Writer, s *xlSheet) error {
	// UTF-8 BOM，Excel 识别编码
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if s.Title != "" {
		if err := cw.Write([]string{s.Title}); err != nil {
			return err
		}
	}
	if err := cw.Write(s.Header); err != nil {
		return err
	}
	for _, row := range s.Rows {
		if err := cw.Write(cellsToText(row.Cells)); err != nil {
			return err
		}
	}
	for _, row := range s.FootRows {
		if err := cw.Write(cellsToText(row.Cells)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// cellsToText 把混合单元格转为文本（float64 → 两位小数）。
func cellsToText(cells []xlCell) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		switch v := c.(type) {
		case nil:
			out[i] = ""
		case float64:
			out[i] = strconv.FormatFloat(v, 'f', 2, 64)
		case string:
			out[i] = v
		default:
			out[i] = fmt.Sprint(v)
		}
	}
	return out
}
