-- V22：订单记桌台名（堂食小票要打印桌号，否则对不上单）
ALTER TABLE t_pos_order ADD COLUMN table_name TEXT NOT NULL DEFAULT '';
