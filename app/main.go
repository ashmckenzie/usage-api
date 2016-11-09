package main

import (
  "encoding/json"
  "encoding/xml"
  "errors"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "regexp"
  "strconv"
  "strings"
  "time"

  // "github.com/davecgh/go-spew/spew"
  "github.com/PuerkitoBio/goquery"
  "github.com/gocraft/web"
  "github.com/headzoo/surf"
  "github.com/headzoo/surf/browser"
)

// Logger ..
var Logger = log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds)

var netClient = &http.Client{
  Timeout: time.Second * 20,
}

var version = os.Getenv("VERSION")

var iinetusage *usage
var vodafoneusage *usage

var refreshMax time.Duration = 30

var refreshIINet time.Duration
var refreshVodafone time.Duration

var refreshIINetMin time.Duration = 15
var refreshIINetAt = refreshIINetMin
var refreshVodafoneMin time.Duration = 30
var refreshVodafoneAt = refreshIINetMin

// Context ..
type Context struct {
}

type rawVodafoneusage struct {
  Quota uint64 `json:"unit_total"`
  Used  uint64 `json:"unit_count"`
}

type result struct {
  XMLName      xml.Name      `xml:"ii_feed"`
  Quotas       []quotaReset  `xml:"volume_usage>quota_reset"`
  TrafficTypes []trafficType `xml:"volume_usage>expected_traffic_types>type"`
}

type quotaReset struct {
  Anniversary   uint64 `xml:"anniversary"`
  DaysRemaining uint64 `xml:"days_remaining"`
}

type trafficType struct {
  Classification string  `xml:"classification,attr"`
  Used           uint64  `xml:"used,attr"`
  Quotas         []quota `xml:"quota_allocation"`
}

type quota struct {
  Amount uint64 `xml:",chardata"`
}

type usage struct {
  Quota            uint64  `json:"quota"`
  Used             uint64  `json:"used"`
  Remaining        uint64  `json:"remaining"`
  PercentUsed      float64 `json:"percent_used"`
  PercentRemaining float64 `json:"percent_remaining"`
  DaysRemaining    uint64  `json:"days_remaining"`
}

type output struct {
  Data    data   `json:"data"`
  Version string `json:"version"`
}

type data struct {
  IINet    usage `json:"internet"`
  Vodafone usage `json:"mobile"`
}

func resetRefreshPeriods() {
  refreshIINetAt = refreshIINetMin
  refreshVodafoneAt = refreshVodafoneMin
}

func (c *Context) rootPath(rw web.ResponseWriter, req *web.Request) {
  allData := data{
    IINet:    *iinetusage,
    Vodafone: *vodafoneusage,
  }

  allOutput := output{
    Data:    allData,
    Version: version,
  }

  outputJSON, _ := json.Marshal(allOutput)

  rw.Header().Add("Content-Type", "application/json")
  resetRefreshPeriods()
  fmt.Fprint(rw, string(outputJSON))
}

func strToUInt(str string) (uint64, error) {
  nonFractionalPart := strings.Split(str, ".")
  return strconv.ParseUint(nonFractionalPart[0], 10, 64)
}

func getVodafoneusage() error {
  var quota uint64
  var used uint64
  var daysRemaining uint64
  var attr string
  var err error
  var fm browser.Submittable
  var success bool
  var dataDetail map[string]interface{}
  var periodDetail string

  url := "https://myaccount.myvodafone.com.au/home"

  bow := surf.NewBrowser()
  bow.SetUserAgent("Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36")
  err = bow.Open(url)
  if err != nil {
    Logger.Printf("ERROR: getVodafoneusage(): bow.Open(url): %s", err)
    return err
  }

  if bow.Title() != "My usage | Vodafone Australia" {
    fm, err = bow.Form("form#loginForm")
    if err != nil {
      Logger.Printf(`ERROR: getVodafoneusage(): bow.Form("form#loginForm"): %s`, err)
      return err
    }

    fm.Input("userid", os.Getenv("VODAFONE_MOBILE_NUMBER"))
    fm.Input("password", os.Getenv("VODAFONE_PASSWORD"))
    if fm.Submit() != nil {
      Logger.Printf("ERROR: getVodafoneusage(): fm.Submit(): %s", err)
      return err
    }
  }

  bow.Find("#included-data-plan > figure:nth-child(3)").Each(func(_ int, s *goquery.Selection) { attr, success = s.Attr("data-barchart") })
  if !success {
    panic("Could not locate DOM element for data usage")
  }
  _ = json.Unmarshal([]byte(attr), &dataDetail)

  bow.Find("div.hidden-mobile:nth-child(1) > div:nth-child(1) > div:nth-child(1) > span:nth-child(1)").Each(func(_ int, s *goquery.Selection) {
    periodDetail = strings.Replace(strings.Trim(s.Text(), " "), "\n", " ", -1)
  })
  if len(periodDetail) == 0 {
    err = errors.New("Could not locate DOM element for days remaining")
    Logger.Printf("ERROR: len(periodDetail): %s", err)
    return err
  }

  // spew.Dump(periodDetail)

  endsTodayRegex := regexp.MustCompile(`Ends today`)

  if endsTodayRegex.Match([]byte(periodDetail)) {
    daysRemaining = 0
  } else {
    re := regexp.MustCompile(`(?P<days_remaining>\d+) days left.+Inclusions refresh: (?P<resets_at_date>\d+ \w{3})`)
    r2 := re.FindAllStringSubmatch(periodDetail, -1)[0]
    daysRemaining, _ = strToUInt(r2[1])
  }

  quota, _ = strToUInt(dataDetail["unit_total"].(string))
  used, _ = strToUInt(dataDetail["unit_count"].(string))
  remaining := quota - used
  percentUsed := (float64(used) / float64(quota)) * 100
  percentRemaining := 100 - percentUsed

  vodafoneusage = &usage{Quota: quota, Used: used, Remaining: remaining, PercentUsed: percentUsed, PercentRemaining: percentRemaining, DaysRemaining: daysRemaining}

  Logger.Printf("Vodafone usage=[%v]", vodafoneusage)

  return nil
}

