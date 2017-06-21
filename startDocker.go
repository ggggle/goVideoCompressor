package main

import (
    "os/exec"
    "bytes"
    "fmt"
    "strings"
)

func main()  {
    cmd:=exec.Command("/usr/arukasCli/arukas", "ps", "-a")
    w := bytes.NewBuffer(nil)
    cmd.Stdout = w
    cmd.Run()
    allLines := strings.Split(string(w.Bytes()), "\n")
    for _,line:= range allLines[1:] {
        fmt.Printf("%v\n [%d]", strings.Fields(line), len(strings.Fields(line)))
    }
}