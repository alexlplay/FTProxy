package main

import (
    "fmt"
    "net"
    "os"
    "bufio"
    "strings"
    "ftpcmd"
    "ftpdata"
    "parseindex"
    // "net/http"
    "time"
    "cfg"
    "sync"
)

const (
    CONN_HOST = "0"
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

// global state
type State struct {
    sync.RWMutex
    connectionCount int
}

// per command connexion
type Session struct {
    username string
    loggedIn bool
    commandConn net.Conn            // Command connection
    dataConn net.Conn               // Command connection
    dtpState DtpState
    pasvListener *net.TCPListener   // Listener in PASV mode
    workingDir string
    timer *time.Timer
    topDirParser parseindex.Parser     // Parser to generate top level LIST
    defaultDirParser parseindex.Parser  // Default parser
}

var state State

func main() {
    // Load config (todo, try to use memoization in cfg.go)
    cfg.LoadConfig("ftproxy.conf")
    listenPort := cfg.GetListenPort()
    maxConnections := cfg.GetMaxConnections()
    // Listen for incoming connections.
    l, err := net.Listen(CONN_TYPE, CONN_HOST+":"+listenPort)
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
        state.RLock()
        connCount := state.connectionCount
        state.RUnlock()
        if connCount > (maxConnections - 1) {
            ftpcmd.Write(conn, 421, "Too many connections.")
            ftpcmd.Close(conn)
            fmt.Printf("Too many connections (%d) connection closed\n", state.connectionCount)
        }
        fmt.Printf("Connections: %d\n", connCount+1)
        state.Lock()
        state.connectionCount++
        state.Unlock()
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
        "CWD":  cmdCwd,
        "LIST": cmdList,
        "FEAT": cmdFeat,
    }

    scanner := bufio.NewScanner(conn)
    session := Session{commandConn: conn, workingDir: "/", topDirParser: new(parseindex.ParserConf), defaultDirParser: new(parseindex.ParserAutoIndex)}
    var cmdCallBack func(session *Session, command Command) (bool)
    var exists bool


    conn.Write([]byte("220 (FTProxy)\n"))
    session.timer = time.NewTimer(time.Second * 60)
    go ctrlTimeout(&session)

    for scanner.Scan() {
        session.timer.Reset(time.Second * 60)
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

func ctrlTimeout(session *Session) (bool) {
    <- session.timer.C
    ftpcmd.Close(session.commandConn)
    state.Lock()
    state.connectionCount--
    state.Unlock()
    return true
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
        ftpcmd.Write(session.commandConn, 503, "Already logged in.")
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
    session.timer.Stop()
    ftpcmd.Write(session.commandConn, 221, "Goodbyye.")
    ftpcmd.Close(session.commandConn)
    state.Lock()
    state.connectionCount--
    state.Unlock()
    return true
}

func cmdUnknown(session *Session) (bool) {
    ftpcmd.Write(session.commandConn, 500, "Unknown command.")
    return false
}

func cmdPasv(session *Session, command Command) (bool) {
    addr := session.commandConn.LocalAddr()
    fmt.Println(addr)
    test := addr.(*net.TCPAddr).IP
    fmt.Println(test)
    // laddr := net.TCPAddr{IP: net.IPv4(51, 255, 255, 51), Port: 0}
    laddr := net.TCPAddr{IP: session.commandConn.LocalAddr().(*net.TCPAddr).IP, Port: 0}
    ln, err := net.ListenTCP("tcp4", &laddr)
    if err != nil {
        fmt.Println(err)
        ftpcmd.Write(session.commandConn, 500,  "PASV failed.")
        return false
    }
    if session.pasvListener != nil {
        ftpcmd.Write(session.commandConn, 526,  "Already listening.")
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

    if session.dtpState != DTP_PASSIVE {
        ftpcmd.Write(session.commandConn, 425, "Only PASV implemented.")
        return false
    }

    // Assume passive session from here
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

    session.pasvListener.Close()
    session.pasvListener = nil
    session.dtpState = DTP_NONE

    ftpcmd.Write(session.commandConn, 150, "Opening BINARY mode data connection for x.")
    // Sanity check and space support with quotes
    filePath := ""
    if strings.HasPrefix(command.Args, "/") {
        filePath = command.Args
    } else {
        filePath = fmt.Sprintf("%s/%s", session.workingDir, command.Args)
    }
    vhost := cfg.GetVhost(session.workingDir)
    ret := ftpdata.SendFile(session.dataConn, vhost, filePath)
    if ret != true {
        ftpcmd.Write(session.commandConn, 526, "Transfer failed.")
        ftpdata.Close(session.dataConn)
    }
    ftpdata.Close(session.dataConn)
    ftpcmd.Write(session.commandConn, 226, "Transfer complete.")

    return true
}

func cmdPwd(session *Session, command Command) (bool) {
    ftpcmd.Write(session.commandConn, 257, fmt.Sprintf("\"%s\"", session.workingDir))
    return true
}

func cmdCwd(session *Session, command Command) (bool) {
    // Needs at least a sanity check
    // Needs to support relative paths
    newPath := command.Args
    if strings.HasPrefix(newPath, "/") {
        session.workingDir = newPath
    } else {
        if strings.HasSuffix(session.workingDir, "/") {
            session.workingDir = fmt.Sprintf("%s%s", session.workingDir, newPath)
        } else {
            session.workingDir = fmt.Sprintf("%s/%s", session.workingDir, newPath)
        }
    }
    ftpcmd.Write(session.commandConn, 250, "Directory successfully changed.")
    return true
}

func cmdList(session *Session, command Command) (bool) {
    fmt.Println("BEGIN CMDLIST")
    if session.dtpState == DTP_NONE {
        ftpcmd.Write(session.commandConn, 425, "Use PORT or PASV first.")
        return false
    }

    if session.dtpState != DTP_PASSIVE {
        ftpcmd.Write(session.commandConn, 425, "Only PASV implemented")
        return false
    }

    // Assume passive session from here
    // Same code as in RETR, factor it in AcceptAndClose()
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
    session.dataConn = conn

    // Stop listening
    session.pasvListener.Close()
    session.pasvListener = nil
    session.dtpState = DTP_NONE

    ftpcmd.Write(session.commandConn, 150, "Opening BINARY mode data connection for x.")

    // Sending list.txt works, next two lines
    // listFile := fmt.Sprintf("%s/%s", session.workingDir, "list.txt")
    // ftpdata.SendFile(session.dataConn, listFile)

    var listing string
    var ret bool
    if session.workingDir == "/" {
        fmt.Println("CALLING TOP DIR PARSER")
        listing, ret = session.topDirParser.Parse(session.workingDir)
    } else {
        fmt.Println("CALLING DEFAULT DIR PARSER")
        listing, ret = session.defaultDirParser.Parse(session.workingDir)
    }
    if ret != true {
        ftpcmd.Write(session.commandConn, 526, "Failed to send directory, please retry.")
        return false
    }

    // Attempt to use autoindex --start
    /* vhost := cfg.GetVhost(session.workingDir)
    url := fmt.Sprintf("http://%s/%s/", vhost, session.workingDir)
    fmt.Printf("LIST for url: %s\n", url)
    resp, err := http.Get(url)
    if err != nil {
        fmt.Println("Error trying to GET current directory (for LIST):", err.Error())
        ftpcmd.Write(session.commandConn, 526, "Failed to send directory, please retry.")
        return false
    }
    defer resp.Body.Close()
    objects := parseindex.ParseHtmlList(resp.Body)
    fmt.Printf("Directory objects: %d\n", len(objects))
    listing := parseindex.GenDirList(objects)
    */
    ftpdata.SendString(session.dataConn, listing)
    // Attempt to use autoindex --end

    ftpdata.Close(session.dataConn)
    ftpcmd.Write(session.commandConn, 226, "Directory send OK.")

    return true
}

func cmdFeat(session *Session, command Command) (bool) {
    featReply := "211-Features:\r\n MDTM\r\n211 End\r\n"

    ftpcmd.WriteRaw(session.commandConn, featReply)
    return true
}
