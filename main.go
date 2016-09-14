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
var Logger = log.New(os.Stdout, "usage: ", log.Ldate|log.Lmicroseconds)

var netClient = &http.Client{
	Timeout: time.Second * 20,
}

var iinetUsage *Usage
var vodafoneUsage *Usage

// Context ..
type Context struct {
}

// RawVodafoneUsage ..
type RawVodafoneUsage struct {
	Quota uint64 `json:"unit_total"`
	Used  uint64 `json:"unit_count"`
}

// Result ..
type Result struct {
	XMLName      xml.Name      `xml:"ii_feed"`
	Quotas       []QuotaReset  `xml:"volume_usage>quota_reset"`
	TrafficTypes []TrafficType `xml:"volume_usage>expected_traffic_types>type"`
}

// QuotaReset ..
type QuotaReset struct {
	Anniversary   uint64 `xml:"anniversary"`
	DaysRemaining uint64 `xml:"days_remaining"`
}

// TrafficType ..
type TrafficType struct {
	Classification string  `xml:"classification,attr"`
	Used           uint64  `xml:"used,attr"`
	Quotas         []Quota `xml:"quota_allocation"`
}

// Quota ..
type Quota struct {
	Amount uint64 `xml:",chardata"`
}

// Usage ..
type Usage struct {
	Quota            uint64  `json:"quota"`
	Used             uint64  `json:"used"`
	Remaining        uint64  `json:"remaining"`
	PercentUsed      float64 `json:"percent_used"`
	PercentRemaining float64 `json:"percent_remaining"`
	DaysRemaining    uint64  `json:"days_remaining"`
}

// Data ..
type Data struct {
	IINet    Usage `json:"internet"`
	Vodafone Usage `json:"mobile"`
}

func (c *Context) rootPath(rw web.ResponseWriter, req *web.Request) {
	allData := Data{IINet: *iinetUsage, Vodafone: *vodafoneUsage}
	data, _ := json.Marshal(allData)

	rw.Header().Add("Content-Type", "application/json")

	fmt.Fprint(rw, string(data))
}

func strToUInt(str string) (uint64, error) {
	nonFractionalPart := strings.Split(str, ".")
	return strconv.ParseUint(nonFractionalPart[0], 10, 64)
}

func getVodafoneUsage() error {
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
		Logger.Printf("ERROR: getVodafoneUsage(): bow.Open(url): %s", err)
		return err
	}

	if bow.Title() != "My usage | Vodafone Australia" {
		fm, err = bow.Form("form#loginForm")
		if err != nil {
			Logger.Printf(`ERROR: getVodafoneUsage(): bow.Form("form#loginForm"): %s`, err)
			return err
		}

		fm.Input("userid", os.Getenv("VODAFONE_MOBILE_NUMBER"))
		fm.Input("password", os.Getenv("VODAFONE_PASSWORD"))
		if fm.Submit() != nil {
			Logger.Printf("ERROR: getVodafoneUsage(): fm.Submit(): %s", err)
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

	re := regexp.MustCompile(`(?P<days_remaining>\d+) days left.+Inclusions refresh: (?P<resets_at_date>\d+ \w{3})`)
	r2 := re.FindAllStringSubmatch(periodDetail, -1)[0]

	daysRemaining, _ = strToUInt(r2[1])
	quota, _ = strToUInt(dataDetail["unit_total"].(string))
	used, _ = strToUInt(dataDetail["unit_count"].(string))
	remaining := quota - used
	percentUsed := (float64(used) / float64(quota)) * 100
	percentRemaining := 100 - percentUsed

	vodafoneUsage = &Usage{Quota: quota, Used: used, Remaining: remaining, PercentUsed: percentUsed, PercentRemaining: percentRemaining, DaysRemaining: daysRemaining}

	Logger.Printf("Vodafone usage=[%v]", vodafoneUsage)

	return nil
}

func getIINetUsage() error {
	username := os.Getenv("IINET_USERNAME")
	password := os.Getenv("IINET_PASSWORD")

	url := fmt.Sprintf("https://toolbox.iinet.net.au/cgi-bin/new/volume_usage_xml.cgi?username=%s&action=login&password=%s", username, password)

	response, err := netClient.Get(url)
	if err != nil {
		Logger.Printf("ERROR: getIINetUsage(): netClient.Get(url): %s", err)
		return err
	}
	data, _ := ioutil.ReadAll(response.Body)
	response.Body.Close()

	r := Result{}
	err = xml.Unmarshal([]byte(data), &r)
	if err != nil {
		Logger.Printf("ERROR: getIINetUsage(): ml.Unmarshal([]byte(data), &r): %s", err)
		return err
	}

	daysRemaining := r.Quotas[0].DaysRemaining
	quota := r.TrafficTypes[0].Quotas[0].Amount * 1000000
	used := r.TrafficTypes[0].Used
	remaining := quota - used
	percentUsed := (float64(used) / float64(quota)) * 100
	percentRemaining := 100 - percentUsed

	iinetUsage = &Usage{Quota: quota, Used: used, Remaining: remaining, PercentUsed: percentUsed, PercentRemaining: percentRemaining, DaysRemaining: daysRemaining}

	Logger.Printf("IINet usage=[%v]", iinetUsage)

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

func main() {
	getIINetUsage()
	getVodafoneUsage()

	ticker1 := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker1.C {
			getIINetUsage()
		}
	}()

	ticker2 := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker2.C {
			getVodafoneUsage()
		}
	}()

	router := web.New(Context{}).
		Middleware(loggerMiddleware).
		// Middleware(web.LoggerMiddleware).
		// Middleware(web.ShowErrorsMiddleware).
		Get("/", (*Context).rootPath)

	Logger.Println("Ready for requests!")
	http.ListenAndServe("localhost:3000", router)
}
