-- V28：积分到期提醒的"已提醒"标记（避免同一顾客每单都打）
ALTER TABLE t_pos_member ADD COLUMN points_notified_at TEXT;
