package main

import (
  "encoding/xml"
  "encoding/json"
  "fmt"
  "io/ioutil"
  "net/http"
  "log"
  "os"
  "regexp"
  "strings"
  "strconv"
  "time"

  // "github.com/davecgh/go-spew/spew"
  "github.com/gocraft/web"
  "github.com/headzoo/surf"
  "github.com/PuerkitoBio/goquery"
)

var Logger = log.New(os.Stdout, "usage: ", log.Ldate|log.Lmicroseconds)

var netClient = &http.Client{
  Timeout: time.Second * 10,
}

var iinetUsage *Usage
var vodafoneUsage *Usage

type Context struct {
}

type RawVodafoneUsage struct {
  Quota uint64 `json:"unit_total"`
  Used  uint64 `json:"unit_count"`
}

type Result struct {
  XMLName      xml.Name      `xml:"ii_feed"`
  Quotas       []QuotaReset  `xml:"volume_usage>quota_reset"`
  TrafficTypes []TrafficType `xml:"volume_usage>expected_traffic_types>type"`
}

type QuotaReset struct {
  Anniversary   uint64  `xml:"anniversary"`
  DaysRemaining uint64  `xml:"days_remaining"`
}

type TrafficType struct {
  Classification string  `xml:"classification,attr"`
  Used           uint64  `xml:"used,attr"`
  Quotas         []Quota `xml:"quota_allocation"`
}

type Quota struct {
  Amount uint64 `xml:",chardata"`
}

type Usage struct {
  Quota            uint64  `json:"quota"`
  Used             uint64  `json:"used"`
  Remaining        uint64  `json:"remaining"`
  PercentUsed      float64 `json:"percent_used"`
  PercentRemaining float64 `json:"percent_remaining"`
  DaysRemaining    uint64  `json:"days_remaining"`
}

type Data struct {
  IINet    Usage `json:"internet"`
  Vodafone Usage `json:"mobile"`
}

func (c *Context) Root(rw web.ResponseWriter, req *web.Request) {
  allData := Data{ IINet: *iinetUsage, Vodafone: *vodafoneUsage }
  data, _ := json.Marshal(allData)

  rw.Header().Add("Content-Type", "application/json")

  fmt.Fprint(rw, string(data))
}

func strToUInt(str string) (uint64, error) {
  nonFractionalPart := strings.Split(str, ".")
  return strconv.ParseUint(nonFractionalPart[0], 10, 64)
}

func getVodafoneUsage() *Usage {
  var quota uint64
  var used uint64
  var daysRemaining uint64
  var attr string
  var err error
  var success bool
  var dataDetail map[string]interface{}
  var periodDetail string

  url := "https://myaccount.myvodafone.com.au/home"

  bow := surf.NewBrowser()
  bow.SetUserAgent("Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36")
  err = bow.Open(url)
  if err != nil { panic(err) }

  if bow.Title() != "My usage | Vodafone Australia" {
    fm, err := bow.Form("form#loginForm")
    if err != nil { panic(err) }

    fm.Input("userid", os.Getenv("VODAFONE_MOBILE_NUMBER"))
    fm.Input("password", os.Getenv("VODAFONE_PASSWORD"))
    if fm.Submit() != nil { panic(err) }
  }

  bow.Find("#included-data-plan > figure:nth-child(3)").Each(func(_ int, s *goquery.Selection) { attr, success = s.Attr("data-barchart") })
  if ! success { panic("Could not locate DOM element for data usage") }
  _ = json.Unmarshal([]byte(attr), &dataDetail)

  bow.Find("div.hidden-mobile:nth-child(1) > div:nth-child(1) > div:nth-child(1) > span:nth-child(1)").Each(func(_ int, s *goquery.Selection) {
    periodDetail = strings.Replace(strings.Trim(s.Text(), " "), "\n", " ", -1)
  })
  if len(periodDetail) == 0 { panic("Could not locate DOM element for days remaining") }

  re := regexp.MustCompile(`(?P<days_remaining>\d+) days left.+Inclusions refresh: (?P<resets_at_date>\d+ \w{3})`)
  r2 := re.FindAllStringSubmatch(periodDetail, -1)[0]

  // spew.Dump(r2)

  daysRemaining, _ = strToUInt(r2[1])
  quota, _ = strToUInt(dataDetail["unit_total"].(string))
  used, _ = strToUInt(dataDetail["unit_count"].(string))
  remaining := quota - used
  percentUsed := (float64(used) / float64(quota)) * 100
  percentRemaining := 100 - percentUsed

  usage := Usage{ Quota: quota, Used: used, Remaining: remaining, PercentUsed: percentUsed, PercentRemaining: percentRemaining, DaysRemaining: daysRemaining }

  Logger.Printf("Vodafone usage=[%s]", usage)

  return &usage
}

func getIINetUsage() *Usage {
  username := os.Getenv("IINET_USERNAME")
  password := os.Getenv("IINET_PASSWORD")

  url := fmt.Sprintf("https://toolbox.iinet.net.au/cgi-bin/new/volume_usage_xml.cgi?username=%s&action=login&password=%s", username, password)

  response, _ := netClient.Get(url)
  data, _ := ioutil.ReadAll(response.Body)
  response.Body.Close()

  // data := `
// <ii_feed>
//     <volume_usage>
//         <quota_reset>
//             <anniversary>17</anniversary>
//             <days_so_far>28</days_so_far>
//             <days_remaining>4</days_remaining>
//         </quota_reset>
//         <expected_traffic_types>
//             <type classification="anytime" used="860912892239">
//                 <name>anytime</name>
//                 <quota_allocation>1000000</quota_allocation>
//             </type>
//             <type classification="uploads" used="162617953613">
//                 <name>uploads</name>
//             </type>
//         </expected_traffic_types>
//     </volume_usage>
// </ii_feed>
//   `

  r := Result{}
  _ = xml.Unmarshal([]byte(data), &r)

  daysRemaining := r.Quotas[0].DaysRemaining
  quota := r.TrafficTypes[0].Quotas[0].Amount * 1000000
  used := r.TrafficTypes[0].Used
  remaining := quota - used
  percentUsed := (float64(used) / float64(quota)) * 100
  percentRemaining := 100 - percentUsed

  usage := Usage{ Quota: quota, Used: used, Remaining: remaining, PercentUsed: percentUsed, PercentRemaining: percentRemaining, DaysRemaining: daysRemaining }

  Logger.Printf("IINet usage=[%s]", usage)

  return &usage
}

func main() {
  iinetUsage = getIINetUsage()
  vodafoneUsage = getVodafoneUsage()

  ticker1 := time.NewTicker(15 * time.Minute)
  go func() {
    for range ticker1.C { iinetUsage = getIINetUsage() }
  }()

  ticker2 := time.NewTicker(15 * time.Minute)
  go func() {
    for range ticker2.C { vodafoneUsage = getVodafoneUsage() }
  }()

  router := web.New(Context{}).
    Middleware(web.LoggerMiddleware).
    Middleware(web.ShowErrorsMiddleware).
    Get("/", (*Context).Root)

  Logger.Println("Ready for requests!")
  http.ListenAndServe("localhost:3000", router)
}
