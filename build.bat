setlocal 
set GOARCH=amd64
set GOOS=windows
go build -o .\build\bandcampsync.exe
set GOARCH=386
go build -o .\build\bandcampsync32.exe
set GOARCH=amd64
set GOOS=linux
go build -o .\build\bandcampsync
set GOARCH=386
go build -o .\build\bandcampsync32