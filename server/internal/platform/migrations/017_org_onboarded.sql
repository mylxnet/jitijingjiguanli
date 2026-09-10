-- 017_org_onboarded.sql — 引导完成标记落库，作为「是否已建账」的权威来源。
-- 背景：引导跳转此前完全依赖前端 localStorage 标记 + /api/parties 探测兜底。
--       标记按「浏览器 + 站点来源」存储，清缓存/换浏览器/换访问地址即丢失；
--       兜底探测一旦请求失败或返回异常，会被误判为"未建账"而跳到引导页，
--       表现为"提交数据时偶尔跳转引导页"。
-- 方案：org 表增加 onboarded 标记，/api/me 返回该值，前端守卫以此为准。
ALTER TABLE org ADD COLUMN onboarded INTEGER NOT NULL DEFAULT 0;

-- 回填：已有业务数据的既有组织视为已建账，避免老组织被拉进引导页。
UPDATE org SET onboarded = 1
WHERE EXISTS (SELECT 1 FROM party       WHERE party.org_id = org.id)
   OR EXISTS (SELECT 1 FROM txn         WHERE txn.org_id = org.id)
   OR EXISTS (SELECT 1 FROM receivable  WHERE receivable.org_id = org.id)
   OR EXISTS (SELECT 1 FROM contract    WHERE contract.org_id = org.id)
   OR EXISTS (SELECT 1 FROM fund_move   WHERE fund_move.org_id = org.id)
   OR EXISTS (SELECT 1 FROM app_setting WHERE app_setting.org_id = org.id
                                             AND app_setting.key = 'bank_opening_balance_cents'
                                             AND CAST(app_setting.value AS INTEGER) <> 0)
   OR EXISTS (SELECT 1 FROM category    WHERE category.org_id = org.id
                                             AND category.opening_balance_cents <> 0);
