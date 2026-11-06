-- V30：退款记录班次与"原单营业日"，用于识别跨班次/跨天退款
ALTER TABLE t_pos_refund ADD COLUMN shift_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE t_pos_refund ADD COLUMN order_biz_date TEXT NOT NULL DEFAULT '';
