-- V14：抹零（商家让利，单独记账，保证"销售额=实收"可核对）
ALTER TABLE t_pos_order ADD COLUMN round_off INTEGER NOT NULL DEFAULT 0;
