package main

import (
    "net"
    "fmt"
    "time"
    "os/exec"
    "strings"
    "github.com/dutchcoders/goftp"
    "os"
    "bytes"
)

var SERVER_IP string
var SERVER_PORT = "8055"
var printLog = false
var converSuccess = make(chan string, 10)
var FTP_USERNAME string
var FTP_PASSWORD string

func main() {
    if SERVER_IP = os.Getenv("SERVER_IP"); 0 == len(SERVER_IP) {
        fmt.Print("SERVER_IP NIL")
        return
    }
    if FTP_USERNAME = os.Getenv("FTP_USERNAME"); 0 == len(FTP_USERNAME) {
        fmt.Print("FTP_USERNAME NIL")
        return
    }
    if FTP_PASSWORD = os.Getenv("FTP_PASSWORD"); 0 == len(FTP_PASSWORD) {
        fmt.Print("FTP_PASSWORD NIL")
        return
    }
    if temp := os.Getenv("SERVER_PORT"); len(temp) > 0 {
        SERVER_PORT = temp
    }
    for {
        connect, err := net.DialTimeout("tcp", SERVER_IP+":8055", time.Second*10)
        ipStr := ""
        if err == nil {
            MyPrintf("connect success\n")
            ipStr = connect.LocalAddr().String()
            data := make([]byte, 100)
            MyPrintf("i'm here\n")
            n, err := connect.Read(data)
            if err != nil {
                MyPrintf("error info[%s]\n", err.Error())
                continue
            } else {
                Data := data[:n]
                //Data  dir;%d;args
                if recieveSplit := strings.Split(string(Data), ";"); len(recieveSplit) > 1 {
                    //path   dir/%d.mp4
                    path := strings.Join(recieveSplit[:2], "/") + ".mp4"
                    MyPrintf("args [%v]\n", recieveSplit)
                    go downloadFileAndConvert(path, recieveSplit[2])
                }
            }
            //等待ftp上传完毕
            select {
            case status := <-converSuccess:
                if len(status) > 0 {
                    MyPrintf("tell server[%s]\n", status)
                    _, err := connect.Write([]byte(status))
                    if err == nil {
                        MyPrintf("tell server success\n")
                        connect.Close()
                    }
                }
            }

        } else {
            MyPrintf("connect error[%s]\n", err.Error())
            time.Sleep(time.Second * 10)
        }
        if connect != nil {
            connect.Close()
            MyPrintf("disconnect[%s]\n", ipStr)
        }
    }
}

//path  文件夹名/文件名
func downloadFileAndConvert(path string, args string) {
    fileDir := strings.Split(path, "/")[0]
    fileName := strings.Split(path, "/")[1]
    if len(args) <= 0 {
        MyPrintf("args is nil\n")
        converSuccess <- "fail;" + strings.Split(fileName, ".")[0] + ";转换参数为空"
        return
    }
    nginxServer := "http://" + SERVER_IP + "/"
    cmd := exec.Command("wget", nginxServer+path)
    cmd.Run()
    //path   dir/%d.mp4
    convert(strings.Split(path, "/")[1], args)
    uploadFile("c"+fileName, fileDir)
    return
}

func convert(path string, args string) (result int) {
    MyPrintf("start convert\n")
    selectArg := []string{"-i", path}
    //allArgs := []string{"-threads", "8", "-vcodec", "libx264"}
    allArgs := strings.Split(args, " ")
    convertFileName := "c" + path
    /*if len(args) > 0 {
        argsSplit := strings.Split(args, " ")
        allArgs = append(allArgs, argsSplit...)
    }*/
    cmdLine := append(selectArg, allArgs...)
    cmdLine = append(cmdLine, convertFileName)
    //path   %d.mp4
    MyPrintf("input args[%v]\n", cmdLine)
    cmd := exec.Command("ffmpeg", cmdLine...)
    w := bytes.NewBuffer(nil)
    cmd.Stderr = w
    cmd.Run()
    //MyPrintf("%s\n", string(w.Bytes()))
    f, err := os.OpenFile("c"+path+".log", os.O_WRONLY|os.O_CREATE, 0655)
    defer f.Close()
    if err == nil {
        f.WriteString(string(w.Bytes()))
        MyPrintf("writelog success\n")
    } else {
        MyPrintf("write log error[%v]", err.Error())
    }
    MyPrintf("convert success\n")
    return
}

func uploadFile(name string, path string) {
    defer func() {
        //转换出来的文件
        os.Remove(name)
        os.Remove(name + ".log")
        //转换前的文件
        os.Remove(name[1:])
    }()
    MyPrintf("start upload\n")
    fileName := strings.Split(name[1:], ".")[0]
    ftp, err := goftp.Connect(SERVER_IP + ":21")
    if err != nil {
        converSuccess <- "fail;" + fileName + ";连接ftp服务器失败"
        return
    }
    defer ftp.Close()
    if err = ftp.Login(FTP_USERNAME, FTP_PASSWORD); err != nil {
        converSuccess <- "fail;" + fileName + ";登录ftp服务器失败"
        return
    }
    //先上传转换日志文件
    logfile, err := os.Open(name + ".log")
    if err != nil {
        converSuccess <- "fail;" + fileName + ";wtf日志文件不存在"
        return
    }
    defer logfile.Close()
    if err := ftp.Stor(path+"/"+name+".log", logfile); err != nil {
        converSuccess <- "fail;" + fileName + ";wtf日志文件上传失败"
        return
    }
    file, err := os.Open(name)
    if err != nil {
        converSuccess <- "fail;" + fileName
        return
    }
    defer file.Close()
    if err := ftp.Stor(path+"/"+name[1:], file); err != nil {
        converSuccess <- "fail;" + fileName
        return
    }
    converSuccess <- "success;" + fileName
    MyPrintf("upload over\n")
}

func MyPrintf(format string, a ...interface{}) (n int, err error) {
    if printLog {
        return fmt.Printf(format, a ...)
    }
    return
}
