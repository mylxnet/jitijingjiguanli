package receivable

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

func urlQuery(s string) string { return url.QueryEscape(s) }

func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "receivable.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	seedTestOrg(t, db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("", orgCtx())
	NewHandler(db).Register(authed)
	return db, r
}

func seedTestOrg(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO org(id, name, created_at, updated_at) VALUES(1, '测试组织', '2026-09-02', '2026-09-02')`); err != nil {
		t.Fatalf("插入测试组织失败: %v", err)
	}
}

func orgCtx() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("orgID", int64(1))
		c.Set("userID", int64(1))
		c.Next()
	}
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("编码请求失败: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func apiErr(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析错误体失败: %v body=%s", err, w.Body.String())
	}
	return out.Error.Code
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

// seedParty 直接插入往来单位。
func seedParty(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(
		`INSERT INTO party(org_id, name, note, created_at, updated_at) VALUES(1,?,NULL,?,?)`,
		name, now, now)
	if err != nil {
		t.Fatalf("插入往来单位失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedTxn 直接插入一笔银行收支流水。
func seedTxn(t *testing.T, db *sql.DB, date, direction string, amountCents, catID int64) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(
		`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		 VALUES(1,?,?,?,?,NULL,'normal',?,?)`, date, direction, amountCents, catID, now, now)
	if err != nil {
		t.Fatalf("插入流水失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func seedIncomeCat(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	now := time.Now().UTC()
	// 一级分组 + 权益二级（v0.4）
	res1, err := db.Exec(
		`INSERT INTO category(org_id, name, level, parent_id, status, kind, sort_order, created_at, updated_at)
		 VALUES(1,?,1,NULL,'active','equity',0,?,?)`, name+"类", now, now)
	if err != nil {
		t.Fatalf("插入一级科目失败: %v", err)
	}
	l1, _ := res1.LastInsertId()
	res2, err := db.Exec(
		`INSERT INTO category(org_id, name, level, parent_id, status, kind, sort_order, created_at, updated_at)
		 VALUES(1,?,2,?,'active','equity',0,?,?)`, name, l1, now, now)
	if err != nil {
		t.Fatalf("插入二级科目失败: %v", err)
	}
	id, _ := res2.LastInsertId()
	return id
}

// createPartyAPI 通过 API 建往来单位，返回 id。
func createPartyAPI(t *testing.T, r *gin.Engine, name string) int64 {
	t.Helper()
	w := doJSON(t, r, "POST", "/api/parties", map[string]any{"name": name})
	if w.Code != http.StatusOK {
		t.Fatalf("建往来单位失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析往来单位响应失败: %v", err)
	}
	return out.Data.ID
}

// createReceivableAPI 通过 API 登记应收单，返回 id。
func createReceivableAPI(t *testing.T, r *gin.Engine, partyID int64, kind, title string, amountCents int64, incomeCatID *int64) int64 {
	t.Helper()
	body := map[string]any{
		"partyId": partyID, "recvKind": kind, "title": title, "amountCents": amountCents,
	}
	if incomeCatID != nil {
		body["incomeCategoryId"] = *incomeCatID
	}
	w := doJSON(t, r, "POST", "/api/receivables", body)
	if w.Code != http.StatusOK {
		t.Fatalf("登记应收单失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析应收单响应失败: %v", err)
	}
	return out.Data.ID
}

// createReceiptAPI 通过 API 核销，返回 receipt id；期望非 200 时返回 -1。
func createReceiptAPI(t *testing.T, r *gin.Engine, recID int64, amount int64, date, method string, extra map[string]any) (int64, int) {
	t.Helper()
	body := map[string]any{"amountCents": amount, "receiptDate": date, "method": method}
	for k, v := range extra {
		body[k] = v
	}
	w := doJSON(t, r, "POST", "/api/receivables/"+itoa(recID)+"/receipts", body)
	if w.Code != http.StatusOK {
		return -1, w.Code
	}
	var out struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析核销响应失败: %v", err)
	}
	return out.Data.ID, w.Code
}

func receivableStatus(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var s string
	if err := db.QueryRow(`SELECT status FROM receivable WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatalf("查应收单状态失败: %v", err)
	}
	return s
}

