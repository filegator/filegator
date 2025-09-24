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
⏺ Here are your URL-encoded doc parameter test cases:


✅ SUCCESS CASES

?doc=%2F → /
?doc=%2Fprojects → /projects
?doc=%2Fprojects%2Fweb-app%2Fsrc%2Fcomponents → /projects/web-app/src/components
?doc=%2Ftemp%20files → /temp files (spaces)
?doc=%2Fspecial%20chars%40%23%24 → /special chars@#$ (special chars)

⚠️ FALLBACK CASES (should go to nearest parent)

?doc=%2Fprojects%2Fnonexistent → falls back to /projects
?doc=%2Fdocuments%2Freports%2F2024%2Fmissing → falls back to
/documents/reports/2024
?doc=%2Fprojects%2FREADME.md → falls back to /projects (file, not dir)

❌ FAILURE CASES (should go to root)

?doc=%2Fcompletely%2Fwrong%2Fpath → falls back to /
?doc=%2F..%2F..%2Fetc%2Fpasswd → falls back to / (security)

🔥 EDGE CASES

?doc=invalid%encoding → ignore, log error
?doc=%2 → ignore, log error
?doc= → ignore empty

Test: http://localhost:8080/?doc=[encoded_value]
Check: Console logs + file manager navigation + URL cleanup