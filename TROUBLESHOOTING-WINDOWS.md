# WhatsApp Automation - Windows Troubleshooting Guide

This guide helps resolve common issues when running WhatsApp Automation on Windows.

## Quick Fixes

### Run the Check Script First
Before troubleshooting, run the validation script:
```
check.bat
```

This will identify most common issues automatically.

---

## Common Errors and Solutions

### 1. "Failed to Create Data Directory" - Chrome Error

**Error Message:**
```
Google Chrome cannot read and write to its data directory:
./chrome-data
```

**Cause:** Chrome cannot access the chrome-data folder

**Solutions:**

#### Solution A: Run as Administrator
1. Right-click `run.bat` or `run-dryrun.bat`
2. Select "Run as administrator"

#### Solution B: Check Folder Permissions
1. Locate the `chrome-data` folder in your project directory
2. Right-click → Properties → Security tab
3. Ensure your user account has "Full control"
4. If not, click Edit and grant permissions

#### Solution C: Delete and Recreate
1. Close all Chrome windows
2. Delete the `chrome-data` folder
3. Run `setup-windows.bat` again
4. The folder will be recreated with correct permissions

#### Solution D: Change Location
1. Open `config.yaml`
2. Change `user_data_dir: "./chrome-data"` to `user_data_dir: "C:\\Temp\\chrome-data"`
3. Make sure `C:\Temp` exists

---

### 2. "completed CSV must contain 'name', 'phone_number', and 'hash' columns"

**Error Message:**
```
Failed to initialize completed tracker: failed to load completed contacts:
completed CSV must contain 'name', 'phone_number', and 'hash' columns
```

**Cause:** The `completed.csv` file has incorrect headers

**Solution:**
1. Delete `completed.csv`
2. Run `setup-windows.bat` again
3. Or manually create `completed.csv` with this content:
   ```
   name,phone_number,hash,timestamp
   ```

---

### 3. "Chrome not found"

**Error Message:**
```
WARNING: Chrome not found in default locations
```

**Cause:** Google Chrome is not installed

**Solution:**
1. Download Chrome from: https://www.google.com/chrome/
2. Install using default settings
3. Run setup again

**Note:** Chrome MUST be installed. The application does not work with Edge or other browsers.

---

### 4. "whatsapp-automation.exe not found"

**Error Message:**
```
ERROR: whatsapp-automation.exe not found!
```

**Cause:** Missing executable file

**Solution:**
1. Make sure you downloaded the complete `windows-release` folder
2. Or run `setup.bat` to rebuild from source (requires Go)
3. Check that you're running the script from the correct folder

---

### 5. "config.yaml not found"

**Cause:** Configuration file is missing

**Solution:**
Run the setup script:
```
setup-windows.bat
```

Or manually copy:
```
copy config.example.yaml config.yaml
```

---

### 6. "contacts.csv not found"

**Cause:** Contact list is missing

**Solution:**
1. Create `contacts.csv` in the same folder as the .exe
2. Format:
   ```
   name,phone_number
   John Doe,+1234567890
   Jane Smith,+4412345678
   ```
3. **Important:** Phone numbers MUST include country code (+ and digits)

---

### 7. QR Code Timeout

**Error Message:**
```
timeout waiting for WhatsApp Web login. Please scan the QR code within 60 seconds
```

**Cause:** QR code not scanned in time

**Solutions:**

#### Solution A: Increase Timeout
1. Open `config.yaml`
2. Change `qr_timeout_seconds: 60` to `qr_timeout_seconds: 120`
3. Save and run again

#### Solution B: Run in Non-Headless Mode
1. Open `config.yaml`
2. Change `headless: true` to `headless: false`
3. This lets you see the browser window and QR code

---

### 8. Messages Not Sending

**Symptoms:** Application runs but no messages are sent

**Solutions:**

#### Check 1: Phone Number Format
Phone numbers must include country code:
- ✅ Correct: `+12025551234` (US)
- ✅ Correct: `+442012345678` (UK)
- ❌ Wrong: `2025551234` (missing +1)
- ❌ Wrong: `(202) 555-1234` (has formatting)

#### Check 2: Run Dry-Run First
```
run-dryrun.bat
```
This shows what messages would be sent without actually sending them.

#### Check 3: Check Logs
1. Open `automation.log`
2. Look for error messages
3. Common issues:
   - Invalid phone numbers
   - Template rendering errors
   - WhatsApp Web not loaded

#### Check 4: Already Sent
Check `completed.csv` - contacts listed there won't be messaged again.

---

### 9. "Permission Denied" Errors

**Cause:** Antivirus or Windows security blocking the application

**Solutions:**

#### Solution A: Add Exception
1. Open Windows Security
2. Go to Virus & threat protection
3. Manage settings
4. Add an exclusion
5. Choose Folder and select your project folder

#### Solution B: Temporarily Disable Antivirus
1. Disable antivirus temporarily
2. Run the application
3. Re-enable antivirus after

---

### 10. Browser Crashes or Closes Immediately

**Causes:** Chrome driver issues or system compatibility

**Solutions:**

#### Solution 1: Update Chrome
1. Open Chrome
2. Go to Settings → About Chrome
3. Update to latest version
4. Restart and try again

#### Solution 2: Clear Chrome Data
1. Close all Chrome windows
2. Delete the `chrome-data` folder
3. Run application again

#### Solution 3: Check System Requirements
- Windows 10 or later required
- 4GB RAM minimum recommended
- Chrome must be installed

---

## Advanced Troubleshooting

### Enable Debug Logging

1. Open `config.yaml`
2. Change `level: "info"` to `level: "debug"`
3. Run the application
4. Check `automation.log` for detailed information

### Check Application Arguments

Run with specific config:
```
whatsapp-automation.exe -config=config.yaml
```

Test without sending:
```
whatsapp-automation.exe -dry-run
```

### Verify File Contents

Run the check script:
```
check.bat
```

This validates:
- All required files exist
- File formats are correct
- Chrome is installed
- Permissions are okay

---

## Getting Help

If you still have issues:

1. Run `check.bat` and note any errors
2. Check `automation.log` for error messages
3. Take screenshots of error messages
4. Report issues at: https://github.com/abenelazar/whatsapp-automessage/issues

Include:
- Windows version
- Error messages
- Contents of automation.log (remove sensitive data)
- Steps to reproduce

---

## Clean Installation

If nothing works, try a clean install:

1. **Backup your data:**
   ```
   copy contacts.csv contacts-backup.csv
   copy template.txt template-backup.txt
   copy config.yaml config-backup.yaml
   ```

2. **Run clean script:**
   ```
   clean.bat
   ```

3. **Run setup again:**
   ```
   setup-windows.bat
   ```

4. **Restore your data:**
   ```
   copy contacts-backup.csv contacts.csv
   copy template-backup.txt template.txt
   copy config-backup.yaml config.yaml
   ```

5. **Test with dry-run:**
   ```
   run-dryrun.bat
   ```

---

## Prevention Tips

- Always run `run-dryrun.bat` first to test
- Keep Chrome updated
- Don't modify files while application is running
- Keep backups of your contacts and templates
- Review `automation.log` after each run
- Check `completed.csv` to see what's been sent

---

## Contact Support

For additional help:
- GitHub Issues: https://github.com/abenelazar/whatsapp-automessage/issues
- Email: [Add your support email]
