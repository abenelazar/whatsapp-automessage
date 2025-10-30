@echo off
REM WhatsApp Automation - Complete Windows Setup
REM This script sets up everything needed to run the pre-compiled application
REM No Go installation required!

echo ============================================
echo WhatsApp Automation - Windows Setup
echo ============================================
echo.
echo This script will help you set up WhatsApp Automation
echo on your Windows system.
echo.
pause

REM Check if Chrome is installed
echo.
echo ============================================
echo [1/6] Checking for Google Chrome...
echo ============================================
set CHROME_PATH=
if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" set CHROME_PATH=C:\Program Files\Google\Chrome\Application\chrome.exe
if exist "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" set CHROME_PATH=C:\Program Files (x86)\Google\Chrome\Application\chrome.exe
if exist "%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe" set CHROME_PATH=%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe

if defined CHROME_PATH (
    echo [OK] Chrome found at: %CHROME_PATH%
) else (
    echo [WARNING] Chrome not found in default locations
    echo.
    echo The application requires Google Chrome to work.
    echo Please install Chrome from: https://www.google.com/chrome/
    echo.
    echo After installing Chrome, you can continue with the setup.
    echo.
    set /p CONTINUE="Continue anyway? (y/n): "
    if /i not "%CONTINUE%"=="y" (
        echo Setup cancelled.
        pause
        exit /b 1
    )
)
echo.

REM Check for executable
echo ============================================
echo [2/6] Checking for application executable...
echo ============================================
if not exist "whatsapp-automation.exe" (
    echo [ERROR] whatsapp-automation.exe not found!
    echo.
    echo Please make sure you have downloaded the complete package.
    echo The executable should be in the same folder as this setup script.
    echo.
    pause
    exit /b 1
)
echo [OK] Found whatsapp-automation.exe
echo.

REM Setup config.yaml
echo ============================================
echo [3/6] Setting up configuration (config.yaml)...
echo ============================================
if exist "config.yaml" (
    echo [SKIP] config.yaml already exists
    set /p OVERWRITE="Do you want to reconfigure it? (y/n): "
    if /i not "%OVERWRITE%"=="y" goto skip_config
)

if exist "config.example.yaml" (
    copy config.example.yaml config.yaml >nul
) else (
    REM Create default config if example doesn't exist
    (
        echo # WhatsApp Automation Configuration
        echo.
        echo browser:
        echo   headless: false
        echo   user_data_dir: "./chrome-data"
        echo   qr_timeout_seconds: 60
        echo   page_load_timeout: 30
        echo.
        echo files:
        echo   csv_path: "contacts.csv"
        echo   template_path: "template.txt"
        echo   completed_csv_path: "completed.csv"
        echo.
        echo retry:
        echo   max_retries: 3
        echo   initial_delay_seconds: 2
        echo   max_delay_seconds: 30
        echo   backoff_multiplier: 2
        echo.
        echo rate_limiting:
        echo   messages_per_second: 1
        echo   enabled: true
        echo.
        echo logging:
        echo   level: "info"
        echo   output_file: "automation.log"
    ) > config.yaml
)

echo [OK] Created config.yaml
echo.
echo Let's configure your settings:
echo.

set /p HEADLESS="Run browser in headless mode (background)? (y/n, default=n): "
if /i "%HEADLESS%"=="y" (
    powershell -Command "(gc config.yaml) -replace 'headless: false', 'headless: true' | Out-File -encoding ASCII config.yaml"
    echo   [UPDATED] Headless mode enabled
)

set /p LOG_LEVEL="Log level (debug/info/warn/error, default=info): "
if not "%LOG_LEVEL%"=="" (
    powershell -Command "(gc config.yaml) -replace 'level: \"info\"', 'level: \"%LOG_LEVEL%\"' | Out-File -encoding ASCII config.yaml"
    echo   [UPDATED] Log level set to %LOG_LEVEL%
)

set /p RATE_LIMIT="Messages per second (1-5, default=1): "
if not "%RATE_LIMIT%"=="" (
    powershell -Command "(gc config.yaml) -replace 'messages_per_second: 1', 'messages_per_second: %RATE_LIMIT%' | Out-File -encoding ASCII config.yaml"
    echo   [UPDATED] Rate limit set to %RATE_LIMIT% msg/sec
)

echo.
echo [OK] Configuration completed!

:skip_config
echo.

REM Setup contacts.csv
echo ============================================
echo [4/6] Setting up contacts (contacts.csv)...
echo ============================================
if exist "contacts.csv" (
    echo [SKIP] contacts.csv already exists
    echo.
    set /p VIEW_CONTACTS="View current contacts? (y/n): "
    if /i "%VIEW_CONTACTS%"=="y" (
        type contacts.csv
        echo.
    )
    goto skip_contacts
)

