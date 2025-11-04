@echo off
REM ====================================
REM Portable Go & Make Environment Script
REM ====================================

REM Change this to where you extracted Go, Suggested: C:\Users\username\Go\go
set "GOROOT=%USERPROFILE%\Go\go"

REM Change this to where you want your Go workspace
set "GOPATH=%USERPROFILE%\Go\gopath"

REM Add Make bin directory to PATH
set "MAKE_HOME=%USERPROFILE%\make"

REM Add Go and Make bin directories to PATH
set "PATH=%GOROOT%\bin;%GOPATH%\bin;%MAKE_HOME%\bin;%PATH%"

REM Show versions to confirm setup
echo ====================================
echo Checking installed tools...
echo ====================================

echo Checking Go...
go version 2>nul && echo Go is ready! || echo Go not found - please install Go portable to %GOROOT%

echo.
echo Checking Make...
make --version 2>nul && echo Make is ready! || echo Make not found - please install Make to %MAKE_HOME%\bin

echo.
echo Checking Protoc...
protoc --version 2>nul && echo Protoc is ready! || echo Protoc not found - you may need to install Protocol Buffers

echo ====================================
echo Environment setup complete!
echo You can now use: make build, make run, etc.
echo ====================================
echo.

REM Optional: Start a portable shell with Go and Make ready
cmd /K
