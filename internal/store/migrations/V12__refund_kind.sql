-- V12：退款流水区分「退款」与「作废」
-- 作废也会写一条退款流水（钱要回钱箱账），但报表的"退款"不能把它算进去，
-- 否则作废单既不计销售、又扣一次退款，等于重复扣减。
ALTER TABLE t_pos_refund ADD COLUMN kind TEXT NOT NULL DEFAULT 'refund';
