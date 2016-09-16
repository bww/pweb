package pweb

import (
  "log"
  "net/url"
  "net/http"
  "pweb/proxy"
)

/**
 * Obtain the proxy target for a path or nil if the path is
 * not proxied.
 */
func (s *Server) proxyTarget(p string) *url.URL {
  var target *url.URL
  for k, t := range s.targets {
    lk, lp := len(k), len(p)
    if lp >= lk && p[:lk] == k && (lp == lk || p[lk] == '/') {
      target = t
    }
  }
  return target
}

/** 
 * Proxy a request
 */
func (s *Server) proxyDirector(req *http.Request) {
  target := s.proxyTarget(req.URL.Path)
  omethod, opath := req.Method, req.URL.Path
  if target != nil {
    targetQuery := target.RawQuery
    req.URL.Scheme = target.Scheme
    req.URL.Host = target.Host
    req.URL.Path = proxy.SingleJoiningSlash(target.Path, req.URL.Path)
    if targetQuery == "" || req.URL.RawQuery == "" {
      req.URL.RawQuery = targetQuery + req.URL.RawQuery
    }else{
      req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
    }
  }
  if s.options.Verbose() {
    log.Printf("%s %s \u2192 %s", omethod, opath, req.URL)
  }
}