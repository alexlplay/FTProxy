package main

import "fmt"
import "net/http"
import "io"
import "os"

func main() {
    fmt.Printf("start\n")

    resp, err := http.Get("http://127.0.0.1/")

    s := make([]byte, 10)
    _, _ = resp.Body.Read(s)

    fmt.Println(resp)
    fmt.Println(err)
    fmt.Printf("%s\n", s)

    _, _ = resp.Body.Read(s)
    fmt.Printf("%s\n", s)
    f, _ := os.Create("/tmp/test.out")
    test(resp.Body)
    testWrite(f)
    testP()
}

func test(r io.Reader) {
    c := make([]byte, 20)
    _, _ = r.Read(c)
    fmt.Printf("more stuff:\n")
    fmt.Printf("%s\n", c)
}

func testWrite(w io.Writer) {
    w.Write([]byte("blah\n"))
}


func testP() {
  resp, _ := http.Get("http://127.0.0.1/")
  defer resp.Body.Close()
  out, err := os.Create("/tmp/filename.ext")
  if err != nil {
    // panic?
  }
  defer out.Close()
  io.Copy(out, resp.Body)
}
