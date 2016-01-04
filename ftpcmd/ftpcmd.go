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
    // Format string according to FTP spec, see :
    // https://github.com/dagwieers/vsftpd/blob/master/ftpcmdio.c
    WriteRaw(conn, fmt.Sprintf("%d %s\r\n", status, text))
}

func WriteRaw(conn net.Conn, rawText string) {
    conn.Write([]byte(rawText))
}
