-- V16：登录失败锁定落库（重启不再清零）
ALTER TABLE t_pos_staff ADD COLUMN fail_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE t_pos_staff ADD COLUMN locked_until INTEGER NOT NULL DEFAULT 0; -- epoch 毫秒
