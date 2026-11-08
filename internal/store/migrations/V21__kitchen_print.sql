-- V21：后厨联打（每个打印任务可指定打印机；后厨单与前台小票分开）
ALTER TABLE t_pos_print_job ADD COLUMN printer TEXT NOT NULL DEFAULT '';
