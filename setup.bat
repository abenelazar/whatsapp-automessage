@echo off
REM WhatsApp Automation Setup Script for Windows
REM This script sets up the environment and builds the application

echo ============================================
echo WhatsApp Automation - Setup
echo ============================================
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
if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" (
    set CHROME_PATH=C:\Program Files\Google\Chrome\Application\chrome.exe
)
if exist "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" (
    set CHROME_PATH=C:\Program Files (x86)\Google\Chrome\Application\chrome.exe
)
if exist "%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe" (
    set CHROME_PATH=%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe
)

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

if not exist "config.yaml" (
    if exist "config.example.yaml" (
        copy config.example.yaml config.yaml >nul
        echo Created config.yaml from example
    ) else (
        echo WARNING: config.example.yaml not found
    )
) else (
    echo config.yaml already exists
)

if not exist "contacts.csv" (
    if exist "contacts.example.csv" (
        copy contacts.example.csv contacts.csv >nul
        echo Created contacts.csv from example
    ) else (
        echo WARNING: contacts.example.csv not found
    )
) else (
    echo contacts.csv already exists
)

if not exist "template.txt" (
    echo Creating default template.txt...
    (
        echo Hello {{.Name}}!
        echo.
        echo This is a personalized message for you.
        echo.
        echo Best regards,
        echo The Team
    ) > template.txt
    echo Created default template.txt
) else (
    echo template.txt already exists
)

echo.
echo ============================================
echo Setup completed successfully!
echo ============================================
echo.
echo Next steps:
echo 1. Edit contacts.csv with your contact list
echo 2. Edit template.txt with your message
echo 3. Review config.yaml settings
echo 4. Run: run.bat
echo.
echo For a test run without sending messages:
echo Run: run-dryrun.bat
echo.
pause
