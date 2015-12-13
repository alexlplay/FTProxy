package ftpdata

import (
    "fmt"
    "net"
    "net/http"
    "io"
)

func Close(conn net.Conn) {
    fmt.Printf("closing data connection\n")
    conn.Close()
}

func SendFile(conn net.Conn, filePath string) {
    // Support "ASCII" mode
    // Sanity check
    url := fmt.Sprintf("http://127.0.0.1/%s", filePath)
    fmt.Printf("Sending file: %s", url)
    resp, _ := http.Get(url)
    defer resp.Body.Close()
    io.Copy(conn, resp.Body)
}