func countTxns(t *testing.T, db *sql.DB, direction, status string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM txn WHERE org_id=1 AND direction=? AND status=?`, direction, status).Scan(&n); err != nil {
		t.Fatalf("统计流水失败: %v", err)
	}
	return n
}

// ---------- T1：D11 现金收款验收路径（登记欠款 → 分次收 → 结清 → 银行自动入账） ----------

func TestCashReceiptFlow(t *testing.T) {
	db, r := newEnv(t)
	incCat := seedIncomeCat(t, db, "流转费收入")
	party := createPartyAPI(t, r, "张三")

	// 登记欠 ¥500（50000 分）
	rec := createReceivableAPI(t, r, party, "rent", "2026年度土地流转费", 50000, &incCat)
	if got := receivableStatus(t, db, rec); got != "open" {
		t.Fatalf("初始应收单应 open，实际 %s", got)
	}

	// 现金收 ¥300 → 欠 ¥200，银行自动生成一笔 income 流水
	receiptID, code := createReceiptAPI(t, r, rec, 30000, "2026-09-10", "cash", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("收款 300 失败: %d", code)
	}
	if got := receivableStatus(t, db, rec); got != "open" {
		t.Errorf("收 300 后应仍 open，实际 %s", got)
	}
	if n := countTxns(t, db, "income", "normal"); n != 1 {
		t.Errorf("现金收款应生成 1 笔银行收入流水，实际 %d", n)
	}
	var txnAmount int64
	if err := db.QueryRow(`SELECT amount_cents FROM txn WHERE org_id=1 AND direction='income' AND status='normal'`).Scan(&txnAmount); err != nil {
		t.Fatalf("查收入流水失败: %v", err)
	}
	if txnAmount != 30000 {
		t.Errorf("收入流水金额应 30000，实际 %d", txnAmount)
	}

	// 再收 ¥200 → 结清 closed
	_, code = createReceiptAPI(t, r, rec, 20000, "2026-09-12", "cash", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("收款 200 失败: %d", code)
	}
	if got := receivableStatus(t, db, rec); got != "closed" {
		t.Errorf("收满后应收单应 closed，实际 %s", got)
	}
	if n := countTxns(t, db, "income", "normal"); n != 2 {
		t.Errorf("两笔现金收款应生成 2 笔收入流水，实际 %d", n)
	}

	// 结清后再收 → 拒绝
	_, code = createReceiptAPI(t, r, rec, 1, "2026-09-13", "cash", map[string]any{})
	if code != http.StatusConflict {
		t.Errorf("结清后再核销应 409，实际 %d", code)
	}

	// 往来对象欠款合计 = 0
	var parties []Party
	w := doJSON(t, r, "GET", "/api/parties", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &struct{ Data *[]Party }{&parties}); err != nil {
		t.Fatalf("解析往来对象失败: %v", err)
	}
	_ = receiptID
	if len(parties) != 1 || parties[0].OutstandingCents != 0 {
		t.Errorf("结清后对象欠款合计应 0，实际 %+v", parties)
	}
}

// ---------- T2：抵销（应收欠款抵分红支出，不产生现金流水） ----------

func TestOffsetReceiptFlow(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	expenseCat := seedIncomeCat(t, db, "收益分红发放")
	party := createPartyAPI(t, r, "李四")

	// 先发分红 ¥800（支出流水，真实已付）
	txnID := seedTxn(t, db, "2026-09-01", "expense", 80000, expenseCat)

	// 李四欠流转费 ¥800
	rec := createReceivableAPI(t, r, party, "rent", "2026年度土地流转费", 80000, &incomeCat)

	// 抵销：用分红支出抵应收欠款
	_, code := createReceiptAPI(t, r, rec, 80000, "2026-09-05", "offset", map[string]any{"txnId": txnID})
	if code != http.StatusOK {
		t.Fatalf("抵销失败: %d", code)
	}
	if got := receivableStatus(t, db, rec); got != "closed" {
		t.Errorf("抵销后应收单应 closed，实际 %s", got)
	}
	// 不产生任何新现金流水；分红支出流水保留 normal
	if n := countTxns(t, db, "income", "normal"); n != 0 {
		t.Errorf("抵销不应产生现金收入流水，实际 %d", n)
	}
	if n := countTxns(t, db, "expense", "normal"); n != 1 {
		t.Errorf("分红支出流水应保留 1 笔 normal，实际 %d", n)
	}
	if n := countTxns(t, db, "expense", "voided"); n != 0 {
		t.Errorf("分红支出流水不应被作废，实际 %d", n)
	}
}

// ---------- T3：校验矩阵 ----------

func TestCreateReceiptValidations(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	otherCat := seedIncomeCat(t, db, "其他收入")
	party := createPartyAPI(t, r, "王五")

	// 无预设入账科目的应收单（收款时必须传 categoryId）
	recNoCat := createReceivableAPI(t, r, party, "other", "货款", 100000, nil)
	// 有预设入账科目的应收单
	rec := createReceivableAPI(t, r, party, "rent", "2026年流转费", 50000, &incomeCat)
	// 一笔收入流水（不可用于抵销）
	incomeTxn := seedTxn(t, db, "2026-09-01", "income", 100, incomeCat)

	cases := []struct {
		name   string
		recID  int64
		body   map[string]any
		status int
		want   string
	}{
		{"金额为负", rec, map[string]any{"amountCents": -1, "receiptDate": "2026-09-10", "method": "cash"}, http.StatusBadRequest, "INVALID_AMOUNT"},
		{"坏日期", rec, map[string]any{"amountCents": 100, "receiptDate": "2026/09/10", "method": "cash"}, http.StatusBadRequest, "INVALID_DATE"},
		{"超收", rec, map[string]any{"amountCents": 50001, "receiptDate": "2026-09-10", "method": "cash"}, http.StatusConflict, "RECEIPT_OVER_RECEIVABLE"},
		{"现金未指定入账科目", recNoCat, map[string]any{"amountCents": 100, "receiptDate": "2026-09-10", "method": "cash"}, http.StatusBadRequest, "INCOME_CATEGORY_REQUIRED"},
		{"现金入账科目非法", rec, map[string]any{"amountCents": 100, "receiptDate": "2026-09-10", "method": "cash", "categoryId": 99999}, http.StatusBadRequest, "INVALID_INCOME_CATEGORY"},
		{"抵销缺流水", rec, map[string]any{"amountCents": 100, "receiptDate": "2026-09-10", "method": "offset"}, http.StatusBadRequest, "OFFSET_TXN_REQUIRED"},
		{"抵销流水是收入", rec, map[string]any{"amountCents": 100, "receiptDate": "2026-09-10", "method": "offset", "txnId": incomeTxn}, http.StatusBadRequest, "OFFSET_TXN_INVALID"},
		{"抵销流水不存在", rec, map[string]any{"amountCents": 100, "receiptDate": "2026-09-10", "method": "offset", "txnId": 99999}, http.StatusBadRequest, "OFFSET_TXN_INVALID"},
		{"应收单不存在", 88888, map[string]any{"amountCents": 100, "receiptDate": "2026-09-10", "method": "cash", "categoryId": otherCat}, http.StatusNotFound, "RECEIVABLE_NOT_FOUND"},
	}

	for _, tc := range cases {
		w := doJSON(t, r, "POST", "/api/receivables/"+itoa(tc.recID)+"/receipts", tc.body)
		if w.Code != tc.status {
			t.Errorf("%s: 应 %d，实际 %d (%s)", tc.name, tc.status, w.Code, w.Body.String())
			continue
		}
		if got := apiErr(t, w); got != tc.want {
			t.Errorf("%s: 错误码应 %s，实际 %s", tc.name, tc.want, got)
		}
	}

	// 校验矩阵前预种了一笔 income 流水（供「抵销流水是收入」用例），拒绝后不应新增任何流水 → 仍恰 1 笔
	if n := countTxns(t, db, "income", "normal"); n != 1 {
		t.Errorf("校验拒绝后收入流水应仍 1 笔（预种），实际 %d", n)
	}
}

// ---------- T4：作废核销（cash 连带作废收入流水；offset 保留支出流水） ----------

func TestVoidReceipt(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")
	expenseCat := seedIncomeCat(t, db, "收益分红发放")
	party := createPartyAPI(t, r, "赵六")

	// 现金收部分后作废
	rec1 := createReceivableAPI(t, r, party, "rent", "2026年流转费", 50000, &incomeCat)
	rid1, code := createReceiptAPI(t, r, rec1, 30000, "2026-09-10", "cash", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("现金收款失败: %d", code)
	}

	// 作废该笔收款：收入流水同步作废、应收单仍 open
	w := doJSON(t, r, "PUT", "/api/receipts/"+itoa(rid1), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废收款失败: %d %s", w.Code, w.Body.String())
	}
	if n := countTxns(t, db, "income", "normal"); n != 0 {
		t.Errorf("作废后收入流水应 0 笔 normal，实际 %d", n)
	}
	if n := countTxns(t, db, "income", "voided"); n != 1 {
		t.Errorf("作废后收入流水应有 1 笔 voided，实际 %d", n)
	}
	if got := receivableStatus(t, db, rec1); got != "open" {
		t.Errorf("作废部分收款后应收单应 open，实际 %s", got)
	}

	// 抵销核销作废：分红支出流水保留，应收单退回 open
	rec2 := createReceivableAPI(t, r, party, "rent", "2027年流转费", 80000, &incomeCat)
	expenseTxn := seedTxn(t, db, "2026-09-02", "expense", 80000, expenseCat)
	rid2, code := createReceiptAPI(t, r, rec2, 80000, "2026-09-06", "offset", map[string]any{"txnId": expenseTxn})
	if code != http.StatusOK {
		t.Fatalf("抵销失败: %d", code)
	}
	if got := receivableStatus(t, db, rec2); got != "closed" {
		t.Fatalf("抵销后应收单应 closed，实际 %s", got)
	}
	w = doJSON(t, r, "PUT", "/api/receipts/"+itoa(rid2), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废抵销失败: %d %s", w.Code, w.Body.String())
	}
	if got := receivableStatus(t, db, rec2); got != "open" {
		t.Errorf("作废抵销后应收单应 open，实际 %s", got)
	}
	if n := countTxns(t, db, "expense", "normal"); n != 1 {
		t.Errorf("作废抵销不应动分红支出流水（normal=1），实际 %d", n)
	}

	// 作废不存在
	w = doJSON(t, r, "PUT", "/api/receipts/99999", map[string]any{"status": "voided"})
	if w.Code != http.StatusNotFound {
		t.Errorf("作废不存在应 404，实际 %d", w.Code)
	}
}

// ---------- T5：列表（对象欠款合计 / 应收单过滤 / 详情含核销记录 / 跨组织不可见） ----------

func TestListAndDetail(t *testing.T) {
	db, r := newEnv(t)
	incomeCat := seedIncomeCat(t, db, "流转费收入")

	partyA := createPartyAPI(t, r, "张三")
	partyB := createPartyAPI(t, r, "某公司")

	recA1 := createReceivableAPI(t, r, partyA, "rent", "2026年流转费", 50000, &incomeCat)
	createReceivableAPI(t, r, partyA, "other", "垫付款", 20000, nil)
	createReceivableAPI(t, r, partyB, "dividend", "2026年投资收益", 100000, &incomeCat)
	// 收张三 30000（现金），剩余 20000 + 垫付 20000 = 40000 欠款
	createReceiptAPI(t, r, recA1, 30000, "2026-09-10", "cash", map[string]any{})

	// 对象列表欠款合计：张三 40000，某公司 100000
	var out []Party
	w := doJSON(t, r, "GET", "/api/parties", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &struct{ Data *[]Party }{&out}); err != nil {
		t.Fatalf("解析往来单位失败: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("应有 2 个往来单位，实际 %d", len(out))
	}
	byName := map[string]int64{}
	for _, p := range out {
		byName[p.Name] = p.OutstandingCents
	}
	if byName["张三"] != 40000 || byName["某公司"] != 100000 {
		t.Errorf("欠款合计不对: %+v", byName)
	}

	// 关键字过滤：命中「公司」
	w = doJSON(t, r, "GET", "/api/parties?keyword="+urlQuery("公司"), nil)
	if err := json.Unmarshal(w.Body.Bytes(), &struct{ Data *[]Party }{&out}); err != nil {
		t.Fatalf("解析过滤单位失败: %v", err)
	}
	if len(out) != 1 || out[0].Name != "某公司" {
		t.Errorf("keyword=公司 应命中 1 个（某公司），实际 %d", len(out))
	}

	// 应收单按对象过滤：张三有 2 单
	var recList struct {
		Data struct {
			Items []Receivable `json:"items"`
			Total int          `json:"total"`
		} `json:"data"`
	}
	w = doJSON(t, r, "GET", "/api/receivables?partyId="+itoa(partyA), nil)
	if err := json.Unmarshal(w.Body.Bytes(), &recList); err != nil {
		t.Fatalf("解析应收单失败: %v", err)
	}
	if recList.Data.Total != 2 {
		t.Errorf("张三应有 2 张应收单，实际 %d", recList.Data.Total)
	}
	for _, it := range recList.Data.Items {
		if it.PartyName != "张三" {
			t.Errorf("应收单应带对象名 张三，实际 %s", it.PartyName)
		}
	}

	// 状态过滤：closed 应 0（都未结清），open 应 3
	w = doJSON(t, r, "GET", "/api/receivables?status=closed", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &recList); err != nil {
		t.Fatalf("解析 closed 列表失败: %v", err)
	}
	if recList.Data.Total != 0 {
		t.Errorf("closed 应收单应为 0，实际 %d", recList.Data.Total)
	}

	// 详情：recA1 已收 30000、未收 20000，含 1 条核销记录
	var detail struct {
		Data ReceivableDetail `json:"data"`
	}
	w = doJSON(t, r, "GET", "/api/receivables/"+itoa(recA1), nil)
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("解析详情失败: %v", err)
	}
	if detail.Data.Receivable.PaidCents != 30000 || detail.Data.Receivable.OutstandingCents != 20000 {
		t.Errorf("详情已收/未收不对: %+v", detail.Data.Receivable)
	}
	if len(detail.Data.Receipts) != 1 || detail.Data.Receipts[0].Method != "cash" {
		t.Errorf("详情核销记录应 1 条 cash，实际 %+v", detail.Data.Receipts)
	}

	// 跨组织：org=2 看不到任何对象与应收单
	repo := NewRepo(db)
	items2, err := repo.ListParties(2, "")
	if err != nil {
		t.Fatalf("组织2查询失败: %v", err)
	}
	if len(items2) != 0 {
		t.Errorf("组织 2 应看不到往来对象，实际 %d", len(items2))
	}
	itemsRec2, total2, err := repo.ListReceivables(2, nil, 0, "", "", 1, 50)
	if err != nil || total2 != 0 || len(itemsRec2) != 0 {
		t.Errorf("组织 2 应看不到应收单，实际 total=%d err=%v", total2, err)
	}
}

// createPartyWithType 通过 API 建指定类型往来单位。
func createPartyWithType(t *testing.T, r *gin.Engine, name, typ string) int64 {
	t.Helper()
	w := doJSON(t, r, "POST", "/api/parties", map[string]any{"name": name, "type": typ})
	if w.Code != http.StatusOK {
		t.Fatalf("创建单位失败 code=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Data Party `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析创建结果失败: %v", err)
	}
	return out.Data.ID
}

