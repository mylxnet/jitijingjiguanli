package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestStaticAsset404AndCacheHeaders 换镜像后旧页面引用的产物必须 404（不能回退 index.html，
// 否则浏览器把 HTML 当 JS 模块加载、MIME 校验失败 → 懒加载路由静默打不开），且 index.html 不得被缓存。
func TestStaticAsset404AndCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerStatic(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	get := func(path string) (*http.Response, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}

	resp, body := get("/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/ 应 200，实际 %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("/ 的 Cache-Control 应为 no-cache，实际 %q", got)
	}
	entry := regexp.MustCompile(`assets/index-[A-Za-z0-9_-]+\.js`).FindString(body)
	if entry == "" {
		t.Fatal("index.html 里没找到入口 chunk，测试无法继续")
	}

	resp, _ = get("/" + entry)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s 应 200，实际 %d", entry, resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("带哈希产物的 Cache-Control 应含 immutable，实际 %q", got)
	}

	// 旧镜像留下的产物名：必须 404，且绝不能是 index.html
	resp, body = get("/assets/index-OLDCACHE000.js")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("不存在的产物应 404，实际 %d（回退 index.html 的缺陷未修）", resp.StatusCode)
	}
	if strings.Contains(body, "<!doctype html>") {
		t.Error("不存在的产物仍返回 HTML 内容")
	}
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
		t.Errorf("不存在的产物 Content-Type 不应为 text/html，实际 %q", ct)
	}

	// SPA 回退仍要生效：非 assets 的未知路径回到 index.html
	resp, body = get("/some/spa/route")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "<!doctype html>") {
		t.Errorf("未知路径应回退 index.html，实际 %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("回退的 index.html Cache-Control 应为 no-cache，实际 %q", got)
	}
}
