-- V19：退款路由（原路退回）：记录退款走哪种方式、退给哪个会员
ALTER TABLE t_pos_refund ADD COLUMN method TEXT NOT NULL DEFAULT 'cash';
ALTER TABLE t_pos_refund ADD COLUMN member_id INTEGER NOT NULL DEFAULT 0;
