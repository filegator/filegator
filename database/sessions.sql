CREATE TABLE `sessions` (
      `sess_id` varbinary(128) NOT NULL,
      `sess_data` blob NOT NULL,
      `sess_lifetime` mediumint(9) NOT NULL,
      `sess_time` int(10) unsigned NOT NULL,
      PRIMARY KEY (`sess_id`)
) CHARSET=utf8 COLLATE=utf8_bin;

