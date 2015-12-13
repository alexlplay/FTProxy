package cfg

import (
    "encoding/json"
    "os"
    "fmt"
    "strings"
)

type Cfg struct {
    MaxConnections   int
    DefaultVhost   string
    ListenPort      string
    Vhosts map[string]interface{}
}

var conf Cfg

func LoadConfig(filePath string) {
    file, err := os.Open(filePath)
    if err != nil {
        fmt.Println("Cannot open config file:", err.Error())
        os.Exit(1)
    }
    defer file.Close()
    decoder := json.NewDecoder(file)
    var f map[string]interface{}
    err = decoder.Decode(&f)
    conf.MaxConnections = int(f["maxConnections"].(float64))
    conf.DefaultVhost = f["defaultHttpIp"].(string)
    conf.ListenPort = f["listenPort"].(string)
    conf.Vhosts = f["httpIps"].(map[string]interface{})
}

// Return the vhost for the given path (either dir or file)
func GetVhost(path string) (string) {
    for pathPrefix, vhost := range conf.Vhosts {
        if strings.HasPrefix(path, pathPrefix) {
            return vhost.(string)
        }
    }
    return conf.DefaultVhost
}

func GetVhosts() (map[string]interface{}) {
    return conf.Vhosts
}

func GetListenPort() (string) {
    return conf.ListenPort
}

func GetMaxConnections() (int) {
    return conf.MaxConnections
}
