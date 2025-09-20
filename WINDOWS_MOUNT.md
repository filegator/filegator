# Windows Server Filesystem Mounting

In order to make uploaded data available to 4D instances running on the Windows server,
we need to mount a specific directory from Windows locally.


### Route SMB Port To Localhost
```bash
ssh -L 44445:localhost:445 stefan@85.215.140.246
```

### Mount the Directory
#### On Mac
```bash
sudo mount -t smbfs //stefan@127.0.0.1:44445/Archive ./test_mount
```

#### On Linux
```bash
sudo mount -t cifs //127.0.0.1/Archive ./test_mount -o port=44445,username=stefan
```
