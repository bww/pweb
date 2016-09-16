package main

import (
  "os"
  "fmt"
  "flag"
  "pweb"
  "strings"
)

var DEBUG bool
var DEBUG_VERBOSE bool

/**
 * You know what it does
 */
func main() {
  var proxyRoutes flagList
  
  cmdline       := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
  fDocroot      := cmdline.String   ("docroot",         coalesce(os.Getenv("PWEB_DOCROOT"), "."),       "The document root to serve requests from.")
  fAddr         := cmdline.String   ("bind",            coalesce(os.Getenv("PWEB_BIND_ADDR"), ":8080"), "The address to serve requests from, as '[<host>]:<port>'.")
  fQuiet        := cmdline.Bool     ("quiet",           strToBool(os.Getenv("PWEB_QUIET")),             "Be less verbose.")
  fVerbose      := cmdline.Bool     ("verbose",         strToBool(os.Getenv("PWEB_VERBOSE")),           "Be more verbose.")
  fDebug        := cmdline.Bool     ("debug",           strToBool(os.Getenv("PWEB_DEBUG")),             "Enable debugging mode.")
  cmdline.Var     (&proxyRoutes,     "proxy",                                                           "Define routes to reverse-proxy requests from, as '<path1>=<url1>[,..,<path1>=<url1>]'.")
  cmdline.Parse(os.Args[1:])
  
  var options pweb.Options
  if *fQuiet {
    options |= pweb.OptionQuiet
  }
  if *fVerbose {
    options |= pweb.OptionVerbose
  }
  if *fDebug {
    options |= pweb.OptionDebug
  }
  
  var proxies map[string]string
  if len(proxyRoutes) > 0 {
    proxies = make(map[string]string)
    for _, e := range proxyRoutes {
      for _, r := range strings.Split(e, ",") {
        p := strings.Split(r, "=")
        if len(p) != 2 {
          fmt.Printf("%v: Invalid proxy route: %v\n", os.Args[0], e)
          return
        }
        proxies[p[0]] = p[1]
      }
    }
  }
  
  conf := pweb.Config{
    Addr: *fAddr,
    Root: *fDocroot,
    Options: options,
    Proxies: proxies,
  }
  
  server, err := pweb.New(conf)
  if err != nil {
    panic(err)
  }
  
  panic(server.Run())
}

/**
 * Return the first non-empty string from those provided
 */
func coalesce(v... string) string {
  for _, e := range v {
    if e != "" {
      return e
    }
  }
  return ""
}

/**
 * String to bool
 */
func strToBool(s string, d ...bool) bool {
  if s == "" {
    if len(d) > 0 {
      return d[0]
    }else{
      return false
    }
  }
  return strings.EqualFold(s, "t") || strings.EqualFold(s, "true") || strings.EqualFold(s, "y") || strings.EqualFold(s, "yes")
}

/**
 * Flag string list
 */
type flagList []string

/**
 * Set a flag
 */
func (s *flagList) Set(v string) error {
  *s = append(*s, v)
  return nil
}

/**
 * Describe
 */
func (s *flagList) String() string {
  return fmt.Sprintf("%+v", *s)
}
