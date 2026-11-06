-- V26：会员停用 + 订单记录发放积分（小票要打印）
ALTER TABLE t_pos_member ADD COLUMN active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE t_pos_order ADD COLUMN points_earned INTEGER NOT NULL DEFAULT 0;
