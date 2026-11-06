-- V2：打印任务区分「已提交给执行器」与「执行器已出纸」（ADR-001 §4.4）
ALTER TABLE t_pos_print_job ADD COLUMN agent_job_id TEXT NOT NULL DEFAULT '';
