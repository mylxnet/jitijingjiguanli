-- 008_party_type.sql — v0.6：往来单位类型化，为按类型设标准、一键结转与报表导出做准备。
-- type：flow 流转企业 / invest 投资公司 / other 其它单位；存量单位默认 flow（历史数据按流转企业）。
ALTER TABLE party ADD COLUMN type TEXT NOT NULL DEFAULT 'flow';
