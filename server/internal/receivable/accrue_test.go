package receivable

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestBatchAccrue 批量计提：同年度防重。
func TestBatchAccrue(t *testing.T) {
	db, r := newEnv(t)
	pa := createPartyAPI(t, r, "甲公司")
	pb := createPartyAPI(t, r, "乙公司")

	w := doJSON(t, r, "POST", "/api/receivables/batch", map[string]any{
		"recvYear": 2026,
		"title":    "2026年度土地流转费",
		"items": []map[string]any{
			{"partyId": pa, "recvKind": "rent", "amountCents": 100000},
			{"partyId": pb, "recvKind": "rent", "amountCents": 80000},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("批量计提失败: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			Created int `json:"created"`
			Skipped int `json:"skipped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if created.Data.Created != 2 || created.Data.Skipped != 0 {
		t.Errorf("首次应 created=2 skipped=0，实际 %+v", created.Data)
	}

	// 再次结转：全部跳过
	w = doJSON(t, r, "POST", "/api/receivables/batch", map[string]any{
		"recvYear": 2026, "title": "2026年度土地流转费",
		"items": []map[string]any{{"partyId": pa, "recvKind": "rent", "amountCents": 100000}},
	})
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if created.Data.Created != 0 || created.Data.Skipped != 1 {
		t.Errorf("重复应 created=0 skipped=1，实际 %+v", created.Data)
	}

	// 年度筛选
	var list struct {
		Data struct {
			Items []Receivable `json:"items"`
			Total int          `json:"total"`
		} `json:"data"`
	}
	w = doJSON(t, r, "GET", "/api/receivables?year=2026&kind=rent", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if list.Data.Total != 2 {
		t.Errorf("2026 rent 应收应有 2 张，实际 %d", list.Data.Total)
	}
	for _, it := range list.Data.Items {
		if it.RecvYear != 2026 {
			t.Errorf("应收年度应 2026，实际 %d", it.RecvYear)
		}
	}
	_ = db
}

// TestStandardAccrue 计提标准 + 一键结转 + 启停。
func TestStandardAccrue(t *testing.T) {
	db, r := newEnv(t)
	pa := createPartyAPI(t, r, "甲公司")
	pb := createPartyAPI(t, r, "乙公司")

	save := func(partyID int64, amount int64) {
		t.Helper()
		w := doJSON(t, r, "POST", "/api/recv-standards", map[string]any{
			"partyId": partyID, "recvKind": "rent", "amountCents": amount,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("保存标准失败: %d %s", w.Code, w.Body.String())
		}
	}
	save(pa, 100000)
	save(pb, 80000)

	// 一键结转 2026
	w := doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{
		"year": 2026, "kind": "rent", "title": "2026年度土地流转费",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("一键结转失败: %d %s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct {
			Created int `json:"created"`
			Skipped int `json:"skipped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Data.Created != 2 {
		t.Errorf("一键结转应 created=2，实际 %+v", res.Data)
	}

	// 重复结转：跳过
	w = doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{
		"year": 2026, "kind": "rent", "title": "2026年度土地流转费",
	})
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Data.Skipped != 2 {
		t.Errorf("重复结转应 skipped=2，实际 %+v", res.Data)
	}

	// 停用乙 → 2027 只给甲结转
	var stds []AccrualStandard
	w = doJSON(t, r, "GET", "/api/recv-standards?kind=rent", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &struct{ Data *[]AccrualStandard }{&stds}); err != nil {
		t.Fatalf("解析标准失败: %v", err)
	}
	for _, s := range stds {
		if s.PartyName == "乙公司" {
			w = doJSON(t, r, "PUT", "/api/recv-standards/"+itoa(s.ID), map[string]any{"active": false})
			if w.Code != http.StatusOK {
				t.Fatalf("停用标准失败: %d %s", w.Code, w.Body.String())
			}
		}
	}
	w = doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{
		"year": 2027, "kind": "rent", "title": "2027年度土地流转费",
	})
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Data.Created != 1 || res.Data.Skipped != 0 {
		t.Errorf("2027 应 created=1（乙停用不参与尝试，skipped=0），实际 %+v", res.Data)
	}
	_ = db
}
