---
currentMenu: account
---

## User roles

There are 3 different user roles:
- Admin (for user and file management)
- User (regular, logged in user)
- Guest (anonymous, not logged in)


## User permissions

There are 6 different user permissions admin can assign to each user:

- Read (user can browse and list files and folders)
- Write (user can copy, move, rename, and delete files)
- Upload (user can upload files to the repository)
- Download (user can download files from the repository)
- Batch Download (user can download multiple files and folders at once)
- Zip (user can zip and unzip files)


Some permissions require others. For example, Batch Download requires Read permissions (so that user can list files and select them) as well as basic Download permissions.

## Guest account

Guest account is a predefined account and it is disabled by default since no permissions are assigned.

Admin can enable Guest account which will allow everyone to interact with the repository based on the Guest account permissions.

## Resetting Admin's password

If you forgot your admin password, follow these steps to reset it:

- Backup your current users file `private/users.json` to a safe place
- Copy blank template `private/users.json.blank` over `private/users.json` or simply refresh your browser
- Login as admin with default credentials `admin/admin123`
- Put your original users file back to `private/users.json` replacing the template
- Since you are now logged in as admin, simply go to users page and change your password
- Log out and try to login with the new password

Note: If you're using database Auth adapter then simply run the SQL query below to set default password back to `admin123`


```
UPDATE `users`
SET `password` = '$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq'
WHERE `username` = 'admin';
```


