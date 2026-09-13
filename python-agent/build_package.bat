@echo off
setlocal enabledelayedexpansion

echo ===============================================================================
echo   Bounded Authority Plane (BAP) - Python SDK Package Builder
echo ===============================================================================
echo [*] Building Enterprise distribution packages for Nexus / Artifactory...

cd /d "%~dp0"

:: Clean up old artifacts
if exist "dist" rmdir /s /q "dist"
if exist "build" rmdir /s /q "build"
if exist "bap_sdk.egg-info" rmdir /s /q "bap_sdk.egg-info"

:: Build wheel and sdist using python
python setup.py sdist bdist_wheel
if %ERRORLEVEL% neq 0 (
    echo.
    echo [FAIL] Package build failed! Ensure setuptools and wheel are installed:
    echo        pip install --upgrade setuptools wheel
    exit /b %ERRORLEVEL%
)

echo.
echo ===============================================================================
echo   PACKAGE BUILD SUCCESSFUL!
echo ===============================================================================
echo Generated Artifacts in .\dist:
dir /b dist\

echo.
echo -------------------------------------------------------------------------------
echo HOW TO PUBLISH TO CORPORATE NEXUS / ARTIFACTORY:
echo -------------------------------------------------------------------------------
echo 1. Set your internal repository URL and credentials:
echo    set TWINE_REPOSITORY_URL=https://nexus.internal.company.com/repository/pypi-internal/
echo    set TWINE_USERNAME=svc-bap-publisher
echo    set TWINE_PASSWORD=your-nexus-api-token
echo.
echo 2. Upload with Twine:
echo    twine upload --repository-url https://nexus.internal.company.com/repository/pypi-internal/ dist/*
echo.
echo -------------------------------------------------------------------------------
echo HOW DEVELOPER TEAMS INSTALL FROM INTERNAL NEXUS:
echo -------------------------------------------------------------------------------
echo In their requirements.txt or pip install:
echo    pip install bap-sdk --index-url https://nexus.internal.company.com/repository/pypi-internal/simple
echo ===============================================================================

exit /b 0

