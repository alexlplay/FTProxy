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
    "path"
    "net/http"
    "time"
    "cfg"
    "sync"
    "strconv"
)

const (
    CONN_HOST = ""
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
}

var state State

func main() {
    // Load config (todo, try to use memorization in cfg.go)
    cfg.LoadConfig("ftproxy.conf")
    listenPort := cfg.GetListenPort()
    maxConnections := cfg.GetMaxConnections()
    // Listen for incoming connections.
    l, err := net.Listen(CONN_TYPE, CONN_HOST + ":" + listenPort)
    if err != nil {
        fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    // Close the listener when the application closes.
    defer l.Close()
    fmt.Println("Listening on " + CONN_HOST + ":" + listenPort)
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
        "FEAT": cmdFeat,
        "USER": cmdUser,
        "PASS": cmdPass,
        "QUIT": cmdQuit,
    }

    // Valid commands when authenticated
    authFuncs := map[string]func(session *Session, command Command) (bool) {
        "FEAT": cmdFeat,
        "USER": cmdUser,
        "PASS": cmdPass,
        "MODE": cmdMode,
        "TYPE": cmdType,
        "QUIT": cmdQuit,
        "PASV": cmdPasv,
        "EPSV": cmdEpsv,
        "RETR": cmdRetr,
        "PWD":  cmdPwd,
        "CWD":  cmdCwd,
        "LIST": cmdList,
        "MDTM": cmdMdtm,
        "SIZE": cmdSize,
        "SYST": cmdSyst,
    }

    scanner := bufio.NewScanner(conn)
    session := Session{commandConn: conn, workingDir: "/"}
    var cmdCallBack func(session *Session, command Command) (bool)
    var exists bool

    ftpcmd.Write(session.commandConn, 220, "(FTProxy)")
    session.timer = time.NewTimer(time.Second * 60)
    go ctrlTimeout(&session)

    for scanner.Scan() {
        session.timer.Reset(time.Second * 60 * 3)
        // Should have a limit on line length
        fmt.Printf("----\n")
        fmt.Printf("Session: ")
        fmt.Println(session)
        line := scanner.Text()
        var callBackRet bool
        command := parseCommand(&line)
        fmt.Printf("=> cmd: '%s', args: '%s'\n", command.Verb, command.Args)
        if session.loggedIn != true {
            cmdCallBack, exists = noauthFuncs[command.Verb]
            if exists == true {
                callBackRet = cmdCallBack(&session, command)
            } else {
                _, exists = authFuncs[command.Verb]
                if exists == true {
                    callBackRet = cmdLoginFirst(&session)
                } else {
                    callBackRet = cmdUnknown(&session)
                }
            }
        } else {
            cmdCallBack, exists = authFuncs[command.Verb]
            if exists == true {
                callBackRet = cmdCallBack(&session, command)
            } else {
                callBackRet = cmdUnknown(&session)
            }
        }
        fmt.Printf("<= Returns: ")
        fmt.Println(callBackRet)
        // conn.Write([]byte(strRet + "\n"))
    }

    /* XXX 'QUIT' command and timer force the previous loop to exit
       with a 'use of closed network connection' error ;
       the following tests for a TCP-level disconnection */
    if scanner.Err() == nil {
        session.timer.Stop()
        ftpcmd.Close(conn)
        state.Lock()
        state.connectionCount--
        state.Unlock()

        if session.pasvListener != nil {
            session.pasvListener.Close()
            session.pasvListener = nil
            session.dtpState = DTP_NONE
        }
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

    if session.pasvListener != nil {
        session.pasvListener.Close()
        session.pasvListener = nil
        session.dtpState = DTP_NONE
    }
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
        cmdLoginFirst(session)
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

    if session.pasvListener != nil {
        session.pasvListener.Close()
        session.pasvListener = nil
        session.dtpState = DTP_NONE
    }
    return true
}

func cmdLoginFirst(session *Session) (bool) {
    ftpcmd.Write(session.commandConn, 503, "Login with USER first.")
    return false
}

func cmdUnknown(session *Session) (bool) {
    ftpcmd.Write(session.commandConn, 500, "Unknown command.")
    return false
}

func cmdPasv(session *Session, command Command) (bool) {
    laddr, err := net.ResolveTCPAddr("tcp", CONN_HOST + ":0")
    if err != nil {
        fmt.Println(err)
        ftpcmd.Write(session.commandConn, 500,  "PASV failed.")
        return false
    }
    ln, err := net.ListenTCP("tcp", laddr)
    if err != nil {
        fmt.Println(err)
        ftpcmd.Write(session.commandConn, 500,  "PASV failed.")
        return false
    }
    if session.pasvListener != nil {
        ftpcmd.Write(session.commandConn, 526,  "Already listening.")
        return false
    }
    session.dtpState = DTP_PASSIVE
    session.pasvListener = ln

    /* We are listening on both IPv4 and IPv6, adapt answer given the current *command* protocol */
    ip := session.commandConn.LocalAddr().(*net.TCPAddr).IP.To4()
    port := ln.Addr().(*net.TCPAddr).Port

    var reply string
    if ip == nil {
        /* IPv6, return 0.0.0.0 as IPv4 addr */
        reply = fmt.Sprintf("Entering Passive Mode (0,0,0,0,%d,%d).", port >> 8, port & 0xFF)
    } else {
        /* IPv4, return real address */
        reply = fmt.Sprintf("Entering Passive Mode (%d,%d,%d,%d,%d,%d).", ip[0], ip[1], ip[2], ip[3], port >> 8, port & 0xFF)
    }
    ftpcmd.Write(session.commandConn, 227, reply)
    fmt.Printf("cmdPasv(): listening on port: %d\n", port)

    return true
}

func cmdEpsv(session *Session, command Command) (bool) {
    laddr, err := net.ResolveTCPAddr("tcp", CONN_HOST + ":0")
    if err != nil {
        fmt.Println(err)
        ftpcmd.Write(session.commandConn, 500,  "EPSV failed.")
        return false
    }
    ln, err := net.ListenTCP("tcp", laddr)
    if err != nil {
        fmt.Println(err)
        ftpcmd.Write(session.commandConn, 500,  "EPSV failed.")
        return false
    }
    if session.pasvListener != nil {
        ftpcmd.Write(session.commandConn, 526,  "Already listening.")
        return false
    }
    session.dtpState = DTP_PASSIVE
    session.pasvListener = ln

    port := ln.Addr().(*net.TCPAddr).Port
    reply := fmt.Sprintf("Entering Extended Passive Mode (|||%d|).", port)
    ftpcmd.Write(session.commandConn, 229, reply)
    fmt.Printf("cmdEpsv(): listening on port: %d\n", port)

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

    fileName := command.Args

    if !strings.HasPrefix(fileName, "/") {
        fileName = session.workingDir + "/" + fileName
    }
    fileName = path.Clean(fileName)
    vhost := cfg.GetVhost(fileName)

    // Check if URL is accessible
    var resp *http.Response
    ret := ftpdata.OpenUrl(vhost, fileName, &resp)
    if ret != true {
        // make sure ln is destroyed
        session.pasvListener.Close()
        session.pasvListener = nil
        session.dtpState = DTP_NONE
        ftpcmd.Write(session.commandConn, 550, "Failed to open file.")
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
        ftpdata.CloseUrl(resp)
        ftpcmd.Write(session.commandConn, 500, "Failed to accept data connection.")
        return false
    }
    fmt.Println(conn)
    session.dataConn = conn

    session.pasvListener.Close()
    session.pasvListener = nil
    session.dtpState = DTP_NONE

    ftpcmd.Write(session.commandConn, 150, "Opening BINARY mode data connection for x.")

    ret = ftpdata.SendUrl(session.dataConn, resp)
    ftpdata.CloseUrl(resp)
    ftpdata.Close(session.dataConn)

    if ret != true {
        ftpcmd.Write(session.commandConn, 550, "Failed to open file.")
        return false
    }
    ftpcmd.Write(session.commandConn, 226, "Transfer complete.")
    return true
}

func cmdPwd(session *Session, command Command) (bool) {
    ftpcmd.Write(session.commandConn, 257, fmt.Sprintf("\"%s\"", session.workingDir))
    return true
}

func cmdCwd(session *Session, command Command) (bool) {
    newPath := command.Args
    
    if !strings.HasPrefix(newPath, "/") {
        newPath = session.workingDir + "/" + newPath
    }
    newPath = path.Clean(newPath)

    if parseindex.IsDir(newPath) {
        session.workingDir = newPath
        ftpcmd.Write(session.commandConn, 250, "Directory successfully changed.")
        return true
    }

    ftpcmd.Write(session.commandConn, 550, newPath + ": No such file or directory")
    return false
}

func cmdList(session *Session, command Command) (bool) {
    if session.dtpState == DTP_NONE {
        ftpcmd.Write(session.commandConn, 425, "Use PORT or PASV first.")
        return false
    }

    if session.dtpState != DTP_PASSIVE {
        ftpcmd.Write(session.commandConn, 425, "Only PASV implemented")
        return false
    }

    dirName := command.Args

    if !strings.HasPrefix(dirName, "/") {
        dirName = session.workingDir + "/" + dirName
    }
    dirName = path.Clean(dirName)

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

    listing, ret := parseindex.DirList(dirName)
    if ret != true {
        ftpcmd.Write(session.commandConn, 526, "Failed to send directory, please retry.")
        return false
    }
    ftpdata.SendString(session.dataConn, listing)
    ftpdata.Close(session.dataConn)

    ftpcmd.Write(session.commandConn, 226, "Directory send OK.")

    return true
}

func cmdFeat(session *Session, command Command) (bool) {
    featReply := "211-Features:\r\n MDTM\r\n SIZE\r\n EPSV\r\n211 End\r\n"

    ftpcmd.WriteRaw(session.commandConn, featReply)
    return true
}

func cmdMdtm(session *Session, command Command) (bool) {
    fileName := command.Args

    if !strings.HasPrefix(fileName, "/") {
        fileName = session.workingDir + "/" + fileName
    }
    fileName = path.Clean(fileName)

    _, fileTime, ret := parseindex.FileStat(fileName)
    if ret != true {
        ftpcmd.Write(session.commandConn, 550, "Could not get file modification time.")
        return false
    }

    ftpcmd.Write(session.commandConn, 213, fileTime)

    return true
}

func cmdSize(session *Session, command Command) (bool) {
    fileName := command.Args

    if !strings.HasPrefix(fileName, "/") {
        fileName = session.workingDir + "/" + fileName
    }
    fileName = path.Clean(fileName)

    fileSize, _, ret := parseindex.FileStat(fileName)
    if ret != true {
        ftpcmd.Write(session.commandConn, 550, "Could not get file size.")
        return false
    }

    ftpcmd.Write(session.commandConn, 213, strconv.FormatInt(fileSize, 10))

    return true
}

func cmdSyst(session *Session, command Command) (bool) {
    ftpcmd.Write(session.commandConn, 215, "UNIX Type: L8")

    return true
}
