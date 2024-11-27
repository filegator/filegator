---
currentMenu: default
---

## Translations

Language is configured by adjusting the `language` variable in your `configuration.php` file.

Available languages:

- ```english``` (default)
- ```spanish```
- ```german```
- ```indonesian```
- ```turkish```
- ```lithuanian```
- ```portuguese```
- ```dutch```
- ```chinese``` (simplified)
- ```bulgarian```
- ```serbian```
- ```french```
- ```slovak```
- ```polish```
- ```italian```
- ```korean```
- ```czech```
- ```galician```
- ```russian```
- ```hungarian```
- ```swedish```
- ```japanese```
- ```slovenian```
- ```hebrew```
- ```romanian```
- ```arabic``` (see https://docs.filegator.io/translations/default.html#rtl-support)
- ```portuguese_br``` (brazilian portuguese pt-BR)
- ```persian```
- ```estonian```
- ```ukrainian```

Please help us translating FileGator to your language by submitting a Pull Request on GitHub.


## How to translate

First, you must setup the project like described in the 'Development' section. Default language file is located under `frontend/translations/english.js` You can add more languages in the same folder. Once your language file is in place, it needs to be added to `frontend/mixins/shared.js`. After this, recompile everything with `npm run build` and then you can use it by adjusting the `language` variable in your `configuration.php` file.

You should only translate value on the right, for example:

```
'Close': 'Schliessen',
```

Default language file:

```
const data = {
  'Selected': 'Selected: {0} of {1}',
  'Uploading files': 'Uploading {0}% of {1}',
  'File size error': '{0} is too large, please upload files less than {1}',
  'Upload failed': '{0} failed to upload',
  'Per page': '{0} Per Page',
  'Folder': 'Folder',
  'Login failed, please try again': 'Login failed, please try again',
  'Already logged in': 'Already logged in.',
  'Please enter username and password': 'Please enter username and password.',
  'Not Found': 'Not Found',
  'Not Allowed': 'Not Allowed',
  'Please log in': 'Please log in',
  'Unknown error': 'Unknown error',
  'Add files': 'Add files',
  'New': 'New',
  'New name': 'New name',
  'Username': 'Username',
  'Password': 'Password',
  'Login': 'Log in',
  'Logout': 'Log out',
  'Profile': 'Profile',
  'No pagination': 'No pagination',
  'Time': 'Time',
  'Name': 'Name',
  'Size': 'Size',
  'Home': 'Home',
  'Copy': 'Copy',
  'Move': 'Move',
  'Rename': 'Rename',
  'Required': 'Please fill out this field',
  'Zip': 'Zip',
  'Batch Download': 'Batch Download',
  'Unzip': 'Unzip',
  'Delete': 'Delete',
  'Download': 'Download',
  'Copy link': 'Copy link',
  'Done': 'Done',
  'File': 'File',
  'Drop files to upload': 'Drop files to upload',
  'Close': 'Close',
  'Select Folder': 'Select Folder',
  'Users': 'Users',
  'Files': 'Files',
  'Role': 'Role',
  'Cancel': 'Cancel',
  'Paused': 'Paused',
  'Confirm': 'Confirm',
  'Create': 'Create',
  'User': 'User',
  'Admin': 'Admin',
  'Save': 'Save',
  'Read': 'Read',
  'Write': 'Write',
  'Upload': 'Upload',
  'Permissions': 'Permissions',
  'Homedir': 'Home Folder',
  'Leave blank for no change': 'Leave blank for no change',
  'Are you sure you want to do this?': 'Are you sure you want to do this?',
  'Are you sure you want to allow access to everyone?': 'Are you sure you want to allow access to everyone?',
  'Are you sure you want to stop all uploads?': 'Are you sure you want to stop all uploads?',
  'Something went wrong': 'Something went wrong',
  'Invalid directory': 'Invalid directory',
  'This field is required': 'This field is required',
  'Username already taken': 'Username already taken',
  'User not found': 'User not found',
  'Old password': 'Old password',
  'New password': 'New password',
  'Wrong password': 'Wrong password',
  'Updated': 'Updated',
  'Deleted': 'Deleted',
  'Your file is ready': 'Your file is ready',
  'View': 'View',
}

export default data

```

## RTL support

Thanks to @yaniv1983 who provided these RTL tweaks for hebrew language in [#301](https://github.com/filegator/filegator/issues/301).

To enable RTL support, simply add this to your configuration.php 'add_to_head' section:

```
<style>
body {
    direction: rtl;
    color: #000000;
}
.search-btn[data-v-081c0a81] {
    margin-left: 10px;
    margin-right: unset;
}
#multi-actions a[data-v-081c0a81] {
    margin: 0 0 15px 15px;
}
.dropdown .dropdown-menu .has-link a, a.dropdown-item, button.dropdown-item {
    padding-left: 3rem;
    padding-right: unset;
    text-align: right;
}
table td:not([align]), table th:not([align]) {
    text-align: right;
}
.b-table .table th .th-wrap .icon {
    margin-left: 0;
    margin-right: .5rem;
    font-size: 1rem;
}
.b-table .table th .th-wrap.is-numeric {
    flex-direction: unset;
    text-align: right;
}
.progress-icon[data-v-07f55d0a] {
    margin-right: 15px;
    margin-left: unset;
}
.progress-items[data-v-07f55d0a] {
    overflow-y: hidden;
    margin-left: -100px;
    padding-left: 100px;
    max-height: 300px;
    margin-right: unset;
    padding-right: unset;
}
.navbar-burger {
    margin-left: unset;
    margin-right: auto;
}
@media screen and (min-width: 1024px) {
.navbar-end {
    justify-content: flex-end;
    margin-right: auto;
    margin-left: unset;
}}
@media (min-width: 1088px) {
.logo img[data-v-cd57c856] {
    max-height: 3.5rem !important;
}}
</style>
```

