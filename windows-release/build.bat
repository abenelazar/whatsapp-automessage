@echo off
REM WhatsApp Automation - Build Script for Windows
REM Rebuilds the application from source

echo ============================================
echo WhatsApp Automation - Build
echo ============================================
echo.

REM Check if Go is installed
echo Checking for Go installation...
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: Go is not installed!
    echo.
    echo Please install Go from: https://go.dev/dl/
    echo.
    pause
    exit /b 1
)
echo Go is installed:
go version
echo.

REM Clean previous build
if exist "whatsapp-automation.exe" (
    echo Removing previous build...
    del whatsapp-automation.exe
)

REM Build the application
echo Building application...
go build -o whatsapp-automation.exe

if %errorlevel% neq 0 (
    echo.
    echo ERROR: Build failed!
    echo.
    pause
    exit /b 1
)

echo.
echo ============================================
echo Build completed successfully!
echo ============================================
echo.
echo Created: whatsapp-automation.exe
echo.
echo To run the application: run.bat
echo.
pause
