@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title 集体台账
cd /d "%~dp0"

rem ===== 配置（可按需改） =====
set "APP_PORT=8080"
set "PORT_MAX=8099"
set "DATA_DIR=%~dp0data"
set "APP_BACKUP_DIR=%~dp0backups"

rem ===== 自动检测空闲端口：8080 被占则依次 +1 =====
:probe
netstat -ano | findstr /r /c:":!APP_PORT! .*LISTENING" >nul 2>&1
if errorlevel 1 goto port_free
set /a APP_PORT+=1
if !APP_PORT! gtr %PORT_MAX% (
    echo 端口 8080-%PORT_MAX% 均被占用，请释放后重试。
    pause
    exit /b 1
)
goto probe

:port_free
echo 使用端口: %APP_PORT%
set "APP_PORT=%APP_PORT%"
set "DATA_DIR=%DATA_DIR%"
set "APP_BACKUP_DIR=%APP_BACKUP_DIR%"

rem ===== 启动服务（独立小窗口）并等待就绪 =====
start "集体台账服务 (port %APP_PORT%)" "%~dp0jititaizhang.exe"

set /a WAIT=0
:wait_ready
timeout /t 1 /nobreak >nul
set /a WAIT+=1
curl -s -o nul "http://127.0.0.1:%APP_PORT%/api/health"
if not errorlevel 1 goto open_browser
if %WAIT% geq 15 (
    echo 服务启动超时，请查看服务窗口日志。
    pause
    exit /b 1
)
goto wait_ready

:open_browser
echo 服务已就绪，正在打开 http://localhost:%APP_PORT%/
start "" "http://localhost:%APP_PORT%/"
exit /b 0
