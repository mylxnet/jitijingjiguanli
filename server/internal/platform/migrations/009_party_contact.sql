-- 009_party_contact.sql — v0.6：往来单位补充联系方式与流转面积
-- contact_phone：联系电话（文本，可为空）；area_mu：流转面积（亩，流转企业填写，默认 0）
ALTER TABLE party ADD COLUMN contact_phone TEXT NOT NULL DEFAULT '';
ALTER TABLE party ADD COLUMN area_mu REAL NOT NULL DEFAULT 0;
