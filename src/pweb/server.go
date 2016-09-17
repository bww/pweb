package pweb

import (
  "os"
  "io"
  "fmt"
  "log"
  "time"
  "mime"
  "path"
  "strings"
  "net/url"
  "net/http"
  "pweb/proxy"
)

type Options uint32
const (
  OptionNone    = Options(0)
  OptionQuiet   = Options(1 << 0)
  OptionVerbose = Options(1 << 1)
  OptionDebug   = Options(1 << 2)
  OptionStrict  = Options(1 << 3)
)

func (o Options) Quiet() bool {
  return (o & OptionQuiet) == OptionQuiet
}

func (o Options) Verbose() bool {
  return (o & OptionVerbose) == OptionVerbose && !o.Quiet()
}

func (o Options) Debug() bool {
  return (o & OptionDebug) == OptionDebug
}

func (o Options) Strict() bool {
  return (o & OptionStrict) == OptionStrict
}

/**
 * Configuration
 */
type Config struct {
  Addr    string
  Root    string
  Proxies map[string]string
  Options Options
}

/**
 * A server
 */
type Server struct {
  addr    string
  root    string
  proxy   *proxy.ReverseProxy
  targets map[string]*url.URL
  options Options
}

/**
 * Create a servier
 */
func New(conf Config) (*Server, error) {
  s := &Server{
    addr:     conf.Addr,
    root:     conf.Root,
    options:  conf.Options,
  }
  
  if conf.Proxies != nil && len(conf.Proxies) > 0 {
    s.proxy = &proxy.ReverseProxy{Director:s.proxyDirector}
    s.targets = make(map[string]*url.URL)
    for k, v := range conf.Proxies {
      u, err := url.Parse(v)
      if err != nil {
        return nil, err
      }
      s.targets[k] = u
    }
  }
  
  return s, nil
}

/**
 * Run forever
 */
func (s *Server) Run() error {
  mux := http.NewServeMux()
  mux.HandleFunc("/", s.handleRequest)
  
  server := &http.Server{
    Addr: s.addr,
    Handler: mux,
    WriteTimeout: 30 * time.Second,
    ReadTimeout: 30 * time.Second,
  }
  
  if !s.options.Quiet() {
    log.Printf("Accepting connections on %v", s.addr)
  }
  
  return server.ListenAndServe()
}

/**
 * Proxy a request
 */
func (s *Server) handleRequest(rsp http.ResponseWriter, req *http.Request) {
  if s.proxy != nil && s.proxyTarget(req.URL.Path) != nil {
    err := s.proxy.ServeHTTP(rsp, req)
    if err == proxy.FileNotFoundError {
      s.serveRequest(rsp, req)
    }else if err != nil {
      s.serveError(rsp, req, http.StatusBadGateway, fmt.Errorf("Could not proxy request: %v", err))
    }
  }else{
    s.serveRequest(rsp, req)
  }
}

/**
 * Serve a request
 */
func (s *Server) serveRequest(rsp http.ResponseWriter, req *http.Request) {
  
  candidates, mimetype, err := s.routeRequest(req)
  if err != nil {
    s.serveError(rsp, req, http.StatusNotFound, fmt.Errorf("Could not route resource: %s", req.URL.Path))
    return
  }
  
  if s.options.Verbose() {
    log.Printf("%s %s \u2192 {%s}", req.Method, req.URL.Path, strings.Join(candidates, ", "))
  }
  
  for _, e := range candidates {
    file, err := os.Open(e)
    if os.IsNotExist(err) {
      continue
    }else if err != nil {
      s.serveError(rsp, req, http.StatusInternalServerError, fmt.Errorf("Could not read resource: %s", req.URL.Path))
      return
    }
    defer file.Close()
    rsp.Header().Add("Content-Type", mimetype)
    s.serveFile(rsp, req, file)
    return
  }
  
  s.serveError(rsp, req, http.StatusNotFound, fmt.Errorf("No such resource: %s", req.URL.Path))
}

/**
 * Route a request
 */
func (s *Server) routeRequest(request *http.Request) ([]string, string, error) {
  abs := request.URL.Path
  ext := path.Ext(abs)
  
  var mimetype string
  if mimetype = mime.TypeByExtension(ext); mimetype == "" {
    mimetype = "text/plain"
  }
  
  candidates := []string{path.Join(s.root, abs[1:])}
  return candidates, mimetype, nil
}

/**
 * Serve a request
 */
func (s *Server) serveFile(rsp http.ResponseWriter, req *http.Request, file *os.File) {
  if s.options.Verbose() {
    log.Printf("%s %s \u2192 %s", req.Method, req.URL.Path, file.Name())
  }
  
  fstat, err := file.Stat()
  if err != nil {
    s.serveError(rsp, req, http.StatusBadRequest, fmt.Errorf("Could not stat file: %v", file.Name()))
    return
  }
  if fstat.Mode().IsDir() {
    s.serveIndex(rsp, req, file)
    return
  }
  
  _, err = io.Copy(rsp, file)
  if err != nil { // no reason to believe writing an error would work...
    log.Println("Could not write response:", err)
    return
  }
  
}

/**
 * Serve a request
 */
func (s *Server) serveIndex(rsp http.ResponseWriter, req *http.Request, file *os.File) {
  if s.options.Strict() {
    s.serveError(rsp, req, http.StatusForbidden, fmt.Errorf("Indexes are not permitted: %v", file.Name()))
    return
  }
  
  infos, err := file.Readdir(0)
  if err != nil {
    s.serveError(rsp, req, http.StatusBadRequest, fmt.Errorf("Could not read directory: %v", file.Name()))
    return
  }
  
  index := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>PWeb</title>
</head>
<body>
  <ul>
%v
  </ul>
</body>
</html>`
  
  var entries string
  for _, e := range infos {
    n := e.Name()
    if e.IsDir() {
      n += "/"
    }
    ref, err := url.Parse(n)
    if err != nil {
      s.serveError(rsp, req, http.StatusBadRequest, fmt.Errorf("Could not list directory: %v", file.Name()))
      return
    }
    entries += fmt.Sprintf(`<li><a href="%v">%v</a></li>`, ref, e.Name())
  }
  
  rsp.Header().Add("Content-Type", "text/html")
  _, err = rsp.Write([]byte(fmt.Sprintf(index, entries)))
  if err != nil { // no reason to believe writing an error would work...
    log.Println("Could not write response:", err)
    return
  }
  
}

/**
 * Serve an error
 */
func (s *Server) serveError(rsp http.ResponseWriter, req *http.Request, status int, problem error) {
  log.Println("ERROR:", problem)
  rsp.WriteHeader(status)
  rsp.Write([]byte(problem.Error()))
}
