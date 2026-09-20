package receivable

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	cat "jititaizhang/server/internal/category"

	"github.com/gin-gonic/gin"
)

// catBalance 通过 GET /api/categories 读取指定容器 L1 的余额（含其下所有二级）。
// 与前端「长期投资/再投资」页签的本金口径同源：L1 余额 = Σ 二级余额。
func catBalance(t *testing.T, r *gin.Engine, l1Name string) int64 {
	t.Helper()
	w := doJSON(t, r, "GET", "/api/categories", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("查科目树失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []struct {
			Name         string `json:"name"`
			BalanceCents *int64 `json:"balanceCents"`
			Children     []struct {
				Name         string `json:"name"`
				BalanceCents *int64 `json:"balanceCents"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析科目树失败: %v", err)
	}
	for _, c := range out.Data {
		if c.Name != l1Name {
			continue
		}
		if c.BalanceCents == nil {
			t.Fatalf("容器 %s 无余额字段", l1Name)
		}
		return *c.BalanceCents
	}
	t.Fatalf("科目树里没有容器 %s", l1Name)
	return 0
}

// txnCategoryPath 返回某笔流水入账科目的「L1名/L2名」路径。
func txnCategoryPath(t *testing.T, db *sql.DB, txnID int64) string {
	t.Helper()
	var l2, l1 string
	if err := db.QueryRow(
		`SELECT c2.name, c1.name FROM txn t
		 JOIN category c2 ON c2.id = t.category_id
		 JOIN category c1 ON c1.id = c2.parent_id
		 WHERE t.id = ?`, txnID).Scan(&l2, &l1); err != nil {
		t.Fatalf("查询流水 %d 入账科目失败: %v", txnID, err)
	}
	return fmt.Sprintf("%s/%s", l1, l2)
}

// TestReinvestDividendIncomeContainer 锁定收益两类的入账容器：
// reinvest_dividend 的现金收缴入「再投资收益」，dividend 入「投资收益」，
// 两者都不得碰本金容器（再投资 / 长期投资），否则投资页签的本金会被收益虚增。
func TestReinvestDividendIncomeContainer(t *testing.T) {
	db, r := newEnv(t)
	cat.NewHandler(db).Register(r.Group("", orgCtx()))

	seedL1(t, db, "长期投资")
	l1Reinvest := seedL1(t, db, "再投资")
	seedL1(t, db, "投资收益")
	seedL1(t, db, "再投资收益")

	// 再投资单位：建单位联动会在「再投资」下建同名二级（本金容器），期初 16834.45
	reinvestParty := createPartyWithType(t, r, "羊鸡福", "reinvest")
	if _, err := db.Exec(
		`UPDATE category SET opening_balance_cents=1683445
		 WHERE org_id=1 AND level=2 AND parent_id=? AND name='羊鸡福'`, l1Reinvest); err != nil {
		t.Fatalf("设置再投资本金期初失败: %v", err)
	}
	if got := catBalance(t, r, "再投资"); got != 1683445 {
		t.Fatalf("基线：再投资本金应 1683445，实际 %d", got)
	}

	// 再投资收益 ¥505.03：不预设入账科目 → 走自动解析（与页面「收缴」一致）
	recRI := createReceivableAPI(t, r, reinvestParty, "reinvest_dividend", "2025年再投资收益", 50503, nil, 2025)
	if w := doJSON(t, r, "POST", "/api/party-collect", map[string]any{
		"partyId": reinvestParty, "recvKind": "reinvest_dividend",
		"amountCents": 50503, "receiptDate": "2026-09-10",
	}); w.Code != http.StatusOK {
		t.Fatalf("再投资收益收缴失败: %d %s", w.Code, w.Body.String())
	}
	if got := receivableStatus(t, db, recRI); got != "closed" {
		t.Errorf("全额收缴后应收单应 closed，实际 %s", got)
	}

	// 生成 income 流水：应收单 #id 核销
	txnRI := latestIncomeTxn(t, db, 50503)
	if got := txnCategoryPath(t, db, txnRI); got != "再投资收益/羊鸡福" {
		t.Errorf("再投资收益入账容器应 再投资收益/羊鸡福，实际 %s", got)
	}
	if got := catBalance(t, r, "再投资收益"); got != 50503 {
		t.Errorf("再投资收益容器余额应 50503，实际 %d", got)
	}
	// 本金容器不受收益影响（这条断言就是本次改造要守住的口径）
	if got := catBalance(t, r, "再投资"); got != 1683445 {
		t.Errorf("再投资本金应仍为 1683445（被收益污染则变大），实际 %d", got)
	}

	// 回归：长期投资单位的 dividend 收缴仍落「投资收益」，不动「长期投资」
	investParty := createPartyWithType(t, r, "某合作社", "invest")
	recDI := createReceivableAPI(t, r, investParty, "dividend", "2025年长期投资收益", 110000, nil, 2025)
	if w := doJSON(t, r, "POST", "/api/party-collect", map[string]any{
		"partyId": investParty, "recvKind": "dividend",
		"amountCents": 110000, "receiptDate": "2026-09-10",
	}); w.Code != http.StatusOK {
		t.Fatalf("长期投资收益收缴失败: %d %s", w.Code, w.Body.String())
	}
	txnDI := latestIncomeTxn(t, db, 110000)
	if got := txnCategoryPath(t, db, txnDI); got != "投资收益/某合作社" {
		t.Errorf("长期投资收益入账容器应 投资收益/某合作社，实际 %s", got)
	}
	if got := catBalance(t, r, "投资收益"); got != 110000 {
		t.Errorf("投资收益容器余额应 110000，实际 %d", got)
	}
	if got := catBalance(t, r, "长期投资"); got != 0 {
		t.Errorf("长期投资本金容器应仍为 0，实际 %d", got)
	}
	if got := receivableStatus(t, db, recDI); got != "closed" {
		t.Errorf("全额收缴后应收单应 closed，实际 %s", got)
	}
}

// latestIncomeTxn 取最近一笔指定金额的正常收入流水 id。
func latestIncomeTxn(t *testing.T, db *sql.DB, amountCents int64) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(
		`SELECT id FROM txn WHERE amount_cents=? AND direction='income' AND status='normal'
		 ORDER BY id DESC LIMIT 1`, amountCents).Scan(&id); err != nil {
		t.Fatalf("未找到金额 %d 的收入流水: %v", amountCents, err)
	}
	return id
}
