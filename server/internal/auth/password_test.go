package auth

import "testing"

// TestChangePassword 修改密码：校验原口令、成功后旧口令失效新口令可登录。
func TestChangePassword(t *testing.T) {
	svc, _ := newTestEnv(t)

	if _, _, err := svc.RegisterOrg("甲村", "admin", "old-pass"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	u, err := svc.repo.FindByUsername("admin")
	if err != nil {
		t.Fatalf("查找用户失败: %v", err)
	}
	if u == nil {
		t.Fatal("用户不存在")
	}

	// 原密码错误 → ErrOldPasswordWrong
	if err := svc.ChangePassword(u.ID, "wrong", "new-pass"); err != ErrOldPasswordWrong {
		t.Errorf("原密码错误应报 ErrOldPasswordWrong，实际 %v", err)
	}
	// 新密码过短
	if err := svc.ChangePassword(u.ID, "old-pass", "123"); err != ErrInvalidPassword {
		t.Errorf("短密码应报 ErrInvalidPassword，实际 %v", err)
	}

	// 成功修改
	if err := svc.ChangePassword(u.ID, "old-pass", "new-pass"); err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}

	// 旧口令登录失败
	if _, _, err := svc.Login("admin", "old-pass"); err != ErrInvalidCredentials {
		t.Errorf("旧口令应无法登录，实际 %v", err)
	}
	// 新口令登录成功
	if _, _, err := svc.Login("admin", "new-pass"); err != nil {
		t.Errorf("新口令应可登录: %v", err)
	}
}
