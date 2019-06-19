## User roles

There are 3 different user roles:
- Admin (for user management)
- User (regular, logged in user)
- Guest (anonymous, not logged in)


## User permissions

There are 6 different user permissions you can assign to each user:

- Read (user can browse and list files and folders)
- Write (user can copy, move, rename, and delete files)
- Upload (user can upload files to the repository)
- Download (user can download files from the repository)
- Bach Download (user can download multiple files and folders at once)
- Zip (user can zip and unzip files)


Some permissions require others. For example, Batch Download requires Read permissions (so than user can list files and select them) as well as basic Download permissions.

## Guest account

Guest account is predefined account and it is disabled by default since no permissions is assigned.

Admin can enable Guest account which will allow everyone to interact with the repository based on the Guest account permissions.

