-- V13：挂单（点单未付款，先挂起）
ALTER TABLE t_pos_order ADD COLUMN held_at TEXT;