func decodeAccrueResult(t *testing.T, w *httptest.ResponseRecorder) BatchAccrueResult {
	t.Helper()
	var out struct {
		Data BatchAccrueResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析结转结果失败: %v", err)
	}
	return out.Data
}

// TestPartyTypeTypedAccrueVoid 验证：单位类型建档 + 按类型结转（rent→流转企业 / dividend→投资公司）+ 未收款应收作废重结。
func TestPartyTypeTypedAccrueVoid(t *testing.T) {
	db, r := newEnv(t)
	_ = db
	flow := createPartyWithType(t, r, "甲公司", "flow")
	invest := createPartyWithType(t, r, "乙公司", "invest")

	// 列表返回类型；投资金额初始为 0（同名长期投资公司未投出）
	wList := doJSON(t, r, "GET", "/api/parties", nil)
	var list struct {
		Data []Party `json:"data"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &list); err != nil {
		t.Fatalf("解析单位列表失败: %v", err)
	}
	byName := map[string]Party{}
	for _, p := range list.Data {
		byName[p.Name] = p
	}
	if byName["甲公司"].Type != "flow" || byName["乙公司"].Type != "invest" {
		t.Errorf("单位类型错误：甲=%s 乙=%s", byName["甲公司"].Type, byName["乙公司"].Type)
	}

	// 标准：流转企业 5 万流转费；投资公司 8 万分红 + 一条不匹配的流转费标准（结转应忽略）
	saveStd := func(party int64, kind string, amount int64) {
		w := doJSON(t, r, "POST", "/api/recv-standards", map[string]any{"partyId": party, "recvKind": kind, "amountCents": amount})
		if w.Code != http.StatusOK {
			t.Fatalf("保存标准失败 code=%d body=%s", w.Code, w.Body.String())
		}
	}
	saveStd(flow, "rent", 50000)
	saveStd(invest, "dividend", 80000)
	saveStd(invest, "rent", 99999)

	// 一键结转 rent：只应给甲公司（乙公司的 rent 标准因类型不符被忽略）
	wr := decodeAccrueResult(t, doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{"year": 2026, "kind": "rent"}))
	if wr.Created != 1 || wr.Skipped != 0 {
		t.Errorf("rent 结转应新增 1 条，实际 created=%d skipped=%d", wr.Created, wr.Skipped)
	}
	wd := decodeAccrueResult(t, doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{"year": 2026, "kind": "dividend"}))
	if wd.Created != 1 || wd.Skipped != 0 {
		t.Errorf("dividend 结转应新增 1 条，实际 created=%d skipped=%d", wd.Created, wd.Skipped)
	}

	// 找到甲公司 2026 rent 应收单
	wRec := doJSON(t, r, "GET", "/api/receivables?partyId="+itoa(flow)+"&kind=rent", nil)
	var recList struct {
		Data ReceivableListResponse `json:"data"`
	}
	if err := json.Unmarshal(wRec.Body.Bytes(), &recList); err != nil {
		t.Fatalf("解析应收单失败: %v", err)
	}
	if len(recList.Data.Items) != 1 {
		t.Fatalf("甲公司应有 1 张 rent 应收单，实际 %d", len(recList.Data.Items))
	}
	recID := recList.Data.Items[0].ID

	// 作废后可重结：作废 → 2026 rent 应能再次新增
	if code := apiErr(t, doJSON(t, r, "PUT", "/api/receivables/"+itoa(recID)+"/void", nil)); code != "" {
		t.Fatalf("作废失败 code=%s", code)
	}
	wr2 := decodeAccrueResult(t, doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{"year": 2026, "kind": "rent"}))
	if wr2.Created != 1 {
		t.Errorf("作废后重新结转应新增 1 条，实际 created=%d", wr2.Created)
	}
}

func decodePreview(t *testing.T, w *httptest.ResponseRecorder) PreviewAccrueResult {
	t.Helper()
	var out struct {
		Data PreviewAccrueResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析预览失败: %v body=%s", err, w.Body.String())
	}
	return out.Data
}

// TestPreviewAccrue 验证：预览带出类型匹配标准并标注已存在；与 accrue 一致。
func TestPreviewAccrue(t *testing.T) {
	_, r := newEnv(t)
	flow := createPartyWithType(t, r, "甲公司", "flow")
	invest := createPartyWithType(t, r, "乙公司", "invest")
	saveStd := func(party int64, kind string, amount int64) {
		w := doJSON(t, r, "POST", "/api/recv-standards", map[string]any{"partyId": party, "recvKind": kind, "amountCents": amount})
		if w.Code != http.StatusOK {
			t.Fatalf("保存标准失败 body=%s", w.Body.String())
		}
	}
	saveStd(flow, "rent", 50000)
	saveStd(invest, "dividend", 80000)
	saveStd(invest, "rent", 99999) // 类型不符，预览应忽略

	pv := decodePreview(t, doJSON(t, r, "GET", "/api/recv-standards/preview?year=2026", nil))
	if len(pv.Items) != 2 {
		t.Fatalf("预览应有 2 行，实际 %d", len(pv.Items))
	}
	for _, it := range pv.Items {
		if it.Exists {
			t.Errorf("结转前不应有已存在行: %+v", it)
		}
	}

	// 确认结转后再预览：应全部标注已存在
	doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{"year": 2026, "kind": "rent"})
	doJSON(t, r, "POST", "/api/recv-standards/accrue", map[string]any{"year": 2026, "kind": "dividend"})
	pv2 := decodePreview(t, doJSON(t, r, "GET", "/api/recv-standards/preview?year=2026", nil))
	if len(pv2.Items) != 2 {
		t.Fatalf("结转后预览应有 2 行，实际 %d", len(pv2.Items))
	}
	for _, it := range pv2.Items {
		if !it.Exists {
			t.Errorf("结转后应标注已存在: %+v", it)
		}
	}
}
