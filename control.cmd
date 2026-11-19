@echo off
title SmartPing

setlocal
set uac=~uac_permission_tmp_%random%
md "%SystemRoot%\system32\%uac%" 2>nul
if %errorlevel%==0 ( rd "%SystemRoot%\system32\%uac%" >nul 2>nul ) else (
    echo set uac = CreateObject^("Shell.Application"^)>"%temp%\%uac%.vbs"
    echo uac.ShellExecute "%~s0","","","runas",1 >>"%temp%\%uac%.vbs"
    echo WScript.Quit >>"%temp%\%uac%.vbs"
    "%temp%\%uac%.vbs" /f
    del /f /q "%temp%\%uac%.vbs" & exit )
endlocal  

:BG
cls
echo ������������������������������������������������������������������������������������������������������������������������������
echo ��                        SmartPing                            ��
echo ������������������������������������������������������������������������������������������������������������������������������
echo ��                                                             ��
echo ��INS USE                                                      ��
echo ��        build      run go get and build                      ��
echo ��        run        run smartpingplus                             ��
echo ��        install    install smartpingplus as service (use nssm)   ��
echo ��        uninstall  uninstall smartpingplus service               ��
echo ��        start      start smartpingplus service (after install)   ��
echo ��        stop       stop smartpingplus service                    ��
echo ��        restart    stop and start smartpingplus                  ��
echo ��        version    show smartpingplus version                    ��
echo ��                                                             ��
echo ������������������������������������������������������������������������������������������������������������������������������

%~d0
cd %~dp0
::SET select=
SET /P select="Please Enter Instructions:"
IF "%select%"=="build" (
    ::go get -v ./...
    ::go build -o %~dp0\bin\goping.exe  %~dp0\src\ping\goping\goping.go
    go build -o %~dp0\bin\smartpingplus.exe  %~dp0\src\smartping.go
    echo Build Finish..
    pause
    GOTO BG
) ELSE (
    IF "%select%"=="run" (
        %~dp0/bin/smartpingplus.exe 
    ) ELSE ( 
        IF "%select%"=="install" (
            %~dp0\\bin\\nssm.exe install smartpingplus %~dp0\\bin\\smartpingplus.exe 
            pause
            GOTO BG
        ) ELSE ( 
            IF "%select%"=="start" (
                net start smartpingplus 
                pause
                GOTO BG
            ) ELSE (
                IF "%select%"=="stop" (
                    net stop smartpingplus 
                    pause
                    GOTO BG
                ) ELSE (
                    IF "%select%"=="restart" (
                        net stop smartpingplus 
                        net start smartpingplus 
                        pause
                        GOTO BG
                    ) ELSE (
                        IF "%select%"=="uninstall" (
                            sc delete smartpingplus 
                            pause
                            GOTO BG
                        ) ELSE (
                             IF "%select%"=="version" (
                                %~dp0\bin\smartpingplus.exe -v 
                                pause
                                GOTO BG
                            ) ELSE (
                                 echo Param Error Try Again!
                                 pause
                                 GOTO BG
                            )
                        ) 
                    ) 
                ) 
            ) 
        ) 
    )
)

pause

exit