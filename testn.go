package main

import "fmt"
import "os"
import "parseindex"

func main() {
    fmt.Println("start")
    fi, err := os.Open("./nginx.index")
    if err != nil {
        fmt.Println("cannot open file", err.Error())
        os.Exit(1)
    }
    fmt.Println(parseindex.ParseNginxHtmlList(fi))
}
