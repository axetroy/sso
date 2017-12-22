package main

import (
  "net/http"
  "io/ioutil"
  "os"
  "fmt"
  "path"
  "errors"
  "strconv"
  "time"
  "net"
  "github.com/urfave/cli"
  "github.com/axetroy/local-ip"
)

func getFreePort() (port int, err error) {
  listener, err := net.Listen("tcp", "127.0.0.1:0")
  if err != nil {
    return 0, err
  }
  defer listener.Close()

  addr := listener.Addr().String()
  _, portString, err := net.SplitHostPort(addr)
  if err != nil {
    return 0, err
  }

  return strconv.Atoi(portString)
}

func main() {

  var (
    downloading      bool   // 是否正在下载
    downloadTimes    int    // 已被下载的次数
    shareFileAbsPath string // 分享文件的绝对路径
    ip               string // ip
    err              error
  )

  ticker := time.NewTicker(time.Microsecond * 100)

  go func() {
    for range ticker.C {
      if downloadTimes > 0 {
        fmt.Println("The file have been download. close server.")
        os.Exit(0)
      }
    }
  }()

  if err, ip = local_ip.Get(); err != nil {
    panic(err)
    return
  }

  app := cli.NewApp()

  app.Name = "sso"
  app.Usage = "sso" + " [path]"
  app.Version = "0.0.1"
  app.Description = "Share Object once"

  app.Flags = []cli.Flag{
    cli.Int64Flag{
      Name:  "port, p",
      Usage: "The port of server",
    },
  }

  server := &http.Server{Addr: ":1066"}

  http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {

    if downloadTimes > 0 {
      fmt.Println("The file have been download. close server.")
      os.Exit(0)
      return
    }

    if downloading {
      writer.Write([]byte("The file is being downloading."))
      return
    }

    downloading = true

    var (
      b      []byte
      err    error
      length int
    )
    if b, err = ioutil.ReadFile(shareFileAbsPath); err != nil {
      writer.Write([]byte(err.Error()))
    } else {
      writer.Header().Add("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(shareFileAbsPath)))
      writer.Header().Add("Content-Length", strconv.Itoa(len(b)))
      if length, err = writer.Write(b); err != nil {
        writer.Write([]byte(err.Error()))
        return
      }

      if length == len(b) {
        downloadTimes++
      }

    }
  })

  app.Action = func(c *cli.Context) (err error) {
    defer func() {
      if err != nil {
        fmt.Println(err)
        os.Exit(1)
        return
      }
    }()

    var (
      cwd         string
      fileStat    os.FileInfo
      filePath    = c.Args().Get(0)
      port        = c.Int64("port")
      absFilePath string
    )

    if port == 0 {
      if portInt, err := getFreePort(); err != nil {
        return err
      } else {
        port = int64(portInt)
      }
    }

    server.Addr = ":" + strconv.Itoa(int(port))

    if cwd, err = os.Getwd(); err != nil {
      return
    }

    if path.IsAbs(filePath) {
      absFilePath = filePath
    } else {
      absFilePath = path.Join(cwd, filePath)
    }

    shareFileAbsPath = absFilePath

    if fileStat, err = os.Stat(absFilePath); err != nil {
      return
    }

    if fileStat.IsDir() {
      err = errors.New("can not share an dir")
      return
    }

    fmt.Println("The file " + path.Base(absFilePath) + " have been shared on http://" + ip + server.Addr)

    fmt.Println("Once file been downloaded, Service will shut down automatically.")

    err = server.ListenAndServe()

    return
  }

  app.Run(os.Args)
}
