package receivable

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// seedL1 插入一级科目容器（测试库无预置科目，需显式建）。
func seedL1(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(
		`INSERT INTO category(org_id, name, level, parent_id, status, kind, preset, sort_order, created_at, updated_at)
		 VALUES(1, ?, 1, NULL, 'active', 'equity', 1, 0, ?, ?)`, name, now, now)
	if err != nil {
		t.Fatalf("插入一级科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// findCatID 按 L1 名 + L2 名查科目 id；不存在返回 0。
func findCatID(t *testing.T, db *sql.DB, l1, l2 string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(
		`SELECT c.id FROM category c JOIN category p ON p.id=c.parent_id
		 WHERE c.org_id=1 AND p.name=? AND c.name=?`, l1, l2).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	if err != nil {
		t.Fatalf("查科目失败: %v", err)
	}
	return id
}

// countRows 计数。
func countRows(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	return n
}

// catIDOfTxn 读取一笔流水的科目 id。
func catIDOfTxn(t *testing.T, db *sql.DB, txnID int64) int64 {
	t.Helper()
	var cid int64
	if err := db.QueryRow(`SELECT category_id FROM txn WHERE id=?`, txnID).Scan(&cid); err != nil {
		t.Fatalf("查流水科目失败: %v", err)
	}
	return cid
}

// createPartyTypedAt 指定类型建单位，返回 id。
func createPartyTypedAt(t *testing.T, r *gin.Engine, name, ptype string) int64 {
	t.Helper()
	w := doJSON(t, r, "POST", "/api/parties", map[string]any{"name": name, "type": ptype})
	if w.Code != http.StatusOK {
		t.Fatalf("建单位(%s)失败: %d %s", ptype, w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析建单位响应失败: %v", err)
	}
	return out.Data.ID
}

// 干净单位（无流水、无欠款）→ 可删；科目一并删除；不建归档。
func TestDeletePartyClean(t *testing.T) {
	db, r := newEnv(t)
	seedL1(t, db, "土地流转费收入")
	seedL1(t, db, "流转管理费")
	pid := createPartyAPI(t, r, "甲单位")

	if findCatID(t, db, "土地流转费收入", "甲单位") == 0 {
		t.Fatal("建单位未联动建同名科目")
	}

	w := doJSON(t, r, "DELETE", "/api/parties/"+itoa(pid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("删除应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party WHERE id=?`, pid) != 0 {
		t.Error("单位未删除")
	}
	if findCatID(t, db, "土地流转费收入", "甲单位") != 0 || findCatID(t, db, "流转管理费", "甲单位") != 0 {
		t.Error("同名二级科目未删除")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM category WHERE name='历史归档'`) != 0 {
		t.Error("无流水时不应创建历史归档科目")
	}
}

// 有未结清应收 → 拒绝。
func TestDeletePartyBlockedByDebt(t *testing.T) {
	db, r := newEnv(t)
	seedL1(t, db, "土地流转费收入")
	pid := createPartyAPI(t, r, "乙单位")
	createReceivableAPI(t, r, pid, "rent", "2025年度流转费", 50000, nil, 2025)

	w := doJSON(t, r, "DELETE", "/api/parties/"+itoa(pid), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("有欠款应 409，实际 %d %s", w.Code, w.Body.String())
	}
	if got := apiErr(t, w); got != "PARTY_HAS_DEBT" {
		t.Errorf("错误码应 PARTY_HAS_DEBT，实际 %s", got)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party WHERE id=?`, pid) != 1 {
		t.Error("被拒后单位不应被删除")
	}
}

// 科目余额非 0 → 拒绝。
func TestDeletePartyBlockedByBalance(t *testing.T) {
	db, r := newEnv(t)
	seedL1(t, db, "长期投资")
	pid := createPartyTypedAt(t, r, "丙公司", "invest")
	catID := findCatID(t, db, "长期投资", "丙公司")
	if catID == 0 {
		t.Fatal("invest 单位应有同名科目")
	}
	seedTxn(t, db, "2025-01-01", "expense", 50000, catID) // 余额 -50000 ≠ 0

	w := doJSON(t, r, "DELETE", "/api/parties/"+itoa(pid), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("余额非0应 409，实际 %d %s", w.Code, w.Body.String())
	}
	if got := apiErr(t, w); got != "PARTY_HAS_BALANCE" {
		t.Errorf("错误码应 PARTY_HAS_BALANCE，实际 %s", got)
	}
}

// 有历史流水但余额恰为 0 → 可删；流水保留并改挂历史归档；单位科目真删。
func TestDeletePartyMigratesTxns(t *testing.T) {
	db, r := newEnv(t)
	seedL1(t, db, "长期投资")
	pid := createPartyTypedAt(t, r, "丁公司", "invest")
	catID := findCatID(t, db, "长期投资", "丁公司")

	t1 := seedTxn(t, db, "2025-01-01", "expense", 50000, catID)
	t2 := seedTxn(t, db, "2025-06-01", "income", 50000, catID)

	w := doJSON(t, r, "DELETE", "/api/parties/"+itoa(pid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("余额0应可删，实际 %d %s", w.Code, w.Body.String())
	}

	if countRows(t, db, `SELECT COUNT(*) FROM txn WHERE id IN (?, ?)`, t1, t2) != 2 {
		t.Error("流水不应被删除")
	}
	arch := findCatID(t, db, "历史归档", "已删除单位")
	if arch == 0 {
		t.Fatal("有流水时应创建历史归档科目")
	}
	if catIDOfTxn(t, db, t1) != arch || catIDOfTxn(t, db, t2) != arch {
		t.Error("流水应改挂到历史归档科目")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM category WHERE id=?`, catID) != 0 {
		t.Error("单位同名科目应被删除")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party WHERE id=?`, pid) != 0 {
		t.Error("单位应被删除")
	}
}

// 坏账核销（无现金）使应收结清 → 可删；应收单与核销记录一并删除。
func TestDeletePartyWithWrittenOffReceivable(t *testing.T) {
	db, r := newEnv(t)
	seedL1(t, db, "流转管理费")
	pid := createPartyAPI(t, r, "戊单位")
	rec := createReceivableAPI(t, r, pid, "service", "2025年度管理费", 40000, nil, 2025)
	if _, code := createReceiptAPI(t, r, rec, 0, "2026-09-12", "writeoff", map[string]any{"note": "坏账"}); code != http.StatusOK {
		t.Fatalf("坏账核销失败: %d", code)
	}

	w := doJSON(t, r, "DELETE", "/api/parties/"+itoa(pid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("坏账已结清应可删，实际 %d %s", w.Code, w.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM receivable WHERE party_id=?`, pid) != 0 {
		t.Error("应收单应一并删除")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM receipt WHERE receivable_id=?`, rec) != 0 {
		t.Error("核销记录应一并删除")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party WHERE id=?`, pid) != 0 {
		t.Error("单位应被删除")
	}
}

// 同名异类型单位互不误删。
func TestDeletePartySameNameOtherTypeKept(t *testing.T) {
	db, r := newEnv(t)
	seedL1(t, db, "土地流转费收入")
	seedL1(t, db, "长期投资")
	flowPid := createPartyTypedAt(t, r, "同名", "flow")
	investPid := createPartyTypedAt(t, r, "同名", "invest")
	if findCatID(t, db, "长期投资", "同名") == 0 {
		t.Fatal("invest 单位应有同名科目")
	}

	w := doJSON(t, r, "DELETE", "/api/parties/"+itoa(flowPid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("删除 flow 单位失败: %d %s", w.Code, w.Body.String())
	}
	if findCatID(t, db, "土地流转费收入", "同名") != 0 {
		t.Error("flow 单位科目应被删除")
	}
	if findCatID(t, db, "长期投资", "同名") == 0 {
		t.Error("invest 单位科目不应被误删")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party WHERE id=?`, investPid) != 1 {
		t.Error("invest 单位不应被删除")
	}
}

// 单位不存在 → 404。
func TestDeletePartyNotFound(t *testing.T) {
	_, r := newEnv(t)
	w := doJSON(t, r, "DELETE", "/api/parties/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("不存在应 404，实际 %d", w.Code)
	}
}
