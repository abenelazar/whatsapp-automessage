@echo off
REM WhatsApp Automation - Run Script for Windows
REM Runs the WhatsApp automation with normal settings

echo ============================================
echo WhatsApp Automation - Starting
echo ============================================
echo.

REM Check if executable exists
if not exist "whatsapp-automation.exe" (
    echo ERROR: whatsapp-automation.exe not found!
    echo.
    echo Please run setup.bat first to build the application.
    echo.
    pause
    exit /b 1
)

REM Check if config exists
if not exist "config.yaml" (
    echo ERROR: config.yaml not found!
    echo.
    if exist "config.example.yaml" (
        echo Creating config.yaml from example...
        copy config.example.yaml config.yaml
    ) else (
        echo Please create config.yaml or run setup.bat
        pause
        exit /b 1
    )
)

REM Check if contacts exist
if not exist "contacts.csv" (
    echo ERROR: contacts.csv not found!
    echo.
    echo Please create contacts.csv with your contact list.
    echo Format:
    echo name,phone_number
    echo John Doe,+1234567890
    echo.
    pause
    exit /b 1
)

REM Check if template exists
if not exist "template.txt" (
    echo ERROR: template.txt not found!
    echo.
    echo Please create template.txt with your message template.
    echo.
    pause
    exit /b 1
)

echo Starting WhatsApp Automation...
echo.
echo Press Ctrl+C to stop
echo.

REM Run the application
whatsapp-automation.exe

echo.
echo ============================================
echo WhatsApp Automation - Finished
echo ============================================
echo.
echo Check automation.log for details
echo.
pause
