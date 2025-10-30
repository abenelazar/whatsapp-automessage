@echo off
REM WhatsApp Automation Setup Script for Windows
REM This script sets up the environment and builds the application
REM
REM NOTE: This is a Windows-only script (.bat files only run on Windows)
REM For macOS/Linux: Use shell scripts or run Go commands directly

echo ============================================
echo WhatsApp Automation - Setup (Windows)
echo ============================================
echo.
echo NOTE: This script is designed for Windows only.
echo If you're on macOS/Linux, some checks may fail - this is normal.
echo.

REM Check if Go is installed
echo [1/5] Checking for Go installation...
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: Go is not installed!
    echo.
    echo Please install Go from: https://go.dev/dl/
    echo Download the Windows installer and run it.
    echo.
    pause
    exit /b 1
)
echo Go is installed:
go version
echo.

REM Check if Chrome is installed
echo [2/5] Checking for Google Chrome...
set CHROME_PATH=
if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" set CHROME_PATH=C:\Program Files\Google\Chrome\Application\chrome.exe
if exist "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" set CHROME_PATH=C:\Program Files (x86)\Google\Chrome\Application\chrome.exe
if exist "%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe" set CHROME_PATH=%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe

if defined CHROME_PATH (
    echo Chrome found at: %CHROME_PATH%
) else (
    echo WARNING: Chrome not found in default locations
    echo Please install Chrome from: https://www.google.com/chrome/
    echo The application requires Chrome to work.
)
echo.

REM Install Go dependencies
echo [3/5] Installing Go dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo ERROR: Failed to install dependencies
    pause
    exit /b 1
)
echo Dependencies installed successfully
echo.

REM Build the application
echo [4/5] Building the application...
go build -o whatsapp-automation.exe
if %errorlevel% neq 0 (
    echo ERROR: Failed to build application
    pause
    exit /b 1
)
echo Application built successfully: whatsapp-automation.exe
echo.

REM Create config files if they don't exist
echo [5/5] Setting up configuration files...
echo.

REM Setup config.yaml
if not exist "config.yaml" (
    if exist "config.example.yaml" (
        echo Setting up config.yaml...
        copy config.example.yaml config.yaml >nul
        echo Created config.yaml from example
        echo.
        echo Let's configure your settings:
        echo.

        set /p HEADLESS="Run browser in headless mode (background)? (y/n, default=n): "
        if /i "%HEADLESS%"=="y" (
            powershell -Command "(gc config.yaml) -replace 'headless: false', 'headless: true' | Out-File -encoding ASCII config.yaml"
            echo   Updated: headless mode enabled
        )

        set /p LOG_LEVEL="Log level (debug/info/warn/error, default=info): "
        if not "%LOG_LEVEL%"=="" (
            powershell -Command "(gc config.yaml) -replace 'level: \"info\"', 'level: \"%LOG_LEVEL%\"' | Out-File -encoding ASCII config.yaml"
            echo   Updated: log level set to %LOG_LEVEL%
        )

        set /p RATE_LIMIT="Messages per second (default=1): "
        if not "%RATE_LIMIT%"=="" (
            powershell -Command "(gc config.yaml) -replace 'messages_per_second: 1', 'messages_per_second: %RATE_LIMIT%' | Out-File -encoding ASCII config.yaml"
            echo   Updated: rate limit set to %RATE_LIMIT% msg/sec
        )

        echo.
        echo Config.yaml configured successfully!
    ) else (
        echo WARNING: config.example.yaml not found
    )
) else (
    echo config.yaml already exists (skipping configuration)
)
echo.

REM Setup contacts.csv
if not exist "contacts.csv" (
    echo Setting up contacts.csv...
    if exist "contacts.example.csv" (
        copy contacts.example.csv contacts.csv >nul
        echo Created contacts.csv from example
        echo.
        echo The file contains sample contacts. Please edit contacts.csv
        echo and add your actual contacts in the format:
        echo   name,phone_number
        echo   John Doe,+1234567890
        echo.
        echo IMPORTANT: Phone numbers should include country code (e.g., +1 for US)
    ) else (
        echo Creating default contacts.csv...
        (
            echo name,phone_number
            echo John Doe,+1234567890
            echo Jane Smith,+1987654321
        ) > contacts.csv
        echo Created default contacts.csv with sample data
        echo Please edit contacts.csv and add your actual contacts!
    )
) else (
    echo contacts.csv already exists
)
echo.

REM Setup template.txt
if not exist "template.txt" (
    echo Setting up template.txt...
    echo.
    echo Enter your message template (press ENTER for default):
    echo Default template:
    echo   Hello {{.Name}}!
    echo   This is a personalized message.
    echo.
    set /p USE_DEFAULT="Use default template? (y/n, default=y): "

    if /i "%USE_DEFAULT%"=="n" (
        echo.
        echo Creating custom template...
        echo You can use {{.Name}} for contact name and {{.PhoneNumber}} for phone
        echo.
        echo Enter your message (type END on a new line when done):
        (
            setlocal enabledelayedexpansion
            set "lines="
            :read_loop
            set "line="
            set /p "line="
            if "!line!"=="END" goto end_read
            if defined lines (
                set "lines=!lines!
!line!"
            ) else (
                set "lines=!line!"
            )
            goto read_loop
            :end_read
            echo !lines!
            endlocal
        ) > template.txt
        echo Custom template created!
    ) else (
        (
            echo Hello {{.Name}}!
            echo.
            echo This is a personalized message for you.
            echo.
            echo Best regards,
            echo The Team
        ) > template.txt
        echo Default template created!
    )
) else (
    echo template.txt already exists
)
echo.

REM Create chrome-data directory if it doesn't exist
if not exist "chrome-data" (
    mkdir chrome-data
    echo Created chrome-data directory for browser session storage
    echo.
)

REM Create completed.csv if it doesn't exist
if not exist "completed.csv" (
    echo name,phone_number,timestamp,template_hash > completed.csv
    echo Created completed.csv to track sent messages
    echo.
)

echo ============================================
echo Setup completed successfully!
echo ============================================
echo.
echo Your configuration:
echo   - config.yaml: Application settings
echo   - contacts.csv: Your contact list
echo   - template.txt: Message template
echo   - completed.csv: Tracking file (auto-managed)
echo   - chrome-data/: Browser session data
echo.
echo ============================================
echo IMPORTANT - Next Steps:
echo ============================================
echo.
echo 1. EDIT contacts.csv
echo    - Add your real contacts with phone numbers
echo    - Format: name,phone_number
echo    - Include country code: +1234567890
echo.
echo 2. EDIT template.txt
echo    - Customize your message
echo    - Use {{.Name}} for personalization
echo.
echo 3. REVIEW config.yaml
echo    - Adjust settings if needed
echo.
echo 4. TEST with dry-run:
echo    - Run: run-dryrun.bat
echo    - This shows what will be sent without actually sending
echo.
echo 5. SEND messages:
echo    - Run: run.bat
echo    - Scan QR code when prompted
echo.
echo ============================================
echo.
pause
