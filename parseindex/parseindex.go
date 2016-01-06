package parseindex

import "fmt"
import "golang.org/x/net/html"
import "cfg"
import "net/http"
import "strings"
import "time"
//import "bufio"

type FsObjectType int
const (
    FS_NONE FsObjectType = iota
    FS_DIR
    FS_FILE
)

type FsObject struct {
    otype FsObjectType
    name string
    time time.Time
    size int64
}

func GenDirList(objects []FsObject) (string) {
    var listing string
    for _, object := range objects {
        var printTime string
        var lineHdr string
        if(object.time.Year() < time.Now().Year()) {
            printTime = object.time.Format("Jan _2  2006")
        } else {
            printTime = object.time.Format(time.Stamp)
        }
        if object.otype == FS_DIR {
            lineHdr = "d"
        } else {
            lineHdr = "-"
        }
        listing = fmt.Sprintf("%s%srwxr-xr-x 1 ftp ftp %d %s %s\r\n", listing, lineHdr, object.size, printTime, object.name)
    }
    return listing
}

func getTokenAttr(tok *html.Token, attrName string) (string) {
    for _, a := range tok.Attr {
        if a.Key == attrName {
            return a.Val
        }
    }
    return ""
}

type Parser interface {
    Parse(path string) (string, bool)
    Mdtm(dirPath string, fileName string) (string, bool)
}

type ParserAutoIndex struct {
}

func (p ParserAutoIndex) Parse(path string) (string, bool) {
    cfg.LoadConfig("ftproxy.conf")
    vhost := cfg.GetVhost(path)
    url := fmt.Sprintf("http://%s/%s/", vhost, path)
    fmt.Printf("URL FOR INDEX: %s\n", url)
    resp, err := http.Get(url)
    if err != nil {
        return "", false
    }
    defer resp.Body.Close()
    var objects []FsObject
    fmt.Printf("Server header: %s\n", resp.Header["Server"][0])
    if strings.Contains(resp.Header["Server"][0], "nginx") {
        objects = ParseNginxHtmlList(resp.Body)
    } else {
        // Attempt apache
        objects = ParseApacheHtmlList(resp.Body)
    }
    return GenDirList(objects), true
}

type ParserConf struct {
}

func (p ParserConf) Parse(truc string) (string, bool) {
    cfg.LoadConfig("ftproxy.conf")
    vhosts := cfg.GetVhosts()
    var listing string
    for path, _  := range vhosts {
        // Generate fake timestamps for first-level directories (our list of vhosts)
        listing = fmt.Sprintf("%sdrwxr-xr-x 1 ftp ftp 4096 %s %s\r\n", listing, time.Now().Format(time.Stamp), strings.Trim(path, "/"))
    }
    return listing, true
}

/* As with DIR, this assumes we look for a file in the current directory(dirPath).
   fileName must be a single file name. Proper directory handling TBD */
func (p ParserAutoIndex) Mdtm(dirPath string, fileName string) (string, bool) {
    /* Whole section below similar to Parse(), factor it in a separate function */
    cfg.LoadConfig("ftproxy.conf")
    fmt.Printf("MDTM for file: -%s-\n", fileName)
    vhost := cfg.GetVhost(dirPath)
    url := fmt.Sprintf("http://%s/%s/", vhost, dirPath)
    fmt.Printf("URL FOR INDEX: %s\n", url)
    resp, err := http.Get(url)
    if err != nil {
        return "", false
    }
    defer resp.Body.Close()
    var objects []FsObject
    fmt.Printf("Server header: %s\n", resp.Header["Server"][0])
    if strings.Contains(resp.Header["Server"][0], "nginx") {
        objects = ParseNginxHtmlList(resp.Body)
    } else {
        objects = ParseApacheHtmlList(resp.Body)
    }
    /* End section*/
    for _, object := range objects {
        if object.name == fileName && object.otype == FS_FILE {
            fmt.Printf("Found file, time is: %s\n", object.time)
            mdtmTime := object.time.Format("20060102030405")
            return mdtmTime, true
        }
    }
    return "", false
}

func (p ParserConf) Mdtm(dirPath string, fileName string) (string, bool) {
    return "", false
}
