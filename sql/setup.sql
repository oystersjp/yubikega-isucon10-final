CREATE DATABASE IF NOT EXISTS `xsuportal` DEFAULT CHARACTER SET utf8mb4;
CREATE USER IF NOT EXISTS `isucon`@`localhost` IDENTIFIED WITH mysql_native_password BY 'isucon';
GRANT ALL ON `xsuportal`.* TO `isucon`@`localhost`;

ALTER TABLE `benchmark_jobs` ADD INDEX latest_score_job_ids_where (`team_id`,`finished_at`);
ALTER TABLE `benchmark_jobs` ADD INDEX latest_score_job_ids_group_by (`team_id`);

ALTER TABLE `contestants` ADD INDEX team_student_flags_group_by (`team_id`);