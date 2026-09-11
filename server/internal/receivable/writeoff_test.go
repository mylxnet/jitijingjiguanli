package receivable

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// seedReceivableYear0 直接插入一张 recvYear=0 的应收单。
// （POST /api/receivables 会把 recvYear=0 默认成当前年，故「未填年度」只能直插。）
func seedReceivableYear0(t *testing.T, db *sql.DB, partyID, amountCents int64) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(
		`INSERT INTO receivable(org_id, party_id, recv_year, recv_kind, title, amount_cents, status, created_at, updated_at)
		 VALUES(1, ?, 0, 'other', '无年度欠款', ?, 'open', ?, ?)`,
		partyID, amountCents, now, now)
	if err != nil {
		t.Fatalf("插入无年度应收单失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// findReceipt 读取一条核销记录的关键字段。
func findReceipt(t *testing.T, db *sql.DB, id int64) (method string, amount int64, txnID *int64, status string) {
	t.Helper()
	var tid sql.NullInt64
	if err := db.QueryRow(
		`SELECT method, amount_cents, txn_id, status FROM receipt WHERE id = ?`, id,
	).Scan(&method, &amount, &tid, &status); err != nil {
		t.Fatalf("查核销记录失败: %v", err)
	}
	if tid.Valid {
		v := tid.Int64
		txnID = &v
	}
	return
}

// recView 应收单详情中的派生口径字段。
type recView struct {
	Status            string  `json:"status"`
	PaidCents         int64   `json:"paidCents"`
	WriteoffCents     int64   `json:"writeoffCents"`
	WriteoffDate      *string `json:"writeoffDate"`
	WriteoffNote      *string `json:"writeoffNote"`
	WriteoffReceiptID *int64  `json:"writeoffReceiptId"`
	OutstandingCents  int64   `json:"outstandingCents"`
}

// getDetail 通过 API 查询应收单详情（含派生口径）。
func getDetail(t *testing.T, r *gin.Engine, recID int64) recView {
	t.Helper()
	w := doJSON(t, r, "GET", "/api/receivables/"+itoa(recID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("查详情失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			Receivable recView `json:"receivable"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析详情失败: %v", err)
	}
	return out.Data.Receivable
}

// TestWriteoffReceiptFlow 坏账核销全流程：剩余待收全额清零、不生成流水、可撤销。
func TestWriteoffReceiptFlow(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	party := createPartyAPI(t, r, "某流转企业")

	// 应收 ¥1000（往年；坏账核销仅限本年度以前）
	rec := createReceivableAPI(t, r, party, "rent", "往年土地流转费", 100000, &incomeCat, platform.Now().Year()-1)

	// 先现金收 ¥300（产生 1 笔收入流水）
	if _, code := createReceiptAPI(t, r, rec, 30000, "2026-09-05", "cash", map[string]any{}); code != http.StatusOK {
		t.Fatalf("现金部分收款失败: %d", code)
	}
	if got := receivableStatus(t, db, rec); got != "open" {
		t.Fatalf("部分收款后应 open，实际 %s", got)
	}

	// 坏账核销：故意传一个巨大的 amountCents，应被忽略（后端恒取剩余待收）
	rid, code := createReceiptAPI(t, r, rec, 99999999, "2026-09-06", "writeoff", map[string]any{"note": "对方注销，无法收回"})
	if code != http.StatusOK {
		t.Fatalf("坏账核销失败: %d", code)
	}
	method, amount, txnID, status := findReceipt(t, db, rid)
	if method != "writeoff" {
		t.Errorf("核销方式应 writeoff，实际 %s", method)
	}
	if amount != 70000 {
		t.Errorf("坏账金额应为剩余待收 70000，实际 %d", amount)
	}
	if txnID != nil {
		t.Errorf("坏账核销不应关联流水，实际 txn_id=%d", *txnID)
	}
	if status != "normal" {
		t.Errorf("核销记录应 normal，实际 %s", status)
	}
	if got := receivableStatus(t, db, rec); got != "closed" {
		t.Errorf("坏账后应收单应 closed，实际 %s", got)
	}
	// 不产生新的现金流水：仍只有那 1 笔（现金部分收款）
	if n := countTxns(t, db, "income", "normal"); n != 1 {
		t.Errorf("坏账不应产生收入流水，normal 收入应仍 1 笔，实际 %d", n)
	}

	// 详情口径：已核销含坏账、坏账单列、待收为 0
	det := getDetail(t, r, rec)
	if det.PaidCents != 100000 {
		t.Errorf("已核销合计应 100000，实际 %d", det.PaidCents)
	}
	if det.WriteoffCents != 70000 {
		t.Errorf("坏账合计应 70000，实际 %d", det.WriteoffCents)
	}
	if det.OutstandingCents != 0 {
		t.Errorf("待收应 0，实际 %d", det.OutstandingCents)
	}
	if det.WriteoffReceiptID == nil || *det.WriteoffReceiptID != rid {
		t.Errorf("坏账核销记录 id 应 %d，实际 %v", rid, det.WriteoffReceiptID)
	}
	if det.WriteoffDate == nil || *det.WriteoffDate != "2026-09-06" {
		t.Errorf("坏账日期应 2026-09-06，实际 %v", det.WriteoffDate)
	}
	if det.WriteoffNote == nil || *det.WriteoffNote != "对方注销，无法收回" {
		t.Errorf("坏账原因不符，实际 %v", det.WriteoffNote)
	}

	// 列表接口同样返回坏账字段（坏账清单页依赖）
	lw := doJSON(t, r, "GET", "/api/receivables", nil)
	if lw.Code != http.StatusOK {
		t.Fatalf("应收列表失败: %d %s", lw.Code, lw.Body.String())
	}
	var lout struct {
		Data struct {
			Items []recView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &lout); err != nil {
		t.Fatalf("解析应收列表失败: %v", err)
	}
	listed := false
	for _, it := range lout.Data.Items {
		if it.WriteoffReceiptID != nil && *it.WriteoffReceiptID == rid && it.WriteoffNote != nil {
			listed = true
		}
	}
	if !listed {
		t.Errorf("应收列表应含坏账核销记录 id/原因（receiptId=%d）", rid)
	}

	// 撤销坏账核销 → 应收单退回 open、待收恢复、坏账归零
	w := doJSON(t, r, "PUT", "/api/receipts/"+itoa(rid), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废坏账核销失败: %d %s", w.Code, w.Body.String())
	}
	if got := receivableStatus(t, db, rec); got != "open" {
		t.Errorf("撤销坏账后应收单应 open，实际 %s", got)
	}
	det = getDetail(t, r, rec)
	if det.WriteoffCents != 0 {
		t.Errorf("撤销后坏账合计应 0，实际 %d", det.WriteoffCents)
	}
	if det.OutstandingCents != 70000 {
		t.Errorf("撤销后待收应恢复为 70000，实际 %d", det.OutstandingCents)
	}
	if det.WriteoffReceiptID != nil || det.WriteoffDate != nil || det.WriteoffNote != nil {
		t.Errorf("撤销后坏账字段应清空，实际 id=%v date=%v note=%v",
			det.WriteoffReceiptID, det.WriteoffDate, det.WriteoffNote)
	}
}

// TestWriteoffValidations 坏账核销校验：原因必填、已结清不可再核销。
func TestWriteoffValidations(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	party := createPartyAPI(t, r, "某流转企业")
	rec := createReceivableAPI(t, r, party, "rent", "往年土地流转费", 50000, &incomeCat, platform.Now().Year()-1)

	// 缺原因 → 400 BAD_DEBT_NOTE_REQUIRED
	w := doJSON(t, r, "POST", "/api/receivables/"+itoa(rec)+"/receipts",
		map[string]any{"receiptDate": "2026-09-06", "method": "writeoff"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺原因应 400，实际 %d %s", w.Code, w.Body.String())
	}
	if got := apiErr(t, w); got != "BAD_DEBT_NOTE_REQUIRED" {
		t.Errorf("错误码应 BAD_DEBT_NOTE_REQUIRED，实际 %s", got)
	}
	// 纯空白原因同样拒绝
	w = doJSON(t, r, "POST", "/api/receivables/"+itoa(rec)+"/receipts",
		map[string]any{"receiptDate": "2026-09-06", "method": "writeoff", "note": "   "})
	if w.Code != http.StatusBadRequest {
		t.Errorf("空白原因应 400，实际 %d", w.Code)
	}
	// 拒绝后不应产生任何核销记录
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM receipt WHERE receivable_id = ?`, rec).Scan(&n); err != nil {
		t.Fatalf("统计核销记录失败: %v", err)
	}
	if n != 0 {
		t.Errorf("校验拒绝后不应有核销记录，实际 %d", n)
	}

	// 正常核销（不传金额）→ 200 且结清
	if _, code := createReceiptAPI(t, r, rec, 0, "2026-09-06", "writeoff", map[string]any{"note": "坏账"}); code != http.StatusOK {
		t.Fatalf("坏账核销失败: %d", code)
	}
	// 已结清再核销 → 409 RECEIVABLE_CLOSED
	w = doJSON(t, r, "POST", "/api/receivables/"+itoa(rec)+"/receipts",
		map[string]any{"receiptDate": "2026-09-07", "method": "writeoff", "note": "再核一次"})
	if w.Code != http.StatusConflict {
		t.Errorf("已结清再核销应 409，实际 %d %s", w.Code, w.Body.String())
	}
	if got := apiErr(t, w); got != "RECEIVABLE_CLOSED" {
		t.Errorf("错误码应 RECEIVABLE_CLOSED，实际 %s", got)
	}
}

// TestWriteoffPartyOutstanding 坏账后该单位欠款合计应减少（closed 单被排除）。
func TestWriteoffPartyOutstanding(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	party := createPartyAPI(t, r, "欠款企业")

	rec := createReceivableAPI(t, r, party, "rent", "往年土地流转费", 60000, &incomeCat, platform.Now().Year()-1)
	// 一个 helper：读该单位欠款合计
	outstanding := func() int64 {
		t.Helper()
		w := doJSON(t, r, "GET", "/api/parties", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("查单位失败: %d", w.Code)
		}
		var out struct {
			Data []struct {
				ID               int64 `json:"id"`
				OutstandingCents int64 `json:"outstandingCents"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("解析单位列表失败: %v", err)
		}
		for _, p := range out.Data {
			if p.ID == party {
				return p.OutstandingCents
			}
		}
		return -1
	}

	if got := outstanding(); got != 60000 {
		t.Fatalf("坏账前欠款合计应 60000，实际 %d", got)
	}
	if _, code := createReceiptAPI(t, r, rec, 0, "2026-09-06", "writeoff", map[string]any{"note": "坏账"}); code != http.StatusOK {
		t.Fatalf("坏账核销失败: %d", code)
	}
	if got := outstanding(); got != 0 {
		t.Errorf("坏账后欠款合计应 0，实际 %d", got)
	}
}

// TestWriteoffYearRestriction 坏账核销仅限「本年度以前」：本年度拒绝、往年与未填年度允许。
func TestWriteoffYearRestriction(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	party := createPartyAPI(t, r, "年度企业")
	cur := platform.Now().Year()

	// 本年度应收 → 拒绝
	recCur := createReceivableAPI(t, r, party, "rent", "本年度欠款", 50000, &incomeCat, cur)
	w := doJSON(t, r, "POST", "/api/receivables/"+itoa(recCur)+"/receipts",
		map[string]any{"receiptDate": "2026-09-12", "method": "writeoff", "note": "坏账"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("本年度应拒绝，实际 %d %s", w.Code, w.Body.String())
	}
	if got := apiErr(t, w); got != "BAD_DEBT_YEAR_NOT_ALLOWED" {
		t.Errorf("错误码应 BAD_DEBT_YEAR_NOT_ALLOWED，实际 %s", got)
	}
	// 拒绝后不应有任何核销记录
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM receipt WHERE receivable_id = ?`, recCur).Scan(&n); err != nil {
		t.Fatalf("统计核销记录失败: %v", err)
	}
	if n != 0 {
		t.Errorf("本年度拒绝后不应有核销记录，实际 %d", n)
	}

	// 往年应收 → 允许
	recPast := createReceivableAPI(t, r, party, "rent", "往年欠款", 50000, &incomeCat, cur-1)
	if _, code := createReceiptAPI(t, r, recPast, 0, "2026-09-12", "writeoff", map[string]any{"note": "往年坏账"}); code != http.StatusOK {
		t.Fatalf("往年应收应允许核销，实际 %d", code)
	}
	if got := receivableStatus(t, db, recPast); got != "closed" {
		t.Errorf("往年坏账后应 closed，实际 %s", got)
	}

	// 未填年度（recvYear=0，需直插）→ 允许
	recNone := seedReceivableYear0(t, db, party, 30000)
	if _, code := createReceiptAPI(t, r, recNone, 0, "2026-09-12", "writeoff", map[string]any{"note": "无年度坏账"}); code != http.StatusOK {
		t.Fatalf("未填年度应收应允许核销，实际 %d", code)
	}
	if got := receivableStatus(t, db, recNone); got != "closed" {
		t.Errorf("未填年度坏账后应 closed，实际 %s", got)
	}
}
