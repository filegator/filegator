# FileGator UAT checklist — staging

**Tester:** ______________________ **Date:** _______________ **Role:** Admin / User (circle one)

---

## What's new in this update

You're testing three changes that will go live on the production URL once we sign off:

1. **Multi-factor authentication (MFA)** — admins are now required to enroll an authenticator app on first login. Other users can opt in.
2. **Self-service password reset** — a "Forgot password?" link on the login screen emails you a reset link.
3. **Smoother session handling** after MFA/email/password changes — you should no longer get logged out unexpectedly mid-task.

**Heads-up:** when you log in for the first time on staging, your old session won't carry over — that's expected. If you're an admin, you'll be asked to set up an authenticator app immediately.

---

## Tasks (please tick as you go)

### Login & MFA

- [ ] Log in with your existing username and password
- [ ] **(Admins only)** Complete MFA setup using Google Authenticator, Microsoft Authenticator, 1Password, or similar
  - [ ] Scan the QR code or type the secret manually
  - [ ] Enter the 6-digit code; reach the "backup codes" screen
  - [ ] **Save the backup codes somewhere safe** — these get you in if you lose your phone
  - [ ] Refresh the page on the backup-codes screen → codes are still visible (this used to be a bug)
- [ ] Log out and log back in — verify it prompts for your authenticator code
- [ ] **(Admins only)** Log in once using a backup code instead of the authenticator
- [ ] Try logging in again with the same backup code → should be rejected (codes are single-use)

### File operations (use a real folder you actually work in)

- [ ] Browse to your normal working folder — file list matches what you expect
- [ ] Upload a small file
- [ ] Download a file you uploaded
- [ ] Rename a file
- [ ] Move a file between folders
- [ ] Delete a file (and restore from trash, if your role allows)
- [ ] Upload a large file (>50 MB) if relevant to your workflow
- [ ] Open a deep link (a saved bookmark to a specific folder) while logged out → after logging in, you should land back at that folder, not the root

### Password reset round-trip

- [ ] On the login page, click **Forgot password?**
- [ ] Enter your email; the page should say "if an account exists, an email is on the way"
- [ ] Check your inbox (subject starts with **[STAGING]**) — usually arrives within a minute
- [ ] Email looks correctly branded (logo, color, support address)
- [ ] Click the reset link → land on a "set new password" page
- [ ] Enter a new password → land on a confirmation, then log in with it

### Admin-only

- [ ] Create a new user, set their role and homedir
- [ ] Regenerate your MFA backup codes from your profile
  - [ ] Old backup codes stop working
  - [ ] New codes are shown immediately and survive a page refresh
- [ ] Disable MFA on a test account (requires the password + a current authenticator code)

---

## Anything weird?

Note anything that surprised you — slow, confusing wording, ugly layout, missing buttons, errors, anything:

```
________________________________________________________________
________________________________________________________________
________________________________________________________________
________________________________________________________________
```

---

## Sign-off

> I confirm staging behaves correctly for my role and I'm OK with these changes going to production.

Signed: ______________________  Date: _______________
