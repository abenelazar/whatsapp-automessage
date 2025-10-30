@echo off
REM WhatsApp Automation - Clean Script for Windows
REM Removes session data, logs, and tracking files

echo ============================================
echo WhatsApp Automation - Clean
echo ============================================
echo.
echo WARNING: This will remove:
echo - chrome-data folder (WhatsApp session)
echo - automation.log (log file)
echo - completed.csv (tracking file)
echo.
echo You will need to scan the QR code again
echo after cleaning the session data.
echo.

set /p CONFIRM="Are you sure? (y/N): "
if /i not "%CONFIRM%"=="y" (
    echo.
    echo Cancelled.
    pause
    exit /b 0
)

echo.
echo Cleaning...

REM Remove chrome-data directory
if exist "chrome-data" (
    echo Removing chrome-data folder...
    rmdir /s /q chrome-data
    echo Removed chrome-data
) else (
    echo chrome-data folder not found
)

REM Remove log file
if exist "automation.log" (
    echo Removing automation.log...
    del automation.log
    echo Removed automation.log
) else (
    echo automation.log not found
)

REM Remove completed tracking
if exist "completed.csv" (
    echo Removing completed.csv...
    del completed.csv
    echo Removed completed.csv
) else (
    echo completed.csv not found
)

echo.
echo ============================================
echo Clean completed!
echo ============================================
echo.
echo Session data has been removed.
echo Next time you run the application, you will
echo need to scan the QR code again.
echo.
pause
