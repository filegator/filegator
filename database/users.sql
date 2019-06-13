CREATE TABLE `users` (
    `id` int(10) NOT NULL AUTO_INCREMENT,
    `username` varchar(255) NOT NULL,
    `name` varchar(255) NOT NULL,
    `role` varchar(20) NOT NULL,
    `permissions` varchar(200) NOT NULL,
    `homedir` varchar(2000) NOT NULL,
    `password` varchar(255) NOT NULL,
    PRIMARY KEY (`id`),
    KEY `username` (`username`)
) CHARSET=utf8 COLLATE=utf8_bin;

INSERT INTO `users` (`username`, `name`, `role`, `permissions`, `homedir`, `password`)
VALUES
('guest', 'Guest', 'guest', '', '/', ''),
('admin', 'Admin', 'admin', 'read|write|upload|download|batchdownload|zip', '/', '$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq');
