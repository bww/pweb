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
  return (o & OptionVerbose) == OptionVerbose
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
  Proxy   string
  Routes  map[string][]string
  Options Options
}

/**
 * A server
 */
type Server struct {
  addr    string
  root    string
  routes  map[string][]string
  peer    *url.URL
  proxy   *proxy.ReverseProxy
  options Options
}

/**
 * Create a servier
 */
func New(conf Config) (*Server, error) {
  s := &Server{
    addr:     conf.Addr,
    root:     conf.Root,
    routes:   conf.Routes,
    options:  conf.Options,
  }
  if conf.Proxy != "" {
    target, err := url.Parse(conf.Proxy)
    if err != nil {
      return nil, err
    }
    s.proxy = proxy.NewSingleHostReverseProxy(target)
    s.peer = target
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
  
  return server.ListenAndServe()
}

/**
 * Proxy a request
 */
func (s *Server) handleRequest(rsp http.ResponseWriter, req *http.Request) {
  if s.proxy != nil && (s.options & OptionVerbose) == OptionVerbose {
    if u, err := url.Parse(req.URL.Path); err == nil {
      log.Printf("%s %s \u2192 %v", req.Method, req.URL.Path, s.peer.ResolveReference(u))
    }
  }
  if s.proxy == nil {
    s.serveError(rsp, req, http.StatusBadGateway, fmt.Errorf("No proxy is configured for non-managed resource: %s", req.URL.Path))
  }else if err := s.proxy.ServeHTTP(rsp, req); err == proxy.FileNotFoundError {
    s.serveRequestWithOptions(rsp, req, false) // attempt to serve the local version
  }else if err != nil {
    s.serveError(rsp, req, http.StatusBadGateway, err)
  }
}

/**
 * Route a request
 */
func (s *Server) routeRequest(request *http.Request) ([]string, string, error) {
  alternates := make([]string, 0)
  absolute := request.URL.Path
  
  for k, v := range s.routes {
    if strings.HasPrefix(absolute, k) {
      for _, e := range v {
        alternates = append(alternates, path.Join(e, absolute[len(k):]))
      }
    }
  }
  
  ext := path.Ext(absolute)
  relatives := make([]string, len(alternates))
  bases := make([]string, len(alternates))
  
  for i, e := range alternates {
    r := path.Join(s.root, e[1:])
    relatives[i] = r
    bases[i] = r[:len(r) - len(ext)]
  }
  
  var mimetype string
  if mimetype = mime.TypeByExtension(ext); mimetype == "" {
    mimetype = "text/plain"
  }
  
  return relatives, mimetype, nil
}

/**
 * Serve a request
 */
func (s *Server) serveRequest(rsp http.ResponseWriter, req *http.Request) {
  s.serveRequestWithOptions(rsp, req, true)
}

/**
 * Serve a request
 */
func (s *Server) serveRequestWithOptions(rsp http.ResponseWriter, req *http.Request, allowProxy bool) {
  
  candidates, mimetype, err := s.routeRequest(req)
  if err != nil {
    s.serveError(rsp, req, http.StatusNotFound, fmt.Errorf("Could not map resource: %s", req.URL.Path))
    return
  }
  
  if (s.options & OptionVerbose) == OptionVerbose {
    log.Printf("%s %s \u2192 {%s}", req.Method, req.URL.Path, strings.Join(candidates, ", "))
  }
  
  for _, e := range candidates {
    file, err := os.Open(e)
    if err == nil {
      defer file.Close()
      if (s.options & OptionVerbose) == OptionVerbose || (s.options & OptionQuiet) != OptionQuiet {
        log.Printf("%s %s \u2192 %s", req.Method, req.URL.Path, e)
      }
      rsp.Header().Add("Content-Type", mimetype)
      s.serveFile(rsp, req, file)
      return
    }
  }
  
  if !allowProxy || (s.options & OptionStrict) == OptionStrict || s.proxy == nil {
    s.serveError(rsp, req, http.StatusNotFound, fmt.Errorf("No such resource: %s", req.URL.Path))
  }else{
    s.handleRequest(rsp, req)
  }
  
}

/**
 * Serve a request
 */
func (s *Server) serveFile(rsp http.ResponseWriter, req *http.Request, file *os.File) {
  
  fstat, err := file.Stat()
  if err != nil {
    s.serveError(rsp, req, http.StatusBadRequest, fmt.Errorf("Could not stat file: %v", file.Name()))
    return
  }
  if fstat.Mode().IsDir() {
    s.serveError(rsp, req, http.StatusBadRequest, fmt.Errorf("Resource is not a file: %v", file.Name()))
    return
  }
  
  _, err = io.Copy(rsp, file)
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
