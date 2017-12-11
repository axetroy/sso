package main

import (
  "net/http"
  "io/ioutil"
  "os"
  "fmt"
  "path"
  "errors"
  "strconv"
  "github.com/urfave/cli"
  "github.com/axetroy/local-ip"
)

func main() {

  var (
    downloadTimes    int
    shareFileAbsPath string
    ip               string
    err              error
  )

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
      Name:  "port",
      Value: 1066,
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

    var (
      b      []byte
      err    error
      length int
    )
    if b, err = ioutil.ReadFile(shareFileAbsPath); err != nil {
      writer.Write([]byte(err.Error()))
    } else {
      writer.Header().Add("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(shareFileAbsPath)))
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

    fmt.Println("The file " + path.Base(absFilePath) + " Share on http:://" + ip + server.Addr)

    err = server.ListenAndServe()

    return
  }

  app.Run(os.Args)
}
