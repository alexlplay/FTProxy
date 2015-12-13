package main

import (
    "fmt"
    "net"
    "os"
    "bufio"
    "strings"
    "ftpcmd"
    "ftpdata"
)

const (
    CONN_HOST = "localhost"
    CONN_PORT = "3333"
    CONN_TYPE = "tcp"
)

type DtpState int

const (
    DTP_NONE DtpState = iota
    DTP_ACTIVE
    DTP_PASSIVE
)

type Command struct {
    Verb string
    Args string
}

type Session struct {
    username string
    loggedIn bool
    commandConn net.Conn            // Command connection
    dataConn net.Conn               // Command connection
    dtpState DtpState
    pasvListener *net.TCPListener   // Listener in PASV mode
    workingDir string
}

func main() {
    // Listen for incoming connections.
    l, err := net.Listen(CONN_TYPE, CONN_HOST+":"+CONN_PORT)
    if err != nil {
        fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    // Close the listener when the application closes.
    defer l.Close()
    fmt.Println("Listening on " + CONN_HOST + ":" + CONN_PORT)
    for {
        // Listen for an incoming connection.
        conn, err := l.Accept()
        if err != nil {
            fmt.Println("Error accepting: ", err.Error())
            os.Exit(1)
        }
        // Handle connections in a new goroutine.
        go handleRequest(conn)
    }
}


// Handles incoming requests.
// It should have a timeout, and maybe wait before replying on some condition (negative replies?)
func handleRequest(conn net.Conn) {
    // Valid commands when not authenticated
    noauthFuncs := map[string]func(session *Session, command Command) (bool) {
        "USER": cmdUser,
        "PASS": cmdPass,
        "QUIT": cmdQuit,
    }

    // Valid commands when authenticated
    authFuncs := map[string]func(session *Session, command Command) (bool) {
        "USER": cmdUser,
        "PASS": cmdPass,
        "MODE": cmdMode,
        "TYPE": cmdType,
        "QUIT": cmdQuit,
        "PASV": cmdPasv,
        "RETR": cmdRetr,
        "PWD":  cmdPwd,
    }

    scanner := bufio.NewScanner(conn)
    session := Session{commandConn: conn, workingDir: "/"}
    var cmdCallBack func(session *Session, command Command) (bool)
    var exists bool

    conn.Write([]byte("220 (FTProxy)\n"))

    for scanner.Scan() {
        // Should have a limit on line length
        fmt.Println(session)
        line := scanner.Text()
        var callBackRet bool
        command := parseCommand(&line)
        fmt.Printf("cmd: '%s' args: '%s'\n", command.Verb, command.Args)
        // TODO: Check is user is logged in, use a different map for logged in/not logged in
        if session.loggedIn != true {
            cmdCallBack, exists = noauthFuncs[command.Verb]
        } else
        {
            cmdCallBack, exists = authFuncs[command.Verb]
        }
        if exists == true {
            callBackRet = cmdCallBack(&session, command)
        } else {
            callBackRet = cmdUnknown(&session)
        }
        fmt.Println(callBackRet)
        // conn.Write([]byte(strRet + "\n"))
    }
}

func parseCommand(line *string) (Command) {
    pieces := strings.SplitN(*line, " ", 2)
    command := Command{Verb: strings.ToUpper(pieces[0])}
    if len(pieces) > 1 {
        command.Args = pieces[1]
    }
    return command
}

func cmdUser(session *Session, command Command) (bool) {
    if session.loggedIn == true {
        ftpcmd.Write(session.commandConn, 530, "Already logged-in.")
        return true
    }
    username := command.Args
    fmt.Printf("Handling USER command, username: '%s'\n", username)
    if len(username) > 0 {
        session.username = username
        ftpcmd.Write(session.commandConn, 331, fmt.Sprintf("Password required for %s", username))
        return true
    }
    return true
}

func cmdPass(session *Session, command Command) (bool) {
    if session.username == "" {
        ftpcmd.Write(session.commandConn, 503, "Login with USER first.")
        return true
    }
    if session.loggedIn == true {
        ftpcmd.Write(session.commandConn, 503, "530 Already logged in.")
        return true
    }
    session.loggedIn = true
    ftpcmd.Write(session.commandConn, 230, fmt.Sprintf("User %s logged in", session.username))
    return true
}

// Does nothing, only support Stream
func cmdMode(session *Session, command Command) (bool) {
    if strings.ToUpper(command.Args) != "S" {
        ftpcmd.Write(session.commandConn, 504, "Bad MODE command.")
        return false
    } else {
        ftpcmd.Write(session.commandConn, 200, "Mode set to S.")
        return true
    }
}

// Does nothing, only support binary
func cmdType(session *Session, command Command) (bool) {
    uppercaseArgs := strings.ToUpper(command.Args)

    if uppercaseArgs == "A" || uppercaseArgs == "A T" {
        ftpcmd.Write(session.commandConn, 200, "Switching to ASCII mode.")
        return true
    } else if uppercaseArgs == "I" {
        ftpcmd.Write(session.commandConn, 200, "Switching to Binary mode.")
        return true
    } else {
        ftpcmd.Write(session.commandConn, 500, "Unrecognised TYPE command.")
        return true
    }
}

func cmdQuit(session *Session, command Command) (bool) {
    ftpcmd.Write(session.commandConn, 221, "Goodbye.")
    ftpcmd.Close(session.commandConn)
    return true
}

func cmdUnknown(session *Session) (bool) {
    ftpcmd.Write(session.commandConn, 500, "Unknown command.")
    return false
}

func cmdPasv(session *Session, command Command) (bool) {
    laddr := net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
    ln, err := net.ListenTCP("tcp4", &laddr)
    if err != nil {
        fmt.Println(err)
        ftpcmd.Write(session.commandConn, 500,  "5xx PASV failed")
        return false
    }
    ip := ln.Addr().(*net.TCPAddr).IP
    port := ln.Addr().(*net.TCPAddr).Port
    session.dtpState = DTP_PASSIVE
    session.pasvListener = ln
    fmt.Printf("Listening on IP: %s port: %d\n",ip , port)
    // Format IP and port according to FTP spec (see RFC)
    high := port >> 8
    low := port & 0xFF
    reply := fmt.Sprintf("Entering Passive Mode (%d,%d,%d,%d,%d,%d).", ip[0], ip[1], ip[2], ip[3], high, low)
    ftpcmd.Write(session.commandConn,  227, reply)
    return true
}

func cmdRetr(session *Session, command Command) (bool) {
    if session.dtpState == DTP_NONE {
        ftpcmd.Write(session.commandConn, 425, "Use PORT or PASV first.")
        return false
    }

    if session.dtpState == DTP_PASSIVE {
        conn, err := session.pasvListener.AcceptTCP()
        if err != nil {
            fmt.Println(err)
            // make sure ln is destroyed
            session.pasvListener.Close()
            session.pasvListener = nil
            session.dtpState = DTP_NONE
            ftpcmd.Write(session.commandConn, 500, "Failed to accept data connection.")
            return false
        }
        fmt.Println(conn)
        session.dataConn = conn
    }

    session.pasvListener.Close()
    session.pasvListener = nil
    session.dtpState = DTP_NONE

    ftpcmd.Write(session.commandConn, 150, "Opening BINARY mode data connection for x.")
    // Sanity check and space support with quotes
    filePath := command.Args
    ftpdata.SendFile(session.dataConn, filePath)
    ftpdata.Close(session.dataConn)
    ftpcmd.Write(session.commandConn, 226, "Transfer complete.")

    return true
}

func cmdPwd(session *Session, command Command) (bool) {
    ftpcmd.Write(session.commandConn, 257, fmt.Sprintf("\"%s\"", session.workingDir))
    return true
}
