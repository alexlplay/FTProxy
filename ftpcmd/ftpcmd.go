package ftpcmd

import (
    "fmt"
    "net"
)

func Close(conn net.Conn) {
    fmt.Printf("closing command connection\n")
    conn.Close()
}

func Write(conn net.Conn, status int, text string) {
    // Must format string according to FTP spec, see :
    // https://github.com/dagwieers/vsftpd/blob/master/ftpcmdio.c
    response := fmt.Sprintf("%d %s\r\n", status, text)
    conn.Write([]byte(response))
}
