-- V24：订单记录会员等级折扣与会员
ALTER TABLE t_pos_order ADD COLUMN member_discount INTEGER NOT NULL DEFAULT 0;
ALTER TABLE t_pos_order ADD COLUMN member_id INTEGER NOT NULL DEFAULT 0;