func getIINetusage() error {
  username := os.Getenv("IINET_USERNAME")
  password := os.Getenv("IINET_PASSWORD")

  url := fmt.Sprintf("https://toolbox.iinet.net.au/cgi-bin/new/volume_usage_xml.cgi?username=%s&action=login&password=%s", username, password)

  response, err := netClient.Get(url)
  if err != nil {
    Logger.Printf("ERROR: getIINetusage(): netClient.Get(url): %s", err)
    return err
  }
  data, _ := ioutil.ReadAll(response.Body)
  response.Body.Close()

  // spew.Dump(data)

  r := result{}
  err = xml.Unmarshal([]byte(data), &r)
  if err != nil {
    Logger.Printf("ERROR: getIINetusage(): ml.Unmarshal([]byte(data), &r): %s", err)
    return err
  }

  daysRemaining := r.Quotas[0].DaysRemaining
  quota := r.TrafficTypes[0].Quotas[0].Amount * 1000000
  used := r.TrafficTypes[0].Used
  remaining := quota - used
  percentUsed := (float64(used) / float64(quota)) * 100
  percentRemaining := 100 - percentUsed

  iinetusage = &usage{Quota: quota, Used: used, Remaining: remaining, PercentUsed: percentUsed, PercentRemaining: percentRemaining, DaysRemaining: daysRemaining}

  Logger.Printf("IINet usage=[%v]", iinetusage)

  return nil
}

func loggerMiddleware(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
  startTime := time.Now()

  next(rw, req)

  duration := time.Since(startTime).Nanoseconds()
  var durationUnits string
  switch {
  case duration > 2000000:
    durationUnits = "ms"
    duration /= 1000000
  case duration > 1000:
    durationUnits = "μs"
    duration /= 1000
  default:
    durationUnits = "ns"
  }

  Logger.Printf("[%d %s] %d '%s'\n", duration, durationUnits, rw.StatusCode(), req.URL.Path)
}

func listenOn() string {
  port := os.Getenv("PORT")
  if len(port) == 0 {
    port = "3000"
  }
  return fmt.Sprintf("0.0.0.0:%s", port)
}

func setupIINetUsageRefresh() {
  getIINetusage()

  tickerIINetRefresh := time.NewTicker(1 * time.Minute)
  go func() {
    for range tickerIINetRefresh.C {
      if refreshIINet >= refreshIINetAt {
        getIINetusage()
        refreshIINet = 0
        refreshIINetAt = refreshMax
      } else {
        refreshIINet++
      }
    }
  }()
}

func setupVodafoneUsageRefresh() {
  getVodafoneusage()

  tickerVodafoneRefresh := time.NewTicker(1 * time.Minute)
  go func() {
    for range tickerVodafoneRefresh.C {
      if refreshVodafone >= refreshVodafoneAt {
        getVodafoneusage()
        refreshVodafone = 0
        refreshVodafoneAt = refreshMax
      } else {
        refreshVodafone++
      }
    }
  }()
}

func setupHTTP() {
  router := web.New(Context{}).
    Middleware(loggerMiddleware).
    Get("/", (*Context).rootPath)

  listenOn := listenOn()

  Logger.Println("Listening on", listenOn)
  http.ListenAndServe(listenOn, router)
}

func main() {
  setupIINetUsageRefresh()
  setupVodafoneUsageRefresh()
  setupHTTP()
}
