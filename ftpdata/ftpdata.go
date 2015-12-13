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

func SendFile(conn net.Conn, httpIp string, filePath string) (bool) {
    // Support "ASCII" mode
    // Sanity check
    url := fmt.Sprintf("http://%s/%s", httpIp, filePath)
    fmt.Printf("Sending file: %s\n", url)
    resp, err := http.Get(url)
    if err != nil {
        fmt.Println("Error trying to GET file (for RETR):", err.Error())
        return false
    }
    defer resp.Body.Close()
    _, err = io.Copy(conn, resp.Body)
    if err != nil {
        fmt.Println("Error copying file (for RETR):", err.Error())
        return false
    }
    return true
}

func SendString(conn net.Conn, data string) {
    conn.Write([]byte(data))
}