if exist "contacts.example.csv" (
    copy contacts.example.csv contacts.csv >nul
) else (
    REM Create default contacts.csv
    (
        echo name,phone_number
        echo John Doe,+1234567890
        echo Jane Smith,+1987654321
    ) > contacts.csv
)

echo [OK] Created contacts.csv with sample data
echo.
echo IMPORTANT: You need to edit contacts.csv and add your real contacts!
echo.
echo Format:
echo   name,phone_number
echo   John Doe,+1234567890
echo.
echo NOTE: Phone numbers MUST include country code (e.g., +1 for US, +44 for UK)
echo.

:skip_contacts
echo.

REM Setup template.txt
echo ============================================
echo [5/6] Setting up message template (template.txt)...
echo ============================================
if exist "template.txt" (
    echo [SKIP] template.txt already exists
    echo.
    echo Current template:
    type template.txt
    echo.
    set /p EDIT_TEMPLATE="Do you want to edit the template? (y/n): "
    if /i not "%EDIT_TEMPLATE%"=="y" goto skip_template
)

echo.
echo You can use these placeholders in your template:
echo   {{.Name}} - Will be replaced with the contact's name
echo   {{.PhoneNumber}} - Will be replaced with the phone number
echo.

set /p USE_DEFAULT="Use default template? (y/n, default=y): "
if /i "%USE_DEFAULT%"=="n" (
    echo.
    echo Enter your custom message below.
    echo When finished, type END on a new line and press Enter.
    echo.
    echo Example:
    echo   Hello {{.Name}},
    echo   This is a test message.
    echo   END
    echo.

    REM Simple method - just create default and ask user to edit
    (
        echo Hello {{.Name}}!
        echo.
        echo This is a personalized message for you.
        echo.
        echo Please edit this file to customize your message.
        echo.
        echo Best regards,
        echo The Team
    ) > template.txt

    echo [OK] Template created. Please edit template.txt to customize your message.

    set /p EDIT_NOW="Open template.txt in Notepad now? (y/n): "
    if /i "%EDIT_NOW%"=="y" (
        notepad template.txt
    )
) else (
    (
        echo Hello {{.Name}}!
        echo.
        echo This is a personalized message for you.
        echo.
        echo Best regards,
        echo The Team
    ) > template.txt
    echo [OK] Default template created
)

:skip_template
echo.

REM Create necessary directories and files
echo ============================================
echo [6/6] Creating necessary directories...
echo ============================================

if not exist "chrome-data" (
    mkdir chrome-data
    echo [OK] Created chrome-data directory (browser session storage)
)

if not exist "completed.csv" (
    echo name,phone_number,hash,timestamp > completed.csv
    echo [OK] Created completed.csv (tracks sent messages)
)

echo.
echo ============================================
echo Setup Completed Successfully!
echo ============================================
echo.
echo Your setup:
echo   [CONFIG] config.yaml - Application settings
echo   [DATA]   contacts.csv - Your contact list
echo   [DATA]   template.txt - Message template
echo   [AUTO]   completed.csv - Sent message tracker
echo   [AUTO]   chrome-data/ - Browser session data
echo   [AUTO]   automation.log - Application logs
echo.
echo ============================================
echo IMPORTANT - Before Running:
echo ============================================
echo.
echo 1. EDIT contacts.csv
echo    - Open with Notepad or Excel
echo    - Add your real contacts
echo    - Format: name,phone_number
echo    - Example: John Doe,+1234567890
echo    - MUST include country code!
echo.
echo 2. EDIT template.txt
echo    - Open with Notepad
echo    - Customize your message
echo    - Use {{.Name}} for personalization
echo.
echo 3. REVIEW config.yaml (optional)
echo    - Open with Notepad if you want to adjust advanced settings
echo.
echo ============================================
echo How to Use:
echo ============================================
echo.
echo TEST MODE (recommended first):
echo   - Double-click: run-dryrun.bat
echo   - Shows what messages will be sent WITHOUT actually sending
echo   - Safe to test your setup
echo.
echo SEND MESSAGES:
echo   - Double-click: run.bat
echo   - Browser will open WhatsApp Web
echo   - Scan QR code with your phone
echo   - Messages will be sent automatically
echo.
echo OTHER COMMANDS:
echo   - build.bat - Rebuild the application (requires Go)
echo   - clean.bat - Clean up logs and session data
echo.
echo ============================================
echo.

set /p EDIT_FILES="Do you want to edit contacts.csv now? (y/n): "
if /i "%EDIT_FILES%"=="y" (
    notepad contacts.csv
)

echo.
echo Setup complete! You're ready to go.
echo.
echo Run 'run-dryrun.bat' to test your setup.
echo.
pause
