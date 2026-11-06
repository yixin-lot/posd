-- V23：会员累计消费（等级依据）+ 积分过期时间
ALTER TABLE t_pos_member ADD COLUMN total_spent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE t_pos_member ADD COLUMN points_expire_at TEXT;
