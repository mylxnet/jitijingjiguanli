package receivable

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// partyRecvRow 是 GET /api/parties 中与投资页「已收」统计卡/列表列对应的字段子集。
type partyRecvRow struct {
	Name                  string `json:"name"`
	OutstandingCents      int64  `json:"outstandingCents"`
	DividendReceivedCents int64  `json:"dividendReceivedCents"`
	ReinvestReceivedCents int64  `json:"reinvestReceivedCents"`
}

// partiesByName 拉一次往来单位列表，按单位名索引。
func partiesByName(t *testing.T, r *gin.Engine) map[string]partyRecvRow {
	t.Helper()
	w := doJSON(t, r, "GET", "/api/parties", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("查往来单位失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []partyRecvRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析往来单位列表失败: %v", err)
	}
	m := make(map[string]partyRecvRow, len(out.Data))
	for _, p := range out.Data {
		m[p.Name] = p
	}
	return m
}

// TestPartyReceivedReturns 锁定「已收投资收益 / 已收再投资收益」口径：
// 只累计对应 recv_kind 应收下的 normal 核销；坏账冲销与作废核销都不算；
// 抵销（offset，不产生收入流水）要算；跨年度累计；其他 kind 不串味。
func TestPartyReceivedReturns(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "投资收益")
	expenseCat := seedIncomeCat(t, db, "收益分红发放")
	party := createPartyAPI(t, r, "某投资公司")

	// 1) 2024 年投资收益 ¥500 全额现金收到（应收单转 closed）→ 计入，证明不看 receivable.status
	recA := createReceivableAPI(t, r, party, "dividend", "2024年投资收益", 50000, &incomeCat, 2024)
	createReceiptAPI(t, r, recA, 50000, "2025-01-10", "cash", map[string]any{})

	// 2) 2025 年投资收益 ¥100 抵销核销 → 计入（这类收款不产生收入流水，用科目余额口径会少算）
	recB := createReceivableAPI(t, r, party, "dividend", "2025年投资收益", 10000, &incomeCat, 2025)
	offTxn := seedTxn(t, db, "2026-01-05", "expense", 10000, expenseCat)
	if _, code := createReceiptAPI(t, r, recB, 10000, "2026-01-05", "offset", map[string]any{"txnId": offTxn}); code != http.StatusOK {
		t.Fatalf("抵销核销失败: %d", code)
	}

	// 3) 投资收益 ¥300 走坏账冲销 → 不计入（不是收到钱）
	recC := createReceivableAPI(t, r, party, "dividend", "注销收不回", 30000, &incomeCat, 2023)
	createReceiptAPI(t, r, recC, 30000, "2026-02-01", "writeoff", map[string]any{"note": "对方注销"})

	// 4) 投资收益 ¥50 先现金收、再作废核销 → 不计入
	recD := createReceivableAPI(t, r, party, "dividend", "收后退了", 5000, &incomeCat, 2026)
	rid, code := createReceiptAPI(t, r, recD, 5000, "2026-03-01", "cash", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("现金收款失败: %d", code)
	}
	if w := doJSON(t, r, "PUT", "/api/receipts/"+itoa(rid), nil); w.Code != http.StatusOK {
		t.Fatalf("作废核销失败: %d %s", w.Code, w.Body.String())
	}

	// 5) 再投资收益 ¥150 现金收到 → 只进 reinvest 字段，不与 dividend 合并
	recE := createReceivableAPI(t, r, party, "reinvest_dividend", "2025年再投资收益", 15000, &incomeCat, 2025)
	createReceiptAPI(t, r, recE, 15000, "2026-04-01", "cash", map[string]any{})

	// 6) 土地流转费 ¥80 现金收到 → 两个字段都不该动（kind 过滤生效）
	recF := createReceivableAPI(t, r, party, "rent", "2026年流转费", 8000, &incomeCat, 2026)
	createReceiptAPI(t, r, recF, 8000, "2026-05-01", "cash", map[string]any{})

	got, ok := partiesByName(t, r)["某投资公司"]
	if !ok {
		t.Fatalf("往来单位列表里没找到「某投资公司」")
	}
	if got.DividendReceivedCents != 60000 {
		t.Errorf("已收投资收益应 60000（现金 50000 + 抵销 10000；坏账 30000 与作废 5000 不算），实际 %d", got.DividendReceivedCents)
	}
	if got.ReinvestReceivedCents != 15000 {
		t.Errorf("已收再投资收益应 15000，实际 %d", got.ReinvestReceivedCents)
	}
	// 未收仍只统计 open 单：抵销那笔也 closed，作废那笔退回 open（5000）+ 流转费已结清
	if got.OutstandingCents != 5000 {
		t.Errorf("欠款合计应 5000（作废后退回未收的那笔），实际 %d", got.OutstandingCents)
	}
}
