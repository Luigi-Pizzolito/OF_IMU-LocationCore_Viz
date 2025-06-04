1. Install MinGW
2. Move `.dll`s to root dir
3. `CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -v -ldflags="-s -w" -o OF_IMU-LocationCore-Viz.exe main.go`
4. `.exe` needs to be run with DLLs in the same directory